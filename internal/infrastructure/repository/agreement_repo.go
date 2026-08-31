package repository

import (
	"chakam/internal/domain/agreement"
	"context"

	"gorm.io/gorm"

	"errors"
)

type GormAgreementRepository struct {
	db *gorm.DB
}

func NewGormAgreementRepository(db *gorm.DB) *GormAgreementRepository {
	return &GormAgreementRepository{db: db}
}

func (r *GormAgreementRepository) Create(ctx context.Context, a *agreement.Agreement) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *GormAgreementRepository) Update(ctx context.Context, agreementID string, status string) error {
	return r.db.WithContext(ctx).Model(&agreement.Agreement{}).Where("id = ?", agreementID).Update("status", status).Error
}

func (r *GormAgreementRepository) Get(ctx context.Context, id string) (*agreement.Agreement, error) {
	var a agreement.Agreement
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, agreement.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *GormAgreementRepository) GetAll(ctx context.Context, filter string, userID string) ([]*agreement.Agreement, error) {
	var agreements []*agreement.Agreement
	switch filter {
	case "all":
		err := r.db.WithContext(ctx).Where("initiator_id = ? OR counterparty_id = ?", userID, userID).Find(&agreements).Error
		if err != nil {
			return nil, err
		}
	case "pending":
		err := r.db.WithContext(ctx).Where("status = ? AND (initiator_id = ? OR counterparty_id = ?)", "PENDING", userID, userID).Find(&agreements).Error
		if err != nil {
			return nil, err
		}
	case "confirmed":
		err := r.db.WithContext(ctx).Where("status = ? AND (initiator_id = ? OR counterparty_id = ?)", "CONFIRMED", userID, userID).Find(&agreements).Error
		if err != nil {
			return nil, err
		}
	case "created_by_me":
		err := r.db.WithContext(ctx).Where("initiator_id = ?", userID).Find(&agreements).Error
		if err != nil {
			return nil, err
		}
	}
	return agreements, nil
}

func (r *GormAgreementRepository) GetPendingForCounterparty(ctx context.Context, userID string) ([]*agreement.Agreement, error) {
	var agreements []*agreement.Agreement
	err := r.db.WithContext(ctx).Where("status = ? AND counterparty_id = ?", "PENDING", userID).Find(&agreements).Error
	if err != nil {
		return nil, err
	}
	return agreements, nil
}
