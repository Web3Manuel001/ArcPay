package identity

import (
	"time"

	"github.com/google/uuid"
)

type Identity struct {
	ID         uuid.UUID `json:"id"`
	ArcTag     string    `json:"arc_tag"`
	PhoneHash  string    `json:"phone_hash"`
	HolderID   uuid.UUID `json:"holder_id"`
	AccountID  uuid.UUID `json:"account_id"`
	Corridor   string    `json:"corridor"`
	EVMAddress string    `json:"evm_address,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type RegisterTagRequest struct {
	ArcTag      string    `json:"arc_tag"`
	PhoneNumber string    `json:"phone_number"`
	HolderID    uuid.UUID `json:"holder_id"`
	AccountID   uuid.UUID `json:"account_id"`
	Corridor    string    `json:"corridor"`
	EVMAddress  string    `json:"evm_address"`
}

type TagTransferRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	SenderTag      string `json:"sender_tag"`
	RecipientTag   string `json:"recipient_tag"`
	AmountUnits    int64  `json:"amount_units"`
	Narration      string `json:"narration"`
}
