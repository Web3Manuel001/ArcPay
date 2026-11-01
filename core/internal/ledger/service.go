package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrUnbalancedTransaction = errors.New("unbalanced transaction: total debits must equal total credits")
	ErrInsufficientBalance   = errors.New("insufficient balance for debit transaction")
	ErrZeroPostings          = errors.New("transaction must contain at least two postings")
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateAccount(ctx context.Context, holderID uuid.UUID, corridor, currency string, accType AccountType) (*Account, error) {
	acc := &Account{
		ID:          uuid.New(),
		HolderID:    holderID,
		Corridor:    corridor,
		Currency:    currency,
		AccountType: accType,
		IsActive:    true,
	}

	if err := s.repo.CreateAccount(ctx, acc); err != nil {
		return nil, fmt.Errorf("service failed to create account: %w", err)
	}
	return acc, nil
}

func (s *Service) GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	return s.repo.GetBalance(ctx, accountID)
}

func (s *Service) GetHistory(ctx context.Context, accountID uuid.UUID, limit int) ([]HistoryItem, error) {
	return s.repo.GetHistory(ctx, accountID, limit)
}

func (s *Service) ProcessTransaction(ctx context.Context, payload TransactionPayload) (*JournalEntry, error) {
	if len(payload.Postings) < 2 {
		return nil, ErrZeroPostings
	}

	var totalDebits, totalCredits int64

	for _, p := range payload.Postings {
		if p.AmountUnits <= 0 {
			return nil, fmt.Errorf("invalid amount: %d, must be greater than zero", p.AmountUnits)
		}

		if p.Direction == Debit {
			totalDebits += p.AmountUnits
		} else if p.Direction == Credit {
			totalCredits += p.AmountUnits
		} else {
			return nil, fmt.Errorf("invalid direction: %s", p.Direction)
		}
	}

	if totalDebits != totalCredits {
		return nil, fmt.Errorf("%w: debits=%d, credits=%d", ErrUnbalancedTransaction, totalDebits, totalCredits)
	}

	return s.repo.RecordTransaction(ctx, payload)
}
