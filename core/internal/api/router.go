package api

import (
	"net/http"

	"github.com/arcpay/core/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(handler *Handler, authMiddleware *auth.Middleware) http.Handler {
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

		// Public Balances & History
		r.Get("/accounts/{id}/balance", handler.GetBalance)
		r.Get("/accounts/{id}/history", handler.GetAccountHistory)

		// Public Split Invoicing & Paystack Callbacks
		r.Get("/consumer/split/{code}", handler.GetSplitSession)
		r.Post("/consumer/split/{code}/pay", handler.PaySplitShare)
		r.Post("/consumer/deposit/initialize", handler.InitializeDeposit)
		r.Get("/consumer/deposit/verify/{reference}", handler.VerifyDeposit)

		// PROTECTED ROUTES: Requires Authorization: Bearer <token>
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)

			r.Post("/consumer/transfers/tag", handler.TransferByTag)
			r.Post("/consumer/vas/airtime", handler.PurchaseAirtime)
			r.Post("/consumer/split/create", handler.CreateSplitSession)
		})

		// Merchant POS
		r.Route("/business/pos", func(r chi.Router) {
			r.Post("/invoices", handler.CreatePOSInvoice)
			r.Get("/ws/{invoice_id}", handler.StreamPOSUpdates)
		})
	})

	return r
}
