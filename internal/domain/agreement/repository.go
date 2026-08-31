package agreement

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("agreement not found")

type Repository interface {
	Create(ctx context.Context, agreement *Agreement) error
	Update(ctx context.Context, agreementID string, status string) error
	Get(ctx context.Context, id string) (*Agreement, error)
	GetAll(ctx context.Context, filter string, userID string) ([]*Agreement, error)
	GetPendingForCounterparty(ctx context.Context, userID string) ([]*Agreement, error)
}
