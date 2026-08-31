package repository

import (
	"chakam/internal/domain/session"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type GormSessionRepository struct {
	db *gorm.DB
}

func NewGormSessionRepository(db *gorm.DB) *GormSessionRepository {
	return &GormSessionRepository{db: db}
}

func (r *GormSessionRepository) Create(ctx context.Context, s *session.Session) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *GormSessionRepository) GetByUserID(ctx context.Context, userID string) (*session.Session, error) {
	var s session.Session
	// Order by recency: .First() otherwise orders by primary key (the UUID),
	// which is unrelated to which session is current. If any duplicate rows
	// exist, this deterministically returns the newest.
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, session.ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *GormSessionRepository) Update(ctx context.Context, s *session.Session) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *GormSessionRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&session.Session{}, id).Error
}

func (r *GormSessionRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("expires_at <= ?", before).Delete(&session.Session{})
	return result.RowsAffected, result.Error
}
