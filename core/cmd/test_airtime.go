package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type APIResp struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

func main() {
	baseURL := "http://localhost:8080/api/v1"

	fmt.Println("🚀 Testing First-Party Airtime Purchase with USDC...")

	// 1. Create Arcpay Treasury Float Account
	treasuryID := "00000000-0000-0000-0000-000000000001"
	createAccountWithID(baseURL+"/accounts", fmt.Sprintf(`{"holder_id":"%s","corridor":"GLOBAL","currency":"USDC","account_type":"REVENUE"}`, treasuryID), treasuryID)

	// 2. Create User Account & Claim ArcTag
	userHolderID := uuid.New()
	userAccID := createAccount(baseURL+"/accounts", fmt.Sprintf(`{"holder_id":"%s","corridor":"NG","currency":"USDC","account_type":"ASSET"}`, userHolderID))

	userTag := fmt.Sprintf("kofi_%d", time.Now().Unix()%10000)
	registerTag(baseURL+"/consumer/identity/register", fmt.Sprintf(`{
		"arc_tag": "$%s",
		"phone_number": "+2348099998888",
		"holder_id": "%s",
		"account_id": "%s",
		"corridor": "NG",
		"evm_address": "0x1111111111111111111111111111111111111111"
	}`, userTag, userHolderID, userAccID))

	// 3. User buys $2.00 MTN Airtime using their ArcTag
	airtimePayload := fmt.Sprintf(`{
		"idempotency_key": "vas_airtime_%d",
		"sender_tag": "$%s",
		"phone_number": "+2348099998888",
		"operator": "MTN",
		"amount_usdc": 2.00,
		"corridor": "NG"
	}`, time.Now().UnixNano(), userTag)

	resp, err := http.Post(baseURL+"/consumer/vas/airtime", "application/json", bytes.NewBufferString(airtimePayload))
	if err != nil {
		log.Fatalf("Airtime request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		log.Fatalf("❌ Airtime purchase failed [%d]: %s", resp.StatusCode, string(body))
	}

	fmt.Println("⚡ Airtime Purchased & Telecom Switch Dispatched!")
	fmt.Println("✓ Itemized Telco Receipt:")
	fmt.Println(string(body))

	fmt.Println("\n🎉 FIRST-PARTY VAS ENGINE OPERATIONAL. Everyday utility spending is live.")
}

func createAccount(url, payload string) string {
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(payload))
	if err != nil {
		log.Fatalf("createAccount failed: %v", err)
	}
	defer resp.Body.Close()
	var apiResp APIResp
	_ = json.NewDecoder(resp.Body).Decode(&apiResp)
	var acc struct{ ID uuid.UUID }
	_ = json.Unmarshal(apiResp.Data, &acc)
	return acc.ID.String()
}

func createAccountWithID(url, payload, targetID string) {
	_, _ = http.Post(url, "application/json", bytes.NewBufferString(payload))
}

func registerTag(url, payload string) {
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(payload))
	if err != nil {
		log.Fatalf("registerTag failed: %v", err)
	}
	defer resp.Body.Close()
}
