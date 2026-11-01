package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/arcpay/core/internal/identity"
	"github.com/arcpay/core/internal/ledger"
	"github.com/arcpay/core/internal/pos"
	"github.com/arcpay/core/internal/redis"
	"github.com/arcpay/core/internal/split"
	"github.com/arcpay/core/internal/vas"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Handler struct {
	ledgerService   *ledger.Service
	identityService *identity.Service
	vasService      *vas.Service
	splitService    *split.Service
	redisClient     *redis.Client
	posHub          *pos.Hub
}

func NewHandler(
	ledgerService *ledger.Service,
	identityService *identity.Service,
	vasService *vas.Service,
	splitService *split.Service,
	redisClient *redis.Client,
	posHub *pos.Hub,
) *Handler {
	return &Handler{
		ledgerService:   ledgerService,
		identityService: identityService,
		vasService:      vasService,
		splitService:    splitService,
		redisClient:     redisClient,
		posHub:          posHub,
	}
}

// --- ARCSPLIT HANDLERS ---

type CreateSplitRequest struct {
	CreatorTag string `json:"creator_tag"`
	Prompt     string `json:"prompt"` // Natural language e.g. "Split $60 with @chidi and @kofi"
}

func (h *Handler) CreateSplitSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSplitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid split request payload")
		return
	}

	session, err := h.splitService.CreateSplit(r.Context(), req.CreatorTag, req.Prompt)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusCreated, session)
}

type PaySplitRequest struct {
	PayerIdentifier string `json:"payer_identifier"` // Tag or phone
	PaymentMethod   string `json:"payment_method"`   // "ARCPAY_BALANCE", "CARD", "BANK_TRANSFER"
}

func (h *Handler) PaySplitShare(w http.ResponseWriter, r *http.Request) {
	splitCode := chi.URLParam(r, "code")
	var req PaySplitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid pay payload")
		return
	}

	err := h.splitService.PaySplitShare(r.Context(), splitCode, req.PayerIdentifier, req.PaymentMethod)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"status":  "PAID",
		"message": "Share successfully settled to creator's balance",
	})
}

// --- VAS (AIRTIME & BILLS) HANDLERS ---

func (h *Handler) PurchaseAirtime(w http.ResponseWriter, r *http.Request) {
	var req vas.AirtimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid airtime payload")
		return
	}

	if req.IdempotencyKey == "" {
		Error(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}

	acquired, err := h.redisClient.AcquireLock(r.Context(), req.IdempotencyKey, 10*time.Second)
	if err != nil || !acquired {
		Error(w, http.StatusConflict, "transaction currently processing or already executed")
		return
	}

	receipt, err := h.vasService.PurchaseAirtime(r.Context(), req)
	if err != nil {
		_ = h.redisClient.ReleaseLock(r.Context(), req.IdempotencyKey)
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusCreated, receipt)
}

// --- IDENTITY & SOCIAL TAG HANDLERS ---

func (h *Handler) RegisterTag(w http.ResponseWriter, r *http.Request) {
	var req identity.RegisterTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid tag registration payload")
		return
	}

	id, err := h.identityService.RegisterTag(r.Context(), req)
	if err != nil {
		Error(w, http.StatusConflict, err.Error())
		return
	}

	JSON(w, http.StatusCreated, id)
}

func (h *Handler) ResolveTag(w http.ResponseWriter, r *http.Request) {
	tag := chi.URLParam(r, "tag")
	id, err := h.identityService.ResolveTag(r.Context(), tag)
	if err != nil {
		Error(w, http.StatusNotFound, "ArcTag not found")
		return
	}
	JSON(w, http.StatusOK, id)
}

