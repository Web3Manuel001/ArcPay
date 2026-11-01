package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(handler *Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, Idempotency-Key")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{
			"status":  "healthy",
			"network": "Arc L1",
		})
	})

	r.Route("/api/v1", func(r chi.Router) {
		// Public Auth
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", handler.RegisterUser)
			r.Post("/login", handler.LoginUser)
		})

		// Ledger & Accounts
		r.Post("/accounts", handler.CreateAccount)
		r.Get("/accounts/{id}/balance", handler.GetBalance)
		r.Post("/transactions", handler.ProcessTransfer)

		// Consumer Operations
		r.Route("/consumer", func(r chi.Router) {
			r.Post("/faucet", handler.FaucetTopUp) // SANDBOX TEST FAUCET
			r.Post("/identity/register", handler.RegisterTag)
			r.Get("/identity/resolve/{tag}", handler.ResolveTag)
			r.Post("/transfers/tag", handler.TransferByTag)
			r.Post("/vas/airtime", handler.PurchaseAirtime)
			r.Post("/split/create", handler.CreateSplitSession)
			r.Post("/split/{code}/pay", handler.PaySplitShare)
		})

		// Business Operations (POS)
		r.Route("/business/pos", func(r chi.Router) {
			r.Post("/invoices", handler.CreatePOSInvoice)
			r.Get("/ws/{invoice_id}", handler.StreamPOSUpdates)
		})
	})

	return r
}
