package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/arcpay/core/internal/auth"
	"github.com/arcpay/core/internal/identity"
	"github.com/arcpay/core/internal/ledger"
	"github.com/arcpay/core/internal/pos"
	"github.com/arcpay/core/internal/redis"
	"github.com/arcpay/core/internal/split"
	"github.com/arcpay/core/internal/vas"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Handler struct {
	ledgerService   *ledger.Service
	identityService *identity.Service
	authService     *auth.Service
	vasService      *vas.Service
	splitService    *split.Service
	redisClient     *redis.Client
	posHub          *pos.Hub
}

func NewHandler(
	ledgerService *ledger.Service,
	identityService *identity.Service,
	authService *auth.Service,
	vasService *vas.Service,
	splitService *split.Service,
	redisClient *redis.Client,
	posHub *pos.Hub,
) *Handler {
	return &Handler{
		ledgerService:   ledgerService,
		identityService: identityService,
		authService:     authService,
		vasService:      vasService,
		splitService:    splitService,
		redisClient:     redisClient,
		posHub:          posHub,
	}
}

// --- SANDBOX TEST FAUCET ---

type FaucetRequest struct {
	Tag string `json:"tag"` // e.g. "$emmanuel"
}

func (h *Handler) FaucetTopUp(w http.ResponseWriter, r *http.Request) {
	var req FaucetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid faucet payload")
		return
	}

	cleanTag := identity.CleanTag(req.Tag)
	user, err := h.identityService.ResolveTag(r.Context(), cleanTag)
	if err != nil {
		Error(w, http.StatusNotFound, "user tag not found")
		return
	}

	// 100 USDC = 100,000,000 micro-units
	const faucetAmount = 100_000_000
	faucetReserveID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	// Ensure system reserve exists
	_, _ = h.ledgerService.CreateAccount(r.Context(), faucetReserveID, "GLOBAL", "USDC", ledger.Equity)

	idempotencyKey := fmt.Sprintf("faucet_%s_%d", cleanTag, time.Now().UnixNano())
	entry, err := h.ledgerService.ProcessTransaction(r.Context(), ledger.TransactionPayload{
		IdempotencyKey: idempotencyKey,
		ReferenceType:  "SANDBOX_FAUCET",
		Narration:      fmt.Sprintf("Sandbox Test Funds: $100.00 USDC credited to $%s", cleanTag),
		Corridor:       user.Corridor,
		Postings: []ledger.PostingInstruction{
			{AccountID: user.AccountID, AmountUnits: faucetAmount, Direction: ledger.Debit},
			{AccountID: faucetReserveID, AmountUnits: faucetAmount, Direction: ledger.Credit},
		},
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	balanceUnits, _ := h.ledgerService.GetBalance(r.Context(), user.AccountID)

	JSON(w, http.StatusOK, map[string]any{
		"tx_id":          entry.ID,
		"tag":            "$" + cleanTag,
		"credited_usdc":  100.0,
		"new_balance":    float64(balanceUnits) / 1_000_000.0,
		"balance_units":  balanceUnits,
	})
}

// --- AUTH HANDLERS ---

func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var req auth.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid registration request")
		return
	}

	resp, err := h.authService.Register(r.Context(), req)
	if err != nil {
		Error(w, http.StatusConflict, err.Error())
		return
	}

	JSON(w, http.StatusCreated, resp)
}

func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid login request")
		return
	}

	resp, err := h.authService.Login(r.Context(), req)
	if err != nil {
		Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	JSON(w, http.StatusOK, resp)
}

// --- ARCSPLIT HANDLERS ---

type CreateSplitRequest struct {
	CreatorTag string `json:"creator_tag"`
	Prompt     string `json:"prompt"`
}

func (h *Handler) CreateSplitSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSplitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid split request payload")
		return
	}

	session, err := h.splitService.CreateSplit(r.Context(), req.CreatorTag, req.Prompt)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusCreated, session)
}

type PaySplitRequest struct {
	PayerIdentifier string `json:"payer_identifier"`
	PaymentMethod   string `json:"payment_method"`
}

func (h *Handler) PaySplitShare(w http.ResponseWriter, r *http.Request) {
	splitCode := chi.URLParam(r, "code")
	var req PaySplitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid pay payload")
		return
	}

	err := h.splitService.PaySplitShare(r.Context(), splitCode, req.PayerIdentifier, req.PaymentMethod)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"status":  "PAID",
		"message": "Share successfully settled to creator's balance",
	})
}

// --- VAS (AIRTIME) HANDLERS ---

func (h *Handler) PurchaseAirtime(w http.ResponseWriter, r *http.Request) {
	var req vas.AirtimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid airtime payload")
		return
	}

	if req.IdempotencyKey == "" {
		Error(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}

	acquired, err := h.redisClient.AcquireLock(r.Context(), req.IdempotencyKey, 10*time.Second)
	if err != nil || !acquired {
		Error(w, http.StatusConflict, "transaction currently processing or already executed")
		return
	}

	receipt, err := h.vasService.PurchaseAirtime(r.Context(), req)
	if err != nil {
		_ = h.redisClient.ReleaseLock(r.Context(), req.IdempotencyKey)
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusCreated, receipt)
}

// --- SOCIAL IDENTITY HANDLERS ---

func (h *Handler) RegisterTag(w http.ResponseWriter, r *http.Request) {
	var req identity.RegisterTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid tag registration payload")
		return
	}

	id, err := h.identityService.RegisterTag(r.Context(), req)
	if err != nil {
		Error(w, http.StatusConflict, err.Error())
		return
	}

	JSON(w, http.StatusCreated, id)
}

