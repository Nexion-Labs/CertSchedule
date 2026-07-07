// Package scheduler wires the application SchedulerService to a recurring
// in-process cron trigger (robfig/cron), so certificate renewal happens
// automatically without depending on a k8s CronJob.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

// Ticker is implemented by application.SchedulerService.
type Ticker interface {
	Tick(ctx context.Context) error
}

// Cron runs Ticker.Tick on a fixed interval using robfig/cron's "@every" spec.
type Cron struct {
	c        *cron.Cron
	ticker   Ticker
	interval time.Duration
	logger   *slog.Logger
}

func New(ticker Ticker, interval time.Duration, logger *slog.Logger) *Cron {
	return &Cron{c: cron.New(), ticker: ticker, interval: interval, logger: logger}
}

// Start schedules the recurring tick and returns immediately; the cron
// scheduler runs in its own goroutine until Stop is called.
func (s *Cron) Start(ctx context.Context) error {
	spec := fmt.Sprintf("@every %s", s.interval)
	_, err := s.c.AddFunc(spec, func() {
		if err := s.ticker.Tick(ctx); err != nil {
			s.logger.Error("scheduler tick failed", "error", err)
		}
	})
	if err != nil {
		return fmt.Errorf("schedule tick: %w", err)
	}
	s.c.Start()
	s.logger.Info("scheduler started", "interval", s.interval)
	return nil
}

// Stop blocks until the currently running job (if any) completes.
func (s *Cron) Stop() {
	<-s.c.Stop().Done()
}
