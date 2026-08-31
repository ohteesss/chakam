package user

import (
	"context"
	"errors"
)


var ErrNotFound = errors.New("user not found")

type Repository interface {
	Create(ctx context.Context, u *User) error
	GetByPhone(ctx context.Context, phone string) (*User, error)
	GetUserById(ctx context.Context, userId string) (*User, error)
	GetAllUsers(ctx context.Context, offset, limit int) ([]*User, int64, error)
}