func (h *Handler) TransferByTag(w http.ResponseWriter, r *http.Request) {
	var req identity.TagTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid transfer payload")
		return
	}

	if req.IdempotencyKey == "" {
		Error(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}

	sender, err := h.identityService.ResolveTag(r.Context(), req.SenderTag)
	if err != nil {
		Error(w, http.StatusNotFound, fmt.Sprintf("sender tag '%s' does not exist", req.SenderTag))
		return
	}

	recipient, err := h.identityService.ResolveTag(r.Context(), req.RecipientTag)
	if err != nil {
		Error(w, http.StatusNotFound, fmt.Sprintf("recipient tag '%s' does not exist", req.RecipientTag))
		return
	}

	acquired, err := h.redisClient.AcquireLock(r.Context(), req.IdempotencyKey, 10*time.Second)
	if err != nil || !acquired {
		Error(w, http.StatusConflict, "transaction locked or already executing")
		return
	}

	entry, err := h.ledgerService.ProcessTransaction(r.Context(), ledger.TransactionPayload{
		IdempotencyKey: req.IdempotencyKey,
		ReferenceType:  "TAG_TRANSFER",
		Narration:      fmt.Sprintf("Transfer from $%s to $%s: %s", sender.ArcTag, recipient.ArcTag, req.Narration),
		Corridor:       sender.Corridor,
		Postings: []ledger.PostingInstruction{
			{AccountID: sender.AccountID, AmountUnits: req.AmountUnits, Direction: ledger.Debit},
			{AccountID: recipient.AccountID, AmountUnits: req.AmountUnits, Direction: ledger.Credit},
		},
	})
	if err != nil {
		_ = h.redisClient.ReleaseLock(r.Context(), req.IdempotencyKey)
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"journal_id":   entry.ID,
		"from":         "$" + sender.ArcTag,
		"to":           "$" + recipient.ArcTag,
		"amount_usdc":  float64(req.AmountUnits) / 1_000_000.0,
		"amount_units": req.AmountUnits,
		"status":       "SETTLED",
	})
}

// --- BASIC LEDGER HANDLERS ---

type CreateAccountRequest struct {
	HolderID    uuid.UUID          `json:"holder_id"`
	Corridor    string             `json:"corridor"`
	Currency    string             `json:"currency"`
	AccountType ledger.AccountType `json:"account_type"`
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	acc, err := h.ledgerService.CreateAccount(r.Context(), req.HolderID, req.Corridor, req.Currency, req.AccountType)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	JSON(w, http.StatusCreated, acc)
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	accountID, err := uuid.Parse(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid account uuid")
		return
	}

	balanceUnits, err := h.ledgerService.GetBalance(r.Context(), accountID)
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	balanceDecimal := float64(balanceUnits) / 1_000_000.0

	JSON(w, http.StatusOK, map[string]any{
		"account_id":     accountID,
		"balance_units":  balanceUnits,
		"balance_amount": balanceDecimal,
		"currency":       "USDC",
	})
}

type TransferRequest struct {
	IdempotencyKey string                      `json:"idempotency_key"`
	ReferenceType  string                      `json:"reference_type"`
	InvoiceID      string                      `json:"invoice_id,omitempty"`
	Narration      string                      `json:"narration"`
	Corridor       string                      `json:"corridor"`
	Postings       []ledger.PostingInstruction `json:"postings"`
}

func (h *Handler) ProcessTransfer(w http.ResponseWriter, r *http.Request) {
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if req.IdempotencyKey == "" {
		Error(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}

	acquired, err := h.redisClient.AcquireLock(r.Context(), req.IdempotencyKey, 10*time.Second)
	if err != nil || !acquired {
		Error(w, http.StatusConflict, "transaction locked or already processed")
		return
	}

	entry, err := h.ledgerService.ProcessTransaction(r.Context(), ledger.TransactionPayload{
		IdempotencyKey: req.IdempotencyKey,
		ReferenceType:  req.ReferenceType,
		Narration:      req.Narration,
		Corridor:       req.Corridor,
		Postings:       req.Postings,
	})
	if err != nil {
		_ = h.redisClient.ReleaseLock(r.Context(), req.IdempotencyKey)
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if req.InvoiceID != "" {
		h.posHub.BroadcastPayment(req.InvoiceID, entry.ID.String(), req.Postings[0].AmountUnits)
	}

	JSON(w, http.StatusCreated, entry)
}

// --- BUSINESS POS HANDLERS ---

type CreateInvoiceRequest struct {
	MerchantAccount uuid.UUID `json:"merchant_account_id"`
	AmountUnits     int64     `json:"amount_units"`
	TableNumber     string    `json:"table_number"`
}

func (h *Handler) CreatePOSInvoice(w http.ResponseWriter, r *http.Request) {
	var req CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid invoice request")
		return
	}

	invoice := pos.GenerateDynamicInvoice(req.MerchantAccount, req.AmountUnits, req.TableNumber)
	JSON(w, http.StatusCreated, invoice)
}

func (h *Handler) StreamPOSUpdates(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoice_id")
	if invoiceID == "" {
		http.Error(w, "missing invoice_id", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &pos.TerminalClient{
		InvoiceID: invoiceID,
		Conn:      conn,
		Send:      make(chan []byte, 16),
	}

	h.posHub.RegisterClient(client)

	go func() {
		defer func() {
			h.posHub.UnregisterClient(client)
			conn.Close()
		}()
		for msg := range client.Send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}()
}
