package agreement

import (
	"chakam/internal/domain/agreement"
	"context"

	"github.com/google/uuid"
)

type Service struct {
	repo agreement.Repository
}

func NewService(repo agreement.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateAgreement(ctx context.Context, a *agreement.Agreement) (*agreement.Agreement, error) {
	a.ID = uuid.NewString()
	err := s.repo.Create(ctx, a)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) ConfirmAgreement(ctx context.Context, agreementID string) (*agreement.Agreement, error) {
	a, err := s.repo.Get(ctx, agreementID)
	if err != nil {
		return nil, err
	}
	return a, s.repo.Update(ctx, agreementID, StatusConfirmed)
}

func (s *Service) RejectAgreement(ctx context.Context, agreementID string) (*agreement.Agreement, error) {
	a, err := s.repo.Get(ctx, agreementID)
	if err != nil {
		return nil, err
	}
	return a, s.repo.Update(ctx, agreementID, StatusRejected)
}

func (s *Service) CancelAgreement(ctx context.Context, agreementID string) error {
	return s.repo.Update(ctx, agreementID, StatusCancelled)
}

func (s *Service) GetAgreementById(ctx context.Context, id string) (*agreement.Agreement, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) GetAgreements(ctx context.Context, filter string, userID string) ([]*agreement.Agreement, error) {
	return s.repo.GetAll(ctx, filter, userID)
}

func (s *Service) GetPendingForCounterparty(ctx context.Context, userID string) ([]*agreement.Agreement, error) {
	return s.repo.GetPendingForCounterparty(ctx, userID)
}
