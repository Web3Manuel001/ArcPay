package ledger

import (
	"time"

	"github.com/google/uuid"
)

type Direction string

const (
	Debit  Direction = "DEBIT"
	Credit Direction = "CREDIT"
)

type AccountType string

const (
	Asset     AccountType = "ASSET"     // User balances, settlement floats
	Liability AccountType = "LIABILITY" // User deposits held
	Equity    AccountType = "EQUITY"    // Protocol reserve
	Revenue   AccountType = "REVENUE"   // POS fees, swap fees
	Expense   AccountType = "EXPENSE"   // Gas subsidies, partner processing fees
)

type Account struct {
	ID          uuid.UUID   `json:"id"`
	HolderID    uuid.UUID   `json:"holder_id"`
	Corridor    string      `json:"corridor"`
	Currency    string      `json:"currency"`
	AccountType AccountType `json:"account_type"`
	IsActive    bool        `json:"is_active"`
	CreatedAt   time.Time   `json:"created_at"`
}

type JournalEntry struct {
	ID             uuid.UUID `json:"id"`
	IdempotencyKey string    `json:"idempotency_key"`
	ReferenceType  string    `json:"reference_type"`
	Narration      string    `json:"narration"`
	Corridor       string    `json:"corridor"`
	CreatedAt      time.Time `json:"created_at"`
}

type Posting struct {
	ID             uuid.UUID `json:"id"`
	JournalEntryID uuid.UUID `json:"journal_entry_id"`
	AccountID      uuid.UUID `json:"account_id"`
	AmountUnits    int64     `json:"amount_units"`
	Direction      Direction `json:"direction"`
	CreatedAt      time.Time `json:"created_at"`
}

// TransactionPayload defines the atomic double-entry execution instruction
type TransactionPayload struct {
	IdempotencyKey string               `json:"idempotency_key"`
	ReferenceType  string               `json:"reference_type"`
	Narration      string               `json:"narration"`
	Corridor       string               `json:"corridor"`
	Postings       []PostingInstruction `json:"postings"`
}

type PostingInstruction struct {
	AccountID   uuid.UUID `json:"account_id"`
	AmountUnits int64     `json:"amount_units"`
	Direction   Direction `json:"direction"`
}
