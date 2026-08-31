package session

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("session not found")

type Repository interface {
	Create(ctx context.Context, s *Session) error
	GetByUserID(ctx context.Context, userID string) (*Session, error)
	Update(ctx context.Context, s *Session) error
	Delete(ctx context.Context, id string) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}
