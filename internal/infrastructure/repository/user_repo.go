// Package repository holds the GORM/Postgres implementations of the domain
// repository interfaces. This is the "how" — the only place that knows SQL.
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"chakam/internal/domain/user"
)

var _ user.Repository = (*GormUserRepository)(nil)

type GormUserRepository struct {
	db *gorm.DB
}

func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) Create(ctx context.Context, u *user.User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *GormUserRepository) GetByPhone(ctx context.Context, phone string) (*user.User, error) {
	var u user.User

	err := r.db.WithContext(ctx).
		Where("phone_number = ?", phone).
		First(&u).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, user.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *GormUserRepository) GetUserById(ctx context.Context, userId string) (*user.User, error) {
	var u user.User

	err := r.db.WithContext(ctx).
		Where("public_id = ?", userId).
		First(&u).
		Error

	if err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, user.ErrNotFound
		}
		return nil, err

	}

	return &u, nil
}

func (r *GormUserRepository) GetAllUsers(ctx context.Context, offset, limit int) ([]*user.User, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&user.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	users := make([]*user.User, 0)
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&users).
		Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
