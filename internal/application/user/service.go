// Package user (application layer) holds the user USE CASES. It orchestrates the
// domain entity + repository; it knows nothing about HTTP, GORM, or Postgres.
//
// Note: this package and the domain package are both named "user" but live at
// different import paths. We alias the domain import as `domain` to keep them
// distinct and readable.
package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"chakam/internal/domain/user"
	"chakam/internal/pkg/validator"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

var ErrInvalidPagination = errors.New("invalid pagination")

type UsersPage struct {
	Users    []*user.User
	Total    int64
	Page     int
	PageSize int
}

type Service struct {
	repo user.Repository
}

func NewService(repo user.Repository) *Service {
	return &Service{repo: repo}
}

// CreateUser returns the user with this phone number, creating one if absent.
//
// phone must already be in E.164 form ("+2348031234567"). Callers normalise at
// their own boundary because only they can interpret the raw value: a WhatsApp
// wa_id carries a country code but no "+", while a number typed into chat is
// usually local and needs the sender's region to resolve. Rejecting anything
// else here is what keeps one row per human — the same person arriving by the
// webhook and by a counterparty's message must land on the same record.
func (s *Service) CreateUser(ctx context.Context, phone string, name *string) (*user.User, error) {
	// Passing an empty region means only a "+"-prefixed number can parse, so a
	// non-normalised value fails loudly here instead of creating a duplicate.
	e164, err := validator.ToE164(phone, "")
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	existing, err := s.repo.GetByPhone(ctx, e164)
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return nil, fmt.Errorf("create user: lookup by phone: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	u := &user.User{
		PublicID:    uuid.NewString(),
		PhoneNumber: e164,
		Name:        name,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (s *Service) GetUserById(ctx context.Context, userId string) (*user.User, error) {
	existing, err := s.repo.GetUserById(ctx, userId)

	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	return nil, user.ErrNotFound

}

func (s *Service) GetAllUsers(ctx context.Context, page, pageSize int) (*UsersPage, error) {
	if page < 1 || pageSize < 1 || pageSize > MaxPageSize {
		return nil, ErrInvalidPagination
	}

	maxInt := int(^uint(0) >> 1)
	if page > maxInt/pageSize+1 {
		return nil, ErrInvalidPagination
	}

	// Both values are validated above, so this offset is non-negative and
	// cannot overflow an int.
	offset := (page - 1) * pageSize
	users, total, err := s.repo.GetAllUsers(ctx, offset, pageSize)
	if err != nil {
		return nil, fmt.Errorf("get all users: %w", err)
	}
	return &UsersPage{
		Users:    users,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
