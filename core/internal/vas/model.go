package vas

import (
	"time"

	"github.com/google/uuid"
)

type Operator string

const (
	OperatorMTN       Operator = "MTN"
	OperatorAirtel    Operator = "AIRTEL"
	OperatorSafaricom Operator = "SAFARICOM"
	OperatorVodacom   Operator = "VODACOM"
	OperatorGlo       Operator = "GLO"
)

type AirtimeRequest struct {
	IdempotencyKey string   `json:"idempotency_key"`
	SenderTag      string   `json:"sender_tag"`      // e.g., "$chidi"
	PhoneNumber    string   `json:"phone_number"`    // Destination, e.g., "+2348012345678"
	Operator       Operator `json:"operator"`        // MTN, AIRTEL, etc.
	AmountUSDC     float64  `json:"amount_usdc"`     // e.g., 2.00
	Corridor       string   `json:"corridor"`        // 'NG', 'KE', 'GH', 'TZ'
}

type AirtimeReceipt struct {
	ID             uuid.UUID `json:"id"`
	JournalEntryID uuid.UUID `json:"journal_entry_id"`
	PhoneNumber    string    `json:"phone_number"`
	Operator       Operator  `json:"operator"`
	AmountUSDC     float64   `json:"amount_usdc"`
	AmountUnits    int64     `json:"amount_units"`
	LocalAmount    string    `json:"local_amount"` // e.g. "₦3,300.00"
	Status         string    `json:"status"`       // "DELIVERED"
	DeliveredAt    time.Time `json:"delivered_at"`
}
