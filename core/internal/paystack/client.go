package paystack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	secretKey string
	client    *http.Client
}

func NewClient(secretKey string) *Client {
	return &Client{
		secretKey: secretKey,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

type InitResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AuthorizationURL string `json:"authorization_url"`
		AccessCode       string `json:"access_code"`
		Reference        string `json:"reference"`
	} `json:"data"`
}

type VerifyResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Status    string `json:"status"` // "success"
		Reference string `json:"reference"`
		Amount    int64  `json:"amount"` // in Kobo (e.g. 100000 kobo = 1000 NGN)
		Currency  string `json:"currency"`
	} `json:"data"`
}

func (c *Client) InitializeTransaction(ctx context.Context, email string, amountKobo int64, reference, callbackURL string) (*InitResponse, error) {
	payload := map[string]any{
		"email":        email,
		"amount":       amountKobo,
		"reference":    reference,
		"callback_url": callbackURL,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.paystack.co/transaction/initialize", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var initResp InitResponse
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		return nil, err
	}
	if !initResp.Status {
		return nil, fmt.Errorf("paystack initialization failed: %s", initResp.Message)
	}

	return &initResp, nil
}

func (c *Client) VerifyTransaction(ctx context.Context, reference string) (*VerifyResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.paystack.co/transaction/verify/"+reference, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var verifyResp VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return nil, err
	}
	return &verifyResp, nil
}
