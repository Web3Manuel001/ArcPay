package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
		log.Fatalf("Connection failed: %v", err)
	}
	defer conn.Close(ctx)

	sql, err := os.ReadFile("migrations/000005_seed_system_accounts.up.sql")
	if err != nil {
		log.Fatalf("Failed to read migration: %v", err)
	}

	_, err = conn.Exec(ctx, string(sql))
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	fmt.Println("✓ Migration 000005 Applied: Treasury and Faucet Reserve accounts seeded into Supabase!")
}