func (h *Handler) ResolveTag(w http.ResponseWriter, r *http.Request) {
	tag := chi.URLParam(r, "tag")
	id, err := h.identityService.ResolveTag(r.Context(), tag)
	if err != nil {
		Error(w, http.StatusNotFound, "ArcTag not found")
		return
	}
	JSON(w, http.StatusOK, id)
}

func (h *Handler) TransferByTag(w http.ResponseWriter, r *http.Request) {
	var req identity.TagTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid transfer payload")
		return
	}

	if req.IdempotencyKey == "" {
		Error(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}

	sender, err := h.identityService.ResolveTag(r.Context(), req.SenderTag)
	if err != nil {
		Error(w, http.StatusNotFound, fmt.Sprintf("sender tag '%s' does not exist", req.SenderTag))
		return
	}

	recipient, err := h.identityService.ResolveTag(r.Context(), req.RecipientTag)
	if err != nil {
		Error(w, http.StatusNotFound, fmt.Sprintf("recipient tag '%s' does not exist", req.RecipientTag))
		return
	}

	acquired, err := h.redisClient.AcquireLock(r.Context(), req.IdempotencyKey, 10*time.Second)
	if err != nil || !acquired {
		Error(w, http.StatusConflict, "transaction locked or already executing")
		return
	}

	entry, err := h.ledgerService.ProcessTransaction(r.Context(), ledger.TransactionPayload{
		IdempotencyKey: req.IdempotencyKey,
		ReferenceType:  "TAG_TRANSFER",
		Narration:      fmt.Sprintf("Transfer from $%s to $%s: %s", sender.ArcTag, recipient.ArcTag, req.Narration),
		Corridor:       sender.Corridor,
		Postings: []ledger.PostingInstruction{
			{AccountID: sender.AccountID, AmountUnits: req.AmountUnits, Direction: ledger.Debit},
			{AccountID: recipient.AccountID, AmountUnits: req.AmountUnits, Direction: ledger.Credit},
		},
	})
	if err != nil {
		_ = h.redisClient.ReleaseLock(r.Context(), req.IdempotencyKey)
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"journal_id":   entry.ID,
		"from":         "$" + sender.ArcTag,
		"to":           "$" + recipient.ArcTag,
		"amount_usdc":  float64(req.AmountUnits) / 1_000_000.0,
		"amount_units": req.AmountUnits,
		"status":       "SETTLED",
	})
}

// --- BASIC LEDGER & POS HANDLERS ---

type CreateAccountRequest struct {
	HolderID    uuid.UUID          `json:"holder_id"`
	Corridor    string             `json:"corridor"`
	Currency    string             `json:"currency"`
	AccountType ledger.AccountType `json:"account_type"`
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	acc, err := h.ledgerService.CreateAccount(r.Context(), req.HolderID, req.Corridor, req.Currency, req.AccountType)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	JSON(w, http.StatusCreated, acc)
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	accountID, err := uuid.Parse(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid account uuid")
		return
	}

	balanceUnits, err := h.ledgerService.GetBalance(r.Context(), accountID)
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	balanceDecimal := float64(balanceUnits) / 1_000_000.0

	JSON(w, http.StatusOK, map[string]any{
		"account_id":     accountID,
		"balance_units":  balanceUnits,
		"balance_amount": balanceDecimal,
		"currency":       "USDC",
	})
}

type TransferRequest struct {
	IdempotencyKey string                      `json:"idempotency_key"`
	ReferenceType  string                      `json:"reference_type"`
	InvoiceID      string                      `json:"invoice_id,omitempty"`
	Narration      string                      `json:"narration"`
	Corridor       string                      `json:"corridor"`
	Postings       []ledger.PostingInstruction `json:"postings"`
}

func (h *Handler) ProcessTransfer(w http.ResponseWriter, r *http.Request) {
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if req.IdempotencyKey == "" {
		Error(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}

	acquired, err := h.redisClient.AcquireLock(r.Context(), req.IdempotencyKey, 10*time.Second)
	if err != nil || !acquired {
		Error(w, http.StatusConflict, "transaction locked or already processed")
		return
	}

	entry, err := h.ledgerService.ProcessTransaction(r.Context(), ledger.TransactionPayload{
		IdempotencyKey: req.IdempotencyKey,
		ReferenceType:  req.ReferenceType,
		Narration:      req.Narration,
		Corridor:       req.Corridor,
		Postings:       req.Postings,
	})
	if err != nil {
		_ = h.redisClient.ReleaseLock(r.Context(), req.IdempotencyKey)
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if req.InvoiceID != "" {
		h.posHub.BroadcastPayment(req.InvoiceID, entry.ID.String(), req.Postings[0].AmountUnits)
	}

	JSON(w, http.StatusCreated, entry)
}

type CreateInvoiceRequest struct {
	MerchantAccount uuid.UUID `json:"merchant_account_id"`
	AmountUnits     int64     `json:"amount_units"`
	TableNumber     string    `json:"table_number"`
}

func (h *Handler) CreatePOSInvoice(w http.ResponseWriter, r *http.Request) {
	var req CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid invoice request")
		return
	}

	invoice := pos.GenerateDynamicInvoice(req.MerchantAccount, req.AmountUnits, req.TableNumber)
	JSON(w, http.StatusCreated, invoice)
}

func (h *Handler) StreamPOSUpdates(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoice_id")
	if invoiceID == "" {
		http.Error(w, "missing invoice_id", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &pos.TerminalClient{
		InvoiceID: invoiceID,
		Conn:      conn,
		Send:      make(chan []byte, 16),
	}

	h.posHub.RegisterClient(client)

	go func() {
		defer func() {
			h.posHub.UnregisterClient(client)
			conn.Close()
		}()
		for msg := range client.Send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}()
}
