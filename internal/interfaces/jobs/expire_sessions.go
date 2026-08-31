package jobs

import (
	"context"
	"time"

	domainsession "chakam/internal/domain/session"

	"go.uber.org/zap"
)

// ExpireSessions deletes (or marks expired) conversation sessions whose
// expires_at has passed. This is what lets us keep conversation history
// ephemeral — the "No storage of full message content" guarantee — by purging
// stale chats automatically.
//
// It carries its dependencies as fields, like every other component.
type ExpireSessions struct {
	log      *zap.Logger
	repo     domainsession.Repository
	interval time.Duration
}

func NewExpireSessions(log *zap.Logger, repo domainsession.Repository, interval time.Duration) *ExpireSessions {
	return &ExpireSessions{log: log, repo: repo, interval: interval}
}

func (j *ExpireSessions) Name() string            { return "expire_sessions" }
func (j *ExpireSessions) Interval() time.Duration { return j.interval }

func (j *ExpireSessions) Run(ctx context.Context) error {
	deleted, err := j.repo.DeleteExpired(ctx, time.Now())
	if err != nil {
		return err
	}
	j.log.Info("expired sessions purged", zap.Int64("count", deleted))
	return nil
}
