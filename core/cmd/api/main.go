package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arcpay/core/internal/api"
	"github.com/arcpay/core/internal/config"
	"github.com/arcpay/core/internal/database"
	"github.com/arcpay/core/internal/identity"
	"github.com/arcpay/core/internal/jev"
	"github.com/arcpay/core/internal/ledger"
	"github.com/arcpay/core/internal/pos"
	"github.com/arcpay/core/internal/redis"
	"github.com/arcpay/core/internal/split"
	"github.com/arcpay/core/internal/vas"
	"github.com/google/uuid"
)

func main() {
	log.Println("🚀 Initializing Arcpay Core Engine on Arc L1...")

	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pgPool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("❌ Database connection error: %v", err)
	}
	defer pgPool.Close()

	redisClient, err := redis.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("❌ Redis connection error: %v", err)
	}
	log.Println("✓ Connected to Redis Distributed Lock Engine")

	posHub := pos.NewHub()
	go posHub.Run()
	log.Println("✓ Real-Time POS WebSocket Hub Active")

	ledgerRepo := ledger.NewPostgresRepository(pgPool)
	ledgerService := ledger.NewService(ledgerRepo)

	identityRepo := identity.NewRepository(pgPool)
	identityService := identity.NewService(identityRepo)

	// Live Africa's Talking Client
	var atClient *vas.AfricasTalkingClient
	if cfg.ATAPIKey != "" {
		atClient = vas.NewAfricasTalkingClient(cfg.ATUsername, cfg.ATAPIKey, cfg.IsSandbox)
		log.Println("✓ Real Telecom Switch Connected (Africa's Talking API)")
	} else {
		log.Println("⚠️ AT_API_KEY not set in .env. Telco dispatch will simulate onchain settlement.")
	}

	treasuryAccount := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	vasService := vas.NewService(ledgerService, identityService, atClient, treasuryAccount)

	// Jev Decision Engine & ArcSplit Service
	jevEngine := jev.NewEngine()
	splitService := split.NewService(pgPool, jevEngine, identityService, ledgerService, "https://arcpay.me")
	log.Println("✓ Jev System-1 Decision Engine & ArcSplit Active")

	httpHandler := api.NewHandler(ledgerService, identityService, vasService, splitService, redisClient, posHub)

	router := api.NewRouter(httpHandler)
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("⚡ Arcpay API Gateway live on http://localhost:%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Arcpay Core gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("✓ Arcpay Core cleanly terminated.")
}
