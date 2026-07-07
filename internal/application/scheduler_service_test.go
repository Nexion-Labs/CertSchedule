package application

import (
	"context"
	"testing"
	"time"

	"certschedule/internal/domain"
)

func TestSchedulerService_Tick_RenewsDueDomains(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	issuer := &fakeCertIssuer{renewResult: domain.CertResult{
		CertPEM: []byte("c"), KeyPEM: []byte("k"), ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
	}}
	secrets := &fakeSecretUpdater{}

	due := seedDomain(t, domains, &domain.Domain{Name: "due.example.com", AutoRenew: true, ChallengeType: domain.ChallengeHTTP01})
	notDue := seedDomain(t, domains, &domain.Domain{Name: "not-due.example.com", AutoRenew: false, ChallengeType: domain.ChallengeHTTP01})
	_ = notDue

	if err := certs.Create(context.Background(), &domain.Certificate{DomainID: due.ID, ExpiresAt: time.Now().Add(24 * time.Hour)}); err != nil {
		t.Fatalf("seed cert: %v", err)
	}

	certSvc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	scheduler := NewSchedulerService(domains, certSvc, fakeClock{now: time.Now()}, testLogger())

	if err := scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issuer.renewCalls != 1 {
		t.Errorf("expected 1 renew call for the due+auto_renew domain, got %d", issuer.renewCalls)
	}
}

func TestSchedulerService_Tick_ContinuesAfterFailure(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	issuer := &fakeCertIssuer{issueErr: errTest("boom")}
	secrets := &fakeSecretUpdater{}

	seedDomain(t, domains, &domain.Domain{Name: "a.example.com", AutoRenew: true, ChallengeType: domain.ChallengeHTTP01})
	seedDomain(t, domains, &domain.Domain{Name: "b.example.com", AutoRenew: true, ChallengeType: domain.ChallengeHTTP01})

	certSvc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	scheduler := NewSchedulerService(domains, certSvc, fakeClock{now: time.Now()}, testLogger())

	if err := scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick should not return an error even if individual renewals fail: %v", err)
	}
	if issuer.issueCalls != 2 {
		t.Errorf("expected both domains attempted (issue since no cert exists), got %d", issuer.issueCalls)
	}
}
