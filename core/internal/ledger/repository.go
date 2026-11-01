package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDuplicateIdempotency = errors.New("transaction with this idempotency key already exists")
	ErrAccountNotFound      = errors.New("account not found or inactive")
)

type HistoryItem struct {
	ID            uuid.UUID `json:"id"`
	ReferenceType string    `json:"reference_type"`
	Narration     string    `json:"narration"`
	AmountUnits   int64     `json:"amount_units"`
	AmountUSD     float64   `json:"amount_usd"`
	Direction     Direction `json:"direction"` // CREDIT = Money In (+), DEBIT = Money Out (-)
	CreatedAt     time.Time `json:"created_at"`
}

type Repository interface {
	CreateAccount(ctx context.Context, acc *Account) error
	GetAccount(ctx context.Context, id uuid.UUID) (*Account, error)
	GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error)
	GetHistory(ctx context.Context, accountID uuid.UUID, limit int) ([]HistoryItem, error)
	RecordTransaction(ctx context.Context, payload TransactionPayload) (*JournalEntry, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateAccount(ctx context.Context, acc *Account) error {
	query := `
		INSERT INTO accounts (id, holder_id, corridor, currency, account_type, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (id) DO NOTHING
	`
	if acc.ID == uuid.Nil {
		acc.ID = uuid.New()
	}
	_, err := r.pool.Exec(ctx, query, acc.ID, acc.HolderID, acc.Corridor, acc.Currency, acc.AccountType, acc.IsActive)
	return err
}

func (r *PostgresRepository) GetAccount(ctx context.Context, id uuid.UUID) (*Account, error) {
	query := `
		SELECT id, holder_id, corridor, currency, account_type, is_active, created_at
		FROM accounts
		WHERE id = $1
	`
	var acc Account
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&acc.ID, &acc.HolderID, &acc.Corridor, &acc.Currency, &acc.AccountType, &acc.IsActive, &acc.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	return &acc, nil
}

func (r *PostgresRepository) GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN p.direction = 'CREDIT' THEN p.amount_units ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN p.direction = 'DEBIT' THEN p.amount_units ELSE 0 END), 0) as net_balance
		FROM accounts a
		LEFT JOIN postings p ON a.id = p.account_id
		WHERE a.id = $1
	`
	var netBalance int64
	err := r.pool.QueryRow(ctx, query, accountID).Scan(&netBalance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrAccountNotFound
		}
		return 0, err
	}
	return netBalance, nil
}

func (r *PostgresRepository) GetHistory(ctx context.Context, accountID uuid.UUID, limit int) ([]HistoryItem, error) {
	query := `
		SELECT j.id, j.reference_type, j.narration, p.amount_units, p.direction, j.created_at
		FROM postings p
		JOIN journal_entries j ON p.journal_entry_id = j.id
		WHERE p.account_id = $1
		ORDER BY j.created_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []HistoryItem
	for rows.Next() {
		var item HistoryItem
		if err := rows.Scan(&item.ID, &item.ReferenceType, &item.Narration, &item.AmountUnits, &item.Direction, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.AmountUSD = float64(item.AmountUnits) / 1_000_000.0
		items = append(items, item)
	}
	return items, nil
}

func (r *PostgresRepository) RecordTransaction(ctx context.Context, payload TransactionPayload) (*JournalEntry, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	ensureSystemSQL := `
		INSERT INTO accounts (id, holder_id, corridor, currency, account_type, is_active, created_at)
		VALUES 
			('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'SYS', 'USDC', 'REVENUE', TRUE, NOW()),
			('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000002', 'SYS', 'USDC', 'EQUITY', TRUE, NOW())
		ON CONFLICT (id) DO NOTHING;
	`
	if _, err := tx.Exec(ctx, ensureSystemSQL); err != nil {
		return nil, fmt.Errorf("failed to ensure system accounts: %w", err)
	}

	entry := JournalEntry{
		ID:             uuid.New(),
		IdempotencyKey: payload.IdempotencyKey,
		ReferenceType:  payload.ReferenceType,
		Narration:      payload.Narration,
		Corridor:       payload.Corridor,
	}

	insertJournalQuery := `
		INSERT INTO journal_entries (id, idempotency_key, reference_type, narration, corridor, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`
	_, err = tx.Exec(ctx, insertJournalQuery, entry.ID, entry.IdempotencyKey, entry.ReferenceType, entry.Narration, entry.Corridor)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDuplicateIdempotency, err)
	}

	insertPostingQuery := `
		INSERT INTO postings (id, journal_entry_id, account_id, amount_units, direction, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`
	for _, p := range payload.Postings {
		postingID := uuid.New()
		_, err := tx.Exec(ctx, insertPostingQuery, postingID, entry.ID, p.AccountID, p.AmountUnits, p.Direction)
		if err != nil {
			return nil, fmt.Errorf("failed to record posting: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &entry, nil
}
