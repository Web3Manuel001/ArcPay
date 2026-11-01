package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/arcpay/core/internal/identity"
	"github.com/arcpay/core/internal/ledger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pinSalt = "arcpay_auth_salt_v1"

type Service struct {
	pool          *pgxpool.Pool
	ledgerService *ledger.Service
}

func NewService(pool *pgxpool.Pool, ledgerService *ledger.Service) *Service {
	return &Service{
		pool:          pool,
		ledgerService: ledgerService,
	}
}

func HashPIN(pin string) string {
	h := sha256.Sum256([]byte(pin + pinSalt))
	return hex.EncodeToString(h[:])
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	cleanTag := identity.CleanTag(req.ArcTag)
	if len(cleanTag) < 3 {
		return nil, errors.New("arc tag must be at least 3 characters")
	}
	if len(req.PIN) < 4 {
		return nil, errors.New("pin must be at least 4 digits")
	}

	// 1. Create underlying Double-Entry Ledger Account
	holderID := uuid.New()
	corridor := strings.ToUpper(strings.TrimSpace(req.Corridor))
	if corridor == "" {
		corridor = "NG"
	}

	acc, err := s.ledgerService.CreateAccount(ctx, holderID, corridor, "USDC", ledger.Asset)
	if err != nil {
		return nil, fmt.Errorf("failed to create ledger account: %w", err)
	}

	// 2. Hash PIN and Phone
	pinHash := HashPIN(req.PIN)
	phoneHash := identity.HashPhone(req.PhoneNumber)
	userID := uuid.New()

	// 3. Insert Identity with Auth & Tier 0 KYC status
	query := `
		INSERT INTO identities (
			id, arc_tag, phone_hash, holder_id, account_id, corridor, 
			pin_hash, raw_phone, full_name, kyc_tier, kyc_data, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'TIER_0_STARTER', '{}'::jsonb, NOW())
	`
	_, err = s.pool.Exec(ctx, query, 
		userID, cleanTag, phoneHash, holderID, acc.ID, corridor,
		pinHash, req.PhoneNumber, req.FullName,
	)
	if err != nil {
		return nil, fmt.Errorf("arc tag $%s is already claimed: %w", cleanTag, err)
	}

	profile := UserProfile{
		ID:            userID,
		FullName:      req.FullName,
		ArcTag:        "$" + cleanTag,
		PhoneNumber:   req.PhoneNumber,
		AccountID:     acc.ID,
		Corridor:      corridor,
		KYCTier:       "TIER_0_STARTER",
		DailyLimitUSD: 100.0, // Starter tier limit
	}

	return &AuthResponse{
		User:  profile,
		Token: fmt.Sprintf("arc_token_%s_%s", cleanTag, hex.EncodeToString([]byte(userID.String())[:6])),
	}, nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	cleanIdentifier := identity.CleanTag(req.TagOrPhone)
	hashedPin := HashPIN(req.PIN)

	query := `
		SELECT id, arc_tag, holder_id, account_id, corridor, raw_phone, full_name, kyc_tier, pin_hash
		FROM identities
		WHERE arc_tag = $1 OR raw_phone = $2
	`
	var (
		userID      uuid.UUID
		arcTag      string
		holderID    uuid.UUID
		accountID   uuid.UUID
		corridor    string
		rawPhone    string
		fullName    string
		kycTier     string
		realPinHash string
	)

	err := s.pool.QueryRow(ctx, query, cleanIdentifier, req.TagOrPhone).Scan(
		&userID, &arcTag, &holderID, &accountID, &corridor, &rawPhone, &fullName, &kycTier, &realPinHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("invalid credentials: user not found")
		}
		return nil, err
	}

	if realPinHash != hashedPin {
		return nil, errors.New("invalid credentials: incorrect PIN")
	}

	dailyLimit := 100.0
	if kycTier == "TIER_1_BASIC" {
		dailyLimit = 2000.0
	} else if kycTier == "TIER_2_VERIFIED" {
		dailyLimit = 50000.0
	}

	return &AuthResponse{
		User: UserProfile{
			ID:            userID,
			FullName:      fullName,
			ArcTag:        "$" + arcTag,
			PhoneNumber:   rawPhone,
			AccountID:     accountID,
			Corridor:      corridor,
			KYCTier:       kycTier,
			DailyLimitUSD: dailyLimit,
		},
		Token: fmt.Sprintf("arc_token_%s_%s", arcTag, hex.EncodeToString([]byte(userID.String())[:6])),
	}, nil
}
