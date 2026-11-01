package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arcpay/core/internal/auth"
	"github.com/arcpay/core/internal/blockchain"
	"github.com/arcpay/core/internal/identity"
	"github.com/arcpay/core/internal/ledger"
	"github.com/arcpay/core/internal/paystack"
	"github.com/arcpay/core/internal/pos"
	"github.com/arcpay/core/internal/redis"
	"github.com/arcpay/core/internal/split"
	"github.com/arcpay/core/internal/trade"
	"github.com/arcpay/core/internal/vas"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Handler struct {
	ledgerService    *ledger.Service
	identityService  *identity.Service
	authService      *auth.Service
	vasService       *vas.Service
	splitService     *split.Service
	tradeService     *trade.Service
	blockchainClient *blockchain.Client
	paystackClient   *paystack.Client
	redisClient      *redis.Client
	posHub           *pos.Hub
}

func NewHandler(
	ledgerService *ledger.Service,
	identityService *identity.Service,
	authService *auth.Service,
	vasService *vas.Service,
	splitService *split.Service,
	tradeService *trade.Service,
	blockchainClient *blockchain.Client,
	paystackClient *paystack.Client,
	redisClient *redis.Client,
	posHub *pos.Hub,
) *Handler {
	return &Handler{
		ledgerService:    ledgerService,
		identityService:  identityService,
		authService:      authService,
		vasService:       vasService,
		splitService:     splitService,
		tradeService:     tradeService,
		blockchainClient: blockchainClient,
		paystackClient:   paystackClient,
		redisClient:      redisClient,
		posHub:           posHub,
	}
}

// --- ARCPAY TRADE (B2B IMPORTER ESCROW) ---

func (h *Handler) CreateTradeInvoice(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req trade.CreateTradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid trade payload")
		return
	}

	invoice, err := h.tradeService.CreateTradeInvoice(r.Context(), claims.ArcTag, req)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusCreated, invoice)
}

func (h *Handler) FundTradeEscrow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	code := chi.URLParam(r, "code")
	err := h.tradeService.FundTradeEscrow(r.Context(), code, claims.ArcTag)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"status":  "IN_ESCROW",
		"message": "Funds locked securely in escrow until shipping dispatch confirmation",
	})
}

func (h *Handler) ReleaseTradeEscrow(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	code := chi.URLParam(r, "code")
	err := h.tradeService.ReleaseTradeEscrow(r.Context(), code, claims.ArcTag)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"status":  "RELEASED",
		"message": "Escrow released to overseas supplier",
	})
}

// --- BLOCKCHAIN STATUS ---

func (h *Handler) GetNetworkStatus(w http.ResponseWriter, r *http.Request) {
	status := h.blockchainClient.GetStatus(r.Context())
	JSON(w, http.StatusOK, status)
}

// --- AUTHENTICATED P2P TRANSFER ---

type SecureTransferRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	RecipientTag   string `json:"recipient_tag"`
	AmountUnits    int64  `json:"amount_units"`
	Narration      string `json:"narration"`
}

func (h *Handler) TransferByTag(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized session")
		return
	}

	var req SecureTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid transfer payload")
		return
	}

	if req.IdempotencyKey == "" {
		Error(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}
	if req.AmountUnits <= 0 {
		Error(w, http.StatusBadRequest, "transfer amount must be greater than zero")
		return
	}

	recipient, err := h.identityService.ResolveTag(r.Context(), req.RecipientTag)
	if err != nil {
		Error(w, http.StatusNotFound, fmt.Sprintf("recipient tag '%s' does not exist", req.RecipientTag))
		return
	}

	if claims.AccountID == recipient.AccountID {
		Error(w, http.StatusBadRequest, "cannot transfer money to yourself")
		return
	}

	acquired, err := h.redisClient.AcquireLock(r.Context(), req.IdempotencyKey, 10*time.Second)
	if err != nil || !acquired {
		Error(w, http.StatusConflict, "transaction currently processing or already executed")
		return
	}

	entry, err := h.ledgerService.ProcessTransaction(r.Context(), ledger.TransactionPayload{
		IdempotencyKey: req.IdempotencyKey,
		ReferenceType:  "TAG_TRANSFER",
		Narration:      fmt.Sprintf("Transfer from $%s to $%s: %s", claims.ArcTag, recipient.ArcTag, req.Narration),
		Corridor:       "NG",
		Postings: []ledger.PostingInstruction{
			{AccountID: claims.AccountID, AmountUnits: req.AmountUnits, Direction: ledger.Debit},
			{AccountID: recipient.AccountID, AmountUnits: req.AmountUnits, Direction: ledger.Credit},
		},
	})
	if err != nil {
		_ = h.redisClient.ReleaseLock(r.Context(), req.IdempotencyKey)
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	onchainTx := h.blockchainClient.FormatOnchainTxHash(entry.ID.String())

	JSON(w, http.StatusCreated, map[string]any{
		"journal_id":    entry.ID,
		"from":          "$" + claims.ArcTag,
		"to":            "$" + recipient.ArcTag,
		"amount_usdc":   float64(req.AmountUnits) / 1_000_000.0,
		"amount_units":  req.AmountUnits,
		"onchain_tx":    onchainTx,
		"block_network": "Arc L1 Testnet",
		"status":        "SETTLED",
	})
}

// --- PAYSTACK NAIRA DEPOSIT ---

type InitDepositRequest struct {
	Tag       string  `json:"tag"`
	AmountNGN float64 `json:"amount_ngn"`
}

func (h *Handler) InitializeDeposit(w http.ResponseWriter, r *http.Request) {
	var req InitDepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid deposit payload")
		return
	}

	cleanTag := identity.CleanTag(req.Tag)
	if _, err := h.identityService.ResolveTag(r.Context(), cleanTag); err != nil {
		Error(w, http.StatusNotFound, "user not found: "+cleanTag)
		return
	}

	reference := fmt.Sprintf("paystack_dep_%s_%d", cleanTag, time.Now().Unix())
	amountKobo := int64(req.AmountNGN * 100)
	callbackURL := "https://test-arcpay.vercel.app/?verify_ref=" + reference

	if h.paystackClient != nil {
		resp, err := h.paystackClient.InitializeTransaction(r.Context(), "user@arcpay.me", amountKobo, reference, callbackURL)
		if err != nil {
			Error(w, http.StatusBadGateway, err.Error())
			return
		}
		JSON(w, http.StatusOK, map[string]any{
			"authorization_url": resp.Data.AuthorizationURL,
			"reference":         reference,
			"amount_ngn":        req.AmountNGN,
		})
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"authorization_url": callbackURL,
		"reference":         reference,
		"amount_ngn":        req.AmountNGN,
	})
}

