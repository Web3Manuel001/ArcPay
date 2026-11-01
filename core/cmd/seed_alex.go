package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Resp struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

func main() {
	baseURL := "https://arcpay-core.onrender.com/api/v1"

	fmt.Println("🌱 Seeding $alex on Render Backend...")

	// 1. Create Account for Alex
	holderID := uuid.New()
	accPayload := fmt.Sprintf(`{"holder_id":"%s","corridor":"NG","currency":"USDC","account_type":"ASSET"}`, holderID)
	res, err := http.Post(baseURL+"/accounts", "application/json", bytes.NewBufferString(accPayload))
	if err != nil {
		log.Fatalf("Failed to call render: %v", err)
	}
	defer res.Body.Close()

	var r Resp
	_ = json.NewDecoder(res.Body).Decode(&r)
	var acc struct{ ID uuid.UUID }
	_ = json.Unmarshal(r.Data, &acc)

	// 2. Claim $alex ArcTag
	regPayload := fmt.Sprintf(`{
		"arc_tag": "$alex",
		"phone_number": "+2348000000000",
		"holder_id": "%s",
		"account_id": "%s",
		"corridor": "NG",
		"evm_address": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	}`, holderID, acc.ID)

	res2, err := http.Post(baseURL+"/consumer/identity/register", "application/json", bytes.NewBufferString(regPayload))
	if err != nil {
		log.Fatalf("Tag registration failed: %v", err)
	}
	defer res2.Body.Close()

	fmt.Printf("✓ Seeded $alex successfully! Linked to account: %s\n", acc.ID)
}
