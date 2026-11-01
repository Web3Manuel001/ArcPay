package split

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/arcpay/core/internal/identity"
	"github.com/arcpay/core/internal/jev"
	"github.com/arcpay/core/internal/ledger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SplitSession struct {
	ID                  uuid.UUID          `json:"id"`
	SplitCode           string             `json:"split_code"`
	CreatorTag          string             `json:"creator_tag"`
	TotalAmountUnits    int64              `json:"total_amount_units"`
	SharePerPersonUnits int64              `json:"share_per_person_units"`
	TotalAmountUSD      float64            `json:"total_amount_usd"`
	SharePerPersonUSD   float64            `json:"share_per_person_usd"`
	TotalAmountNGN      float64            `json:"total_amount_ngn"`
	SharePerPersonNGN   float64            `json:"share_per_person_ngn"`
	Narration           string             `json:"narration"`
	ShareableLink       string             `json:"shareable_link"`
	Status              string             `json:"status"`
	Participants        []SplitParticipant `json:"participants"`
	CreatedAt           time.Time          `json:"created_at"`
}

type SplitParticipant struct {
	ID             uuid.UUID `json:"id"`
	ParticipantTag string    `json:"participant_tag"`
	PhoneOrEmail   string    `json:"phone_or_email"`
	AmountUnits    int64     `json:"amount_units"`
	Status         string    `json:"status"`
	PaymentMethod  string    `json:"payment_method,omitempty"`
}

type Service struct {
	pool            *pgxpool.Pool
	jevEngine       *jev.Engine
	identityService *identity.Service
	ledgerService   *ledger.Service
	baseURL         string
}

func NewService(pool *pgxpool.Pool, jevEngine *jev.Engine, identityService *identity.Service, ledgerService *ledger.Service, baseURL string) *Service {
	return &Service{
		pool:            pool,
		jevEngine:       jevEngine,
		identityService: identityService,
		ledgerService:   ledgerService,
		baseURL:         baseURL,
	}
}

func generateSplitCode() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("spl_%s", hex.EncodeToString(b))
}