func (h *Handler) VerifyDeposit(w http.ResponseWriter, r *http.Request) {
	reference := chi.URLParam(r, "reference")
	if reference == "" {
		Error(w, http.StatusBadRequest, "reference required")
		return
	}

	parts := strings.Split(reference, "_")
	var targetTag string
	if len(parts) >= 3 {
		targetTag = parts[2]
	} else {
		targetTag = "emma"
	}

	user, err := h.identityService.ResolveTag(r.Context(), targetTag)
	if err != nil {
		Error(w, http.StatusNotFound, "user not found: "+targetTag)
		return
	}

	var amountNGN float64 = 16500.0
	if h.paystackClient != nil {
		vResp, err := h.paystackClient.VerifyTransaction(r.Context(), reference)
		if err != nil || vResp.Data.Status != "success" {
			Error(w, http.StatusPaymentRequired, "paystack verification failed")
			return
		}
		amountNGN = float64(vResp.Data.Amount) / 100.0
	}

	usdcAmount := amountNGN / 1650.0
	units := int64(usdcAmount * 1_000_000)
	inboundFloatID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	_, err = h.ledgerService.ProcessTransaction(r.Context(), ledger.TransactionPayload{
		IdempotencyKey: reference,
		ReferenceType:  "PAYSTACK_DEPOSIT",
		Narration:      fmt.Sprintf("Paystack Deposit ₦%.2f (Credited $%.2f USDC)", amountNGN, usdcAmount),
		Corridor:       user.Corridor,
		Postings: []ledger.PostingInstruction{
			{AccountID: user.AccountID, AmountUnits: units, Direction: ledger.Credit},
			{AccountID: inboundFloatID, AmountUnits: units, Direction: ledger.Debit},
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate key") {
			bal, _ := h.ledgerService.GetBalance(r.Context(), user.AccountID)
			JSON(w, http.StatusOK, map[string]any{
				"status":        "ALREADY_CREDITED",
				"tag":           "$" + targetTag,
				"credited_usdc": usdcAmount,
				"new_balance":   float64(bal) / 1_000_000.0,
			})
			return
		}
		Error(w, http.StatusInternalServerError, "ledger deposit failed: "+err.Error())
		return
	}

	bal, _ := h.ledgerService.GetBalance(r.Context(), user.AccountID)

	JSON(w, http.StatusOK, map[string]any{
		"status":        "CREDITED",
		"tag":           "$" + targetTag,
		"credited_usdc": usdcAmount,
		"new_balance":   float64(bal) / 1_000_000.0,
	})
}

// --- HISTORY & BALANCE ---

func (h *Handler) GetAccountHistory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	accountID, err := uuid.Parse(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid account uuid")
		return
	}

	items, err := h.ledgerService.GetHistory(r.Context(), accountID, 20)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	JSON(w, http.StatusOK, items)
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

	if balanceUnits < 0 {
		balanceUnits = 0
	}

	JSON(w, http.StatusOK, map[string]any{
		"account_id":     accountID,
		"balance_units":  balanceUnits,
		"balance_amount": float64(balanceUnits) / 1_000_000.0,
		"currency":       "USDC",
	})
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

func (h *Handler) GetSplitSession(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	sess, err := h.splitService.GetSplitByCode(r.Context(), code)
	if err != nil {
		Error(w, http.StatusNotFound, "split session not found")
		return
	}
	JSON(w, http.StatusOK, sess)
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

// --- VAS AIRTIME ---

func (h *Handler) PurchaseAirtime(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req vas.AirtimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid airtime payload")
		return
	}

	req.SenderTag = claims.ArcTag // Strictly use verified tag

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

// --- AUTH ---

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

// --- POS TERMINAL ---

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
