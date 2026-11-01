package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid or expired authorization token")

type SessionClaims struct {
	UserID    uuid.UUID `json:"uid"`
	AccountID uuid.UUID `json:"aid"`
	ArcTag    string    `json:"tag"`
	ExpiresAt int64     `json:"exp"`
}

type TokenManager struct {
	secretKey []byte
}

func NewTokenManager(secret string) *TokenManager {
	if secret == "" {
		secret = "arcpay_hmac_super_secret_key_change_in_production_99182"
	}
	return &TokenManager{secretKey: []byte(secret)}
}

// GenerateToken creates a tamper-proof signed session token (Valid for 30 days)
func (tm *TokenManager) GenerateToken(userID, accountID uuid.UUID, arcTag string) (string, error) {
	claims := SessionClaims{
		UserID:    userID,
		AccountID: accountID,
		ArcTag:    arcTag,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour).Unix(),
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)

	// HMAC-SHA256 signature
	mac := hmac.New(sha256.New, tm.secretKey)
	mac.Write([]byte(payloadB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s.%s", payloadB64, sigB64), nil
}

// VerifyToken validates signature and expiry
func (tm *TokenManager) VerifyToken(tokenString string) (*SessionClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	payloadB64, providedSigB64 := parts[0], parts[1]

	// Verify HMAC signature
	mac := hmac.New(sha256.New, tm.secretKey)
	mac.Write([]byte(payloadB64))
	expectedSig := mac.Sum(nil)

	providedSig, err := base64.RawURLEncoding.DecodeString(providedSigB64)
	if err != nil || !hmac.Equal(providedSig, expectedSig) {
		return nil, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims SessionClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}
