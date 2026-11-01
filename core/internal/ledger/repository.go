package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDuplicateIdempotency = errors.New("transaction with this idempotency key already exists")
	ErrAccountNotFound      = errors.New("account not found or inactive")
)

type Repository interface {
	CreateAccount(ctx context.Context, acc *Account) error
	GetAccount(ctx context.Context, id uuid.UUID) (*Account, error)
	GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error)
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
			a.account_type,
			COALESCE(SUM(CASE WHEN p.direction = 'DEBIT' THEN p.amount_units ELSE 0 END), 0) as total_debits,
			COALESCE(SUM(CASE WHEN p.direction = 'CREDIT' THEN p.amount_units ELSE 0 END), 0) as total_credits
		FROM accounts a
		LEFT JOIN postings p ON a.id = p.account_id
		WHERE a.id = $1
		GROUP BY a.account_type
	`
	var accType AccountType
	var debits, credits int64

	err := r.pool.QueryRow(ctx, query, accountID).Scan(&accType, &debits, &credits)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrAccountNotFound
		}
		return 0, err
	}

	switch accType {
	case Asset, Expense:
		return debits - credits, nil
	default:
		return credits - debits, nil
	}
}

// RecordTransaction executes atomic ACID postings with self-healing system accounts
func (r *PostgresRepository) RecordTransaction(ctx context.Context, payload TransactionPayload) (*JournalEntry, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// SELF-HEALING HOOK: Ensure system reserve accounts exist inside this exact transaction
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

	// 1. Insert Journal Entry
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

	// 2. Insert Postings
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

	// 3. Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &entry, nil
}
