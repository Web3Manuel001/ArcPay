package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrInvalidTagFormat = errors.New("invalid tag format: must be 3-20 characters alphanumeric (e.g. $chidi)")
	tagRegex            = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)
	phoneSalt           = "arcpay_salt_live_v1"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func CleanTag(rawTag string) string {
	tag := strings.TrimSpace(strings.ToLower(rawTag))
	tag = strings.TrimPrefix(tag, "$")
	tag = strings.TrimPrefix(tag, "@")
	return tag
}

func HashPhone(phone string) string {
	clean := strings.TrimSpace(phone)
	hash := sha256.Sum256([]byte(clean + phoneSalt))
	return hex.EncodeToString(hash[:])
}

func (s *Service) RegisterTag(ctx context.Context, req RegisterTagRequest) (*Identity, error) {
	tag := CleanTag(req.ArcTag)
	if !tagRegex.MatchString(tag) {
		return nil, ErrInvalidTagFormat
	}

	id := &Identity{
		ID:         uuid.New(),
		ArcTag:     tag,
		PhoneHash:  HashPhone(req.PhoneNumber),
		HolderID:   req.HolderID,
		AccountID:  req.AccountID,
		Corridor:   req.Corridor,
		EVMAddress: req.EVMAddress,
	}

	if err := s.repo.CreateIdentity(ctx, id); err != nil {
		return nil, fmt.Errorf("failed to register tag (might already be claimed): %w", err)
	}

	return id, nil
}

func (s *Service) ResolveTag(ctx context.Context, rawTag string) (*Identity, error) {
	tag := CleanTag(rawTag)
	return s.repo.GetByTag(ctx, tag)
}
