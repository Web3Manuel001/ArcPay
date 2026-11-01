package trade

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/arcpay/core/internal/identity"
	"github.com/arcpay/core/internal/ledger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TradeInvoice struct {
	ID                  uuid.UUID `json:"id"`
	InvoiceCode         string    `json:"invoice_code"`
	ImporterTag         string    `json:"importer_tag"`
	SupplierName        string    `json:"supplier_name"`
	SupplierDestination string    `json:"supplier_destination"`
	AmountUSD           float64   `json:"amount_usd"`
	AmountUnits         int64     `json:"amount_units"`
	AmountNGN           float64   `json:"amount_ngn"`
	FeeUSD              float64   `json:"fee_usd"`
	Narration           string    `json:"narration"`
	ShippingRef         string    `json:"shipping_ref,omitempty"`
	Status              string    `json:"status"`
	CreatedAt           time.Time `json:"created_at"`
}

type CreateTradeRequest struct {
	SupplierName        string  `json:"supplier_name"`
	SupplierDestination string  `json:"supplier_destination"`
	AmountUSD           float64 `json:"amount_usd"`
	Narration           string  `json:"narration"`
}

type Service struct {
	pool            *pgxpool.Pool
	identityService *identity.Service
	ledgerService   *ledger.Service
	treasuryAccount uuid.UUID
}

func NewService(pool *pgxpool.Pool, idService *identity.Service, ledgerService *ledger.Service, treasuryAccount uuid.UUID) *Service {
	return &Service{
		pool:            pool,
		identityService: idService,
		ledgerService:   ledgerService,
		treasuryAccount: treasuryAccount,
	}
}

func generateTradeCode() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("trd_%s", hex.EncodeToString(b))
}

func (s *Service) CreateTradeInvoice(ctx context.Context, importerTag string, req CreateTradeRequest) (*TradeInvoice, error) {
	if req.AmountUSD <= 0 {
		return nil, errors.New("trade amount must be greater than zero")
	}

	cleanTag := identity.CleanTag(importerTag)
	_, err := s.identityService.ResolveTag(ctx, cleanTag)
	if err != nil {
		return nil, fmt.Errorf("importer tag not found: %w", err)
	}

	amountUnits := int64(req.AmountUSD * 1_000_000)
	feeUnits := (amountUnits * 75) / 10000 // 0.75% B2B Trade fee
	amountNGN := req.AmountUSD * 1650.0

	invoiceCode := generateTradeCode()
	invoiceID := uuid.New()

	query := `
		INSERT INTO trade_invoices (
			id, invoice_code, importer_tag, supplier_name, supplier_destination, 
			amount_usd, amount_units, amount_ngn, fee_units, narration, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'PENDING', NOW())
	`
	_, err = s.pool.Exec(ctx, query,
		invoiceID, invoiceCode, cleanTag, req.SupplierName, req.SupplierDestination,
		req.AmountUSD, amountUnits, amountNGN, feeUnits, req.Narration,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trade invoice: %w", err)
	}

	return &TradeInvoice{
		ID:                  invoiceID,
		InvoiceCode:         invoiceCode,
		ImporterTag:         cleanTag,
		SupplierName:        req.SupplierName,
		SupplierDestination: req.SupplierDestination,
		AmountUSD:           req.AmountUSD,
		AmountUnits:         amountUnits,
		AmountNGN:           amountNGN,
		FeeUSD:              float64(feeUnits) / 1_000_000.0,
		Narration:           req.Narration,
		Status:              "PENDING",
		CreatedAt:           time.Now(),
	}, nil
}

func (s *Service) FundTradeEscrow(ctx context.Context, invoiceCode, importerTag string) error {
	cleanTag := identity.CleanTag(importerTag)
	importer, err := s.identityService.ResolveTag(ctx, cleanTag)
	if err != nil {
		return err
	}

	var invoiceID uuid.UUID
	var amountUnits int64
	var status string

	query := `SELECT id, amount_units, status FROM trade_invoices WHERE invoice_code = $1 AND importer_tag = $2`
	err = s.pool.QueryRow(ctx, query, invoiceCode, cleanTag).Scan(&invoiceID, &amountUnits, &status)
	if err != nil {
		return errors.New("trade invoice not found")
	}
	if status != "PENDING" {
		return errors.New("invoice already funded or closed")
	}

	// Verify importer has sufficient balance
	bal, err := s.ledgerService.GetBalance(ctx, importer.AccountID)
	if err != nil || bal < amountUnits {
		return errors.New("insufficient balance to fund trade escrow")
	}

	// Lock funds in Escrow: Debit Importer, Credit System Escrow Float
	escrowFloatID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	idempotencyKey := fmt.Sprintf("escrow_lock_%s", invoiceCode)

	_, err = s.ledgerService.ProcessTransaction(ctx, ledger.TransactionPayload{
		IdempotencyKey: idempotencyKey,
		ReferenceType:  "TRADE_ESCROW_LOCK",
		Narration:      fmt.Sprintf("Escrow Locked for Trade Invoice %s", invoiceCode),
		Corridor:       "NG",
		Postings: []ledger.PostingInstruction{
			{AccountID: importer.AccountID, AmountUnits: amountUnits, Direction: ledger.Debit},
			{AccountID: escrowFloatID, AmountUnits: amountUnits, Direction: ledger.Credit},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to lock escrow funds: %w", err)
	}

	_, _ = s.pool.Exec(ctx, `UPDATE trade_invoices SET status = 'IN_ESCROW', funded_at = NOW() WHERE id = $1`, invoiceID)
	return nil
}

func (s *Service) ReleaseTradeEscrow(ctx context.Context, invoiceCode, importerTag string) error {
	cleanTag := identity.CleanTag(importerTag)

	var invoiceID uuid.UUID
	var amountUnits int64
	var feeUnits int64
	var status string
	var supplierDest string

	query := `SELECT id, amount_units, fee_units, status, supplier_destination FROM trade_invoices WHERE invoice_code = $1 AND importer_tag = $2`
	err := s.pool.QueryRow(ctx, query, invoiceCode, cleanTag).Scan(&invoiceID, &amountUnits, &feeUnits, &status, &supplierDest)
	if err != nil {
		return errors.New("trade invoice not found")
	}
	if status != "IN_ESCROW" && status != "DISPATCHED" {
		return errors.New("escrow is not in a releasable state")
	}

	// Double-entry release: Debit Escrow Float, Credit Treasury (0.75% fee), Disburse remainder
	escrowFloatID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	idempotencyKey := fmt.Sprintf("escrow_release_%s", invoiceCode)

	_, err = s.ledgerService.ProcessTransaction(ctx, ledger.TransactionPayload{
		IdempotencyKey: idempotencyKey,
		ReferenceType:  "TRADE_ESCROW_RELEASE",
		Narration:      fmt.Sprintf("Trade Escrow Disbursed to %s (Ref: %s)", supplierDest, invoiceCode),
		Corridor:       "NG",
		Postings: []ledger.PostingInstruction{
			{AccountID: escrowFloatID, AmountUnits: amountUnits, Direction: ledger.Debit},
			{AccountID: s.treasuryAccount, AmountUnits: feeUnits, Direction: ledger.Credit},
			{AccountID: escrowFloatID, AmountUnits: amountUnits - feeUnits, Direction: ledger.Credit},
		},
	})
	if err != nil {
		return fmt.Errorf("ledger disbursement failed: %w", err)
	}

	_, _ = s.pool.Exec(ctx, `UPDATE trade_invoices SET status = 'RELEASED', released_at = NOW() WHERE id = $1`, invoiceID)
	return nil
}
