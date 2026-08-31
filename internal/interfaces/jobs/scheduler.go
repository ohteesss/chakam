// Package jobs holds background jobs and the scheduler that runs them. Jobs are
// a time-driven inbound adapter — the worker's equivalent of HTTP handlers.
package jobs

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Job is one unit of background work. Each job is a struct that carries its own
// dependencies (repos, clients) as fields and implements this interface — the
// same pattern as the HTTP handlers and repositories.
type Job interface {
	Name() string                  // for logging
	Interval() time.Duration       // how often to run
	Run(ctx context.Context) error // the work; ctx is cancelled on shutdown
}

// Scheduler runs registered jobs, each on its own interval, until its context
// is cancelled. It's deliberately tiny: a ticker per job is all the "cron" an
// MVP needs. (For calendar schedules like "every day at 02:00", swap in
// robfig/cron later — the Job interface wouldn't change.)
type Scheduler struct {
	log  *zap.Logger
	jobs []Job
}

func NewScheduler(log *zap.Logger) *Scheduler {
	return &Scheduler{log: log}
}

// Register adds a job. Call before Run.
func (s *Scheduler) Register(j Job) {
	s.jobs = append(s.jobs, j)
}

// Run starts every job in its own goroutine and BLOCKS until ctx is cancelled.
// On cancellation it waits for any in-flight job runs to finish (via WaitGroup)
// before returning — that's the graceful part: we don't kill a job mid-write.
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, j := range s.jobs {
		wg.Add(1)
		// j is passed as an argument so each goroutine captures its OWN job, not
		// a shared loop variable. (A classic Go bug pre-1.22: closing over the
		// loop variable directly meant every goroutine saw the last job.)
		go func(j Job) {
			defer wg.Done()
			s.runLoop(ctx, j)
		}(j)
	}
	wg.Wait()
}

// runLoop runs one job: once immediately at startup, then on every tick, until
// the context is cancelled.
func (s *Scheduler) runLoop(ctx context.Context, j Job) {
	ticker := time.NewTicker(j.Interval())
	defer ticker.Stop() // a leaked Ticker keeps a goroutine alive forever

	s.log.Info("job registered", zap.String("job", j.Name()), zap.Duration("interval", j.Interval()))

	s.execute(ctx, j) // run once now, so we don't wait a full interval at boot

	for {
		// select blocks until ONE of these channels is ready. This is the
		// canonical Go pattern for "do work on a tick, but bail out the instant
		// we're told to stop."
		select {
		case <-ctx.Done():
			s.log.Info("job stopping", zap.String("job", j.Name()))
			return
		case <-ticker.C:
			s.execute(ctx, j)
		}
	}
}

// execute runs the job once, with timing, error logging, and panic recovery.
// The recover() is critical: a panic in a background goroutine crashes the
// WHOLE process, so each run is contained here.
func (s *Scheduler) execute(ctx context.Context, j Job) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("job panicked", zap.String("job", j.Name()), zap.Any("panic", r))
		}
	}()

	start := time.Now()
	if err := j.Run(ctx); err != nil {
		s.log.Error("job failed", zap.String("job", j.Name()), zap.Error(err))
		return
	}
	s.log.Info("job completed", zap.String("job", j.Name()), zap.Duration("took", time.Since(start)))
}