func (s *Service) CreateSplit(ctx context.Context, creatorTag string, rawInput string) (*SplitSession, error) {
	intent, err := s.jevEngine.ParseSplitIntent(ctx, rawInput)
	if err != nil {
		return nil, fmt.Errorf("jev parser error: %w", err)
	}

	cleanCreator := identity.CleanTag(creatorTag)
	_, err = s.identityService.ResolveTag(ctx, cleanCreator)
	if err != nil {
		return nil, fmt.Errorf("creator tag not found: %w", err)
	}

	totalPeople := int64(len(intent.Participants) + 1)
	totalUnits := int64(intent.TotalAmountUSD * 1_000_000)
	shareUnits := totalUnits / totalPeople

	splitCode := generateSplitCode()
	sessionID := uuid.New()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	insertSessionSQL := `
		INSERT INTO split_sessions (id, split_code, creator_tag, total_amount_units, share_per_person_units, narration, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'OPEN', NOW())
	`
	_, err = tx.Exec(ctx, insertSessionSQL, sessionID, splitCode, cleanCreator, totalUnits, shareUnits, intent.Narration)
	if err != nil {
		return nil, fmt.Errorf("failed to save split session: %w", err)
	}

	insertPartSQL := `
		INSERT INTO split_participants (id, split_id, participant_tag, phone_or_email, amount_units, status)
		VALUES ($1, $2, $3, $4, $5, 'PENDING')
	`
	var participants []SplitParticipant
	for _, p := range intent.Participants {
		cleanP := identity.CleanTag(p)
		partID := uuid.New()
		_, err = tx.Exec(ctx, insertPartSQL, partID, sessionID, cleanP, cleanP, shareUnits)
		if err != nil {
			return nil, fmt.Errorf("failed to save participant: %w", err)
		}
		participants = append(participants, SplitParticipant{
			ID:             partID,
			ParticipantTag: cleanP,
			PhoneOrEmail:   cleanP,
			AmountUnits:    shareUnits,
			Status:         "PENDING",
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	shareableURL := fmt.Sprintf("%s/split/%s", s.baseURL, splitCode)
	shareUSD := float64(shareUnits) / 1_000_000.0

	return &SplitSession{
		ID:                  sessionID,
		SplitCode:           splitCode,
		CreatorTag:          cleanCreator,
		TotalAmountUnits:    totalUnits,
		SharePerPersonUnits: shareUnits,
		TotalAmountUSD:      intent.TotalAmountUSD,
		SharePerPersonUSD:   shareUSD,
		TotalAmountNGN:      intent.TotalAmountUSD * 1650.0,
		SharePerPersonNGN:   shareUSD * 1650.0,
		Narration:           intent.Narration,
		ShareableLink:       shareableURL,
		Status:              "OPEN",
		Participants:        participants,
		CreatedAt:           time.Now(),
	}, nil
}

func (s *Service) GetSplitByCode(ctx context.Context, splitCode string) (*SplitSession, error) {
	query := `
		SELECT id, split_code, creator_tag, total_amount_units, share_per_person_units, narration, status, created_at
		FROM split_sessions
		WHERE split_code = $1
	`
	var sess SplitSession
	err := s.pool.QueryRow(ctx, query, splitCode).Scan(
		&sess.ID, &sess.SplitCode, &sess.CreatorTag, &sess.TotalAmountUnits, 
		&sess.SharePerPersonUnits, &sess.Narration, &sess.Status, &sess.CreatedAt,
	)
	if err != nil {
		return nil, errors.New("split session not found")
	}

	sess.TotalAmountUSD = float64(sess.TotalAmountUnits) / 1_000_000.0
	sess.SharePerPersonUSD = float64(sess.SharePerPersonUnits) / 1_000_000.0
	sess.TotalAmountNGN = sess.TotalAmountUSD * 1650.0
	sess.SharePerPersonNGN = sess.SharePerPersonUSD * 1650.0
	sess.ShareableLink = fmt.Sprintf("%s/split/%s", s.baseURL, sess.SplitCode)

	// Fetch participants
	pQuery := `
		SELECT id, COALESCE(participant_tag, ''), phone_or_email, amount_units, status, COALESCE(payment_method, '')
		FROM split_participants
		WHERE split_id = $1
	`
	rows, err := s.pool.Query(ctx, pQuery, sess.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p SplitParticipant
			_ = rows.Scan(&p.ID, &p.ParticipantTag, &p.PhoneOrEmail, &p.AmountUnits, &p.Status, &p.PaymentMethod)
			sess.Participants = append(sess.Participants, p)
		}
	}

	return &sess, nil
}

func (s *Service) PaySplitShare(ctx context.Context, splitCode, payerIdentifier, paymentMethod string) error {
	sess, err := s.GetSplitByCode(ctx, splitCode)
	if err != nil {
		return err
	}
	if sess.Status != "OPEN" {
		return errors.New("split session is closed")
	}

	creator, err := s.identityService.ResolveTag(ctx, sess.CreatorTag)
	if err != nil {
		return fmt.Errorf("creator unresolvable: %w", err)
	}

	if paymentMethod == "ARCPAY_BALANCE" {
		payerTag := identity.CleanTag(payerIdentifier)
		payer, err := s.identityService.ResolveTag(ctx, payerTag)
		if err != nil {
			return fmt.Errorf("payer not found: %w", err)
		}

		payerBal, _ := s.ledgerService.GetBalance(ctx, payer.AccountID)
		if payerBal < sess.SharePerPersonUnits {
			return errors.New("insufficient balance")
		}

		idempotencyKey := fmt.Sprintf("split_pay_%s_%s", splitCode, payerTag)
		_, err = s.ledgerService.ProcessTransaction(ctx, ledger.TransactionPayload{
			IdempotencyKey: idempotencyKey,
			ReferenceType:  "ARC_SPLIT",
			Narration:      fmt.Sprintf("Split payment for %s", splitCode),
			Corridor:       creator.Corridor,
			Postings: []ledger.PostingInstruction{
				{AccountID: payer.AccountID, AmountUnits: sess.SharePerPersonUnits, Direction: ledger.Debit},
				{AccountID: creator.AccountID, AmountUnits: sess.SharePerPersonUnits, Direction: ledger.Credit},
			},
		})
		if err != nil {
			return err
		}
	} else {
		// External Web2 Card / Paystack
		inboundFloatID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
		idempotencyKey := fmt.Sprintf("split_ps_%s_%d", splitCode, time.Now().UnixNano())
		_, err = s.ledgerService.ProcessTransaction(ctx, ledger.TransactionPayload{
			IdempotencyKey: idempotencyKey,
			ReferenceType:  "ARC_SPLIT_PAYSTACK",
			Narration:      fmt.Sprintf("Paystack Card Split for %s", splitCode),
			Corridor:       creator.Corridor,
			Postings: []ledger.PostingInstruction{
				{AccountID: creator.AccountID, AmountUnits: sess.SharePerPersonUnits, Direction: ledger.Credit},
				{AccountID: inboundFloatID, AmountUnits: sess.SharePerPersonUnits, Direction: ledger.Debit},
			},
		})
		if err != nil {
			return err
		}
		log.Printf("💳 Paystack Split Settled: %d units credited to $%s", sess.SharePerPersonUnits, sess.CreatorTag)
	}

	updateSQL := `
		UPDATE split_participants 
		SET status = 'PAID', payment_method = $1, paid_at = NOW() 
		WHERE split_id = $2 AND (participant_tag = $3 OR phone_or_email = $3)
	`
	_, _ = s.pool.Exec(ctx, updateSQL, paymentMethod, sess.ID, identity.CleanTag(payerIdentifier))

	return nil
}
