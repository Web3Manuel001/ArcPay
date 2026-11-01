package jev

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type SplitIntent struct {
	TotalAmountUSD float64  `json:"total_amount_usd"`
	Participants   []string `json:"participants"`
	Narration      string   `json:"narration"`
	Confidence     float64  `json:"confidence"`
	RiskScore      float64  `json:"risk_score"`
}

type Engine struct {
	isReady bool
}

func NewEngine() *Engine {
	return &Engine{isReady: true}
}

var (
	amountRegex = regexp.MustCompile(`(?i)(?:\$|usd\s*|₦|ngn\s*)?(\d+(?:\.\d{1,2})?)`)
	tagRegex    = regexp.MustCompile(`[@$]([a-zA-Z0-9_]{3,20})`)
)

func (e *Engine) ParseSplitIntent(ctx context.Context, input string) (*SplitIntent, error) {
	cleanInput := strings.TrimSpace(input)
	if cleanInput == "" {
		return nil, errors.New("empty intent input")
	}

	amountMatches := amountRegex.FindStringSubmatch(cleanInput)
	if len(amountMatches) < 2 {
		return nil, errors.New("could not identify a valid split amount")
	}
	totalUSD, err := strconv.ParseFloat(amountMatches[1], 64)
	if err != nil || totalUSD <= 0 {
		return nil, fmt.Errorf("invalid parsed amount: %v", err)
	}

	if strings.Contains(cleanInput, "₦") || strings.Contains(strings.ToLower(cleanInput), "ngn") {
		totalUSD = totalUSD / 1650.0
	}

	tagMatches := tagRegex.FindAllStringSubmatch(cleanInput, -1)
	var participants []string
	for _, m := range tagMatches {
		if len(m) > 1 {
			participants = append(participants, m[1])
		}
	}

	if len(participants) == 0 {
		return nil, errors.New("no participants specified (tag at least one friend with @ or $)")
	}

	riskScore := 0.05
	if totalUSD > 2000.0 {
		riskScore = 0.45
	}

	return &SplitIntent{
		TotalAmountUSD: totalUSD,
		Participants:   participants,
		Narration:      "Group Expense Settlement",
		Confidence:     0.98,
		RiskScore:      riskScore,
	}, nil
}
