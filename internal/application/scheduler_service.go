package application

import (
	"context"
	"log/slog"

	"certschedule/internal/domain"
)

// SchedulerService scans for domains due for renewal and triggers them.
// It is invoked periodically by the scheduler adapter (internal/adapters/scheduler).
type SchedulerService struct {
	domains     domain.DomainRepository
	certificate *CertificateService
	clock       domain.Clock
	logger      *slog.Logger
}

func NewSchedulerService(domains domain.DomainRepository, certificate *CertificateService, clock domain.Clock, logger *slog.Logger) *SchedulerService {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &SchedulerService{domains: domains, certificate: certificate, clock: clock, logger: logger}
}

// Tick finds every auto-renew domain whose certificate is due (or missing)
// and issues/renews it, logging but not stopping on individual failures.
func (s *SchedulerService) Tick(ctx context.Context) error {
	due, err := s.domains.ListDueForRenewal(ctx, s.clock.Now())
	if err != nil {
		return err
	}

	s.logger.Info("scheduler tick", "due_count", len(due))
	for _, d := range due {
		if _, err := s.certificate.EnsureCertificate(ctx, d.ID, domain.TriggerScheduled); err != nil {
			s.logger.Error("scheduled renewal failed", "domain", d.Name, "error", err)
			continue
		}
		s.logger.Info("scheduled renewal succeeded", "domain", d.Name)
	}
	return nil
}
