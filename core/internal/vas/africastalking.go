package vas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AfricasTalkingClient struct {
	username string
	apiKey   string
	baseURL  string
	client   *http.Client
}

func NewAfricasTalkingClient(username, apiKey string, isSandbox bool) *AfricasTalkingClient {
	baseURL := "https://api.africastalking.com/version1"
	if isSandbox {
		baseURL = "https://api.sandbox.africastalking.com/version1"
	}

	return &AfricasTalkingClient{
		username: username,
		apiKey:   apiKey,
		baseURL:  baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type ATRoutingResponse struct {
	ErrorMessage string `json:"errorMessage"`
	NumSent      int    `json:"numSent"`
	TotalAmount  string `json:"totalAmount"`
	TotalDiscount string `json:"totalDiscount"`
	Responses    []struct {
		PhoneNumber  string `json:"phoneNumber"`
		ErrorMessage string `json:"errorMessage"`
		Amount       string `json:"amount"`
		Status       string `json:"status"` // "Sent", "Failed"
		RequestId    string `json:"requestId"`
		Discount     string `json:"discount"`
	} `json:"responses"`
}

// DispatchAirtime sends real airtime via Africa's Talking telecom switches
func (at *AfricasTalkingClient) DispatchAirtime(ctx context.Context, phone string, amountFormatted string) (*ATRoutingResponse, error) {
	endpoint := fmt.Sprintf("%s/airtime/send", at.baseURL)

	// Format recipient payload: [{"phoneNumber":"+2348012345678","amount":"NGN 1000"}]
	recipientJSON := fmt.Sprintf(`[{"phoneNumber":"%s","amount":"%s"}]`, phone, amountFormatted)

	form := url.Values{}
	form.Set("username", at.username)
	form.Set("recipients", recipientJSON)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to build AT request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("apiKey", at.apiKey)

	resp, err := at.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telecom switch network error: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read telco response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("telco switch rejected request [%d]: %s", resp.StatusCode, string(bodyBytes))
	}

	var atResp ATRoutingResponse
	if err := json.Unmarshal(bodyBytes, &atResp); err != nil {
		return nil, fmt.Errorf("failed to parse telco response: %w", err)
	}

	return &atResp, nil
}
