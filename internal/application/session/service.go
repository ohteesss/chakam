package session

import (
	"chakam/internal/domain/session"
	"chakam/internal/pkg/logger"
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	repo session.Repository
}

func NewService(repo session.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateSession(ctx context.Context, userID string) (*session.Session, error) {
	log := logger.New()
	ses, err := s.repo.GetByUserID(ctx, userID)
	if err != nil && err != session.ErrNotFound {
		return nil, err
	}

	// No session yet, or the newest one has expired → start a fresh row. Expired
	// rows are left in place for you to delete on your own schedule; GetByUserID
	// orders by created_at DESC, so reads always resolve to this newest session.
	if err == session.ErrNotFound || ses.IsExpired() {
		ses = &session.Session{
			ID:        uuid.NewString(),
			UserID:    userID,
			ExpiresAt: time.Now().Add(session.SessionTTL),
		}
		if err := s.repo.Create(ctx, ses); err != nil {
			return nil, err
		}
	}

	log.Info("session created", zap.Any("session", ses))
	return ses, nil
}

func (s *Service) GetSessionByUserID(ctx context.Context, userID string) (*session.Session, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *Service) AddMessageToSession(ctx context.Context, userID string, message session.Message) error {

	session, err := s.CreateSession(ctx, userID)
	if err != nil {
		return err
	}
	session.ConversationHistory = append(session.ConversationHistory, message)

	return s.repo.Update(ctx, session)
}

func (s *Service) GetConversationHistory(ctx context.Context, userID string) (session.ConversationHistory, error) {
	session, err := s.CreateSession(ctx, userID)
	if err != nil {
		return nil, err
	}
	return session.ConversationHistory, nil
}
