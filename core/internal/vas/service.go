package vas

import (
	"context"
	"fmt"
	"time"

	"github.com/arcpay/core/internal/identity"
	"github.com/arcpay/core/internal/ledger"
	"github.com/google/uuid"
)

type Service struct {
	ledgerService   *ledger.Service
	identityService *identity.Service
	atClient        *AfricasTalkingClient
	treasuryAccount uuid.UUID
}

func NewService(ledgerService *ledger.Service, identityService *identity.Service, atClient *AfricasTalkingClient, treasuryAccount uuid.UUID) *Service {
	return &Service{
		ledgerService:   ledgerService,
		identityService: identityService,
		atClient:        atClient,
		treasuryAccount: treasuryAccount,
	}
}

func getFXRate(corridor string) (float64, string) {
	switch corridor {
	case "NG":
		return 1650.0, "NGN"
	case "KE":
		return 129.5, "KES"
	case "GH":
		return 16.2, "GHS"
	case "TZ":
		return 2650.0, "TZS"
	default:
		return 1.0, "USD"
	}
}

func (s *Service) PurchaseAirtime(ctx context.Context, req AirtimeRequest) (*AirtimeReceipt, error) {
	// 1. Resolve user's ArcTag
	userIdentity, err := s.identityService.ResolveTag(ctx, req.SenderTag)
	if err != nil {
		return nil, fmt.Errorf("sender identity not found: %w", err)
	}

	amountUnits := int64(req.AmountUSDC * 1_000_000)
	if amountUnits <= 0 {
		return nil, fmt.Errorf("invalid airtime amount: %.2f", req.AmountUSDC)
	}

	// 2. Calculate local fiat amount for telco switch (e.g. "NGN 3300.00")
	rate, currencySymbol := getFXRate(req.Corridor)
	localValue := req.AmountUSDC * rate
	localFormatted := fmt.Sprintf("%s %.2f", currencySymbol, localValue)

	// 3. Atomic Double-Entry Ledger Settlement
	entry, err := s.ledgerService.ProcessTransaction(ctx, ledger.TransactionPayload{
		IdempotencyKey: req.IdempotencyKey,
		ReferenceType:  "VAS_AIRTIME",
		Narration:      fmt.Sprintf("Airtime Recharge: %s %s to %s", req.Operator, localFormatted, req.PhoneNumber),
		Corridor:       req.Corridor,
		Postings: []ledger.PostingInstruction{
			{AccountID: userIdentity.AccountID, AmountUnits: amountUnits, Direction: ledger.Debit},
			{AccountID: s.treasuryAccount, AmountUnits: amountUnits, Direction: ledger.Credit},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ledger debit failed (insufficient funds): %w", err)
	}

	// 4. Dispatch to Real Telecom API (Africa's Talking Switch)
	var telcoStatus = "DELIVERED"
	if s.atClient != nil {
		telcoResp, err := s.atClient.DispatchAirtime(ctx, req.PhoneNumber, localFormatted)
		if err != nil {
			// In production, an alert triggers an automated reversal or support queue
			return nil, fmt.Errorf("telco switch dispatch error: %w", err)
		}
		if len(telcoResp.Responses) > 0 && telcoResp.Responses[0].Status != "Sent" {
			telcoStatus = telcoResp.Responses[0].Status
		}
	}

	return &AirtimeReceipt{
		ID:             uuid.New(),
		JournalEntryID: entry.ID,
		PhoneNumber:    req.PhoneNumber,
		Operator:       req.Operator,
		AmountUSDC:     req.AmountUSDC,
		AmountUnits:    amountUnits,
		LocalAmount:    localFormatted,
		Status:         telcoStatus,
		DeliveredAt:    time.Now(),
	}, nil
}
