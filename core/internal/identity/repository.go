package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTagNotFound = errors.New("arc tag not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateIdentity(ctx context.Context, id *Identity) error {
	query := `
		INSERT INTO identities (id, arc_tag, phone_hash, holder_id, account_id, corridor, evm_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`
	if id.ID == uuid.Nil {
		id.ID = uuid.New()
	}
	_, err := r.pool.Exec(ctx, query, id.ID, id.ArcTag, id.PhoneHash, id.HolderID, id.AccountID, id.Corridor, id.EVMAddress)
	return err
}

func (r *Repository) GetByTag(ctx context.Context, tag string) (*Identity, error) {
	query := `
		SELECT id, arc_tag, phone_hash, holder_id, account_id, corridor, COALESCE(evm_address, ''), created_at
		FROM identities
		WHERE arc_tag = $1
	`
	var id Identity
	err := r.pool.QueryRow(ctx, query, tag).Scan(
		&id.ID, &id.ArcTag, &id.PhoneHash, &id.HolderID, &id.AccountID, &id.Corridor, &id.EVMAddress, &id.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTagNotFound
		}
		return nil, err
	}
	return &id, nil
}
