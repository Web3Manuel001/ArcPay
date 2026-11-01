package auth

import (
	"time"

	"github.com/google/uuid"
)

type RegisterRequest struct {
	FullName    string `json:"full_name"`
	PhoneNumber string `json:"phone_number"` // e.g., "+2348012345678"
	ArcTag      string `json:"arc_tag"`      // e.g., "$emmanuel"
	PIN         string `json:"pin"`          // 4-6 digit secret PIN
	Corridor    string `json:"corridor"`     // "NG", "KE", "GH"
}

type LoginRequest struct {
	TagOrPhone string `json:"tag_or_phone"` // "$emmanuel" or "+2348012345678"
	PIN        string `json:"pin"`
}

type AuthResponse struct {
	User       UserProfile `json:"user"`
	Token      string      `json:"token"`
	CreatedAt  time.Time   `json:"created_at"`
}

type UserProfile struct {
	ID          uuid.UUID `json:"id"`
	FullName    string    `json:"full_name"`
	ArcTag      string    `json:"arc_tag"`
	PhoneNumber string    `json:"phone_number"`
	AccountID   uuid.UUID `json:"account_id"`
	Corridor    string    `json:"corridor"`
	KYCTier     string    `json:"kyc_tier"` // TIER_0_STARTER, TIER_1_BASIC, TIER_2_VERIFIED
	DailyLimitUSD float64 `json:"daily_limit_usd"`
}
