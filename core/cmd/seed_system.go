package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/arcpay/core/internal/config"
	"github.com/jackc/pgx/v5"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to Supabase: %v", err)
	}
	defer conn.Close(ctx)

	seedSQL := `
		INSERT INTO accounts (id, holder_id, corridor, currency, account_type, is_active, created_at)
		VALUES 
			('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'GLOBAL', 'USDC', 'REVENUE', TRUE, NOW()),
			('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000002', 'GLOBAL', 'USDC', 'EQUITY', TRUE, NOW())
		ON CONFLICT (id) DO NOTHING;
	`
	_, err = conn.Exec(ctx, seedSQL)
	if err != nil {
		log.Fatalf("❌ Failed to insert system accounts: %v", err)
	}

	// Verify insertion
	var count int
	_ = conn.QueryRow(ctx, "SELECT count(*) FROM accounts WHERE id IN ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002')").Scan(&count)

	fmt.Printf("✓ Supabase Verified: %d system accounts active in the database!\n", count)
}
