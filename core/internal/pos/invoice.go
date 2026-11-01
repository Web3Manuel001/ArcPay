package pos

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type InvoiceStatus string

const (
	StatusPending   InvoiceStatus = "PENDING"
	StatusPaid      InvoiceStatus = "PAID"
	StatusExpired   InvoiceStatus = "EXPIRED"
	StatusCancelled InvoiceStatus = "CANCELLED"
)

type DynamicInvoice struct {
	ID              string        `json:"id"` // Unique alphanumeric code for QR
	MerchantAccount uuid.UUID     `json:"merchant_account_id"`
	TableNumber     string        `json:"table_number,omitempty"`
	AmountUnits     int64         `json:"amount_units"` // in micro-units
	Currency        string        `json:"currency"`     // USDC
	Status          InvoiceStatus `json:"status"`
	ExpiresAt       time.Time     `json:"expires_at"`
	CreatedAt       time.Time     `json:"created_at"`
}

// GenerateDynamicInvoice creates a 10-minute expiring dynamic checkout invoice
func GenerateDynamicInvoice(merchantAccount uuid.UUID, amountUnits int64, table string) *DynamicInvoice {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	invoiceCode := fmt.Sprintf("arc_%s", hex.EncodeToString(b))

	return &DynamicInvoice{
		ID:              invoiceCode,
		MerchantAccount: merchantAccount,
		TableNumber:     table,
		AmountUnits:     amountUnits,
		Currency:        "USDC",
		Status:          StatusPending,
		ExpiresAt:       time.Now().Add(10 * time.Minute),
		CreatedAt:       time.Now(),
	}
}