package application

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"time"

	"certschedule/internal/domain"
)

// CertificateService orchestrates issuing/renewing certificates via the
// CertIssuer port, persisting history, and syncing results into Kubernetes.
type CertificateService struct {
	domains      domain.DomainRepository
	certificates domain.CertificateRepository
	jobs         domain.JobRepository
	issuer       domain.CertIssuer
	secrets      domain.SecretUpdater
	encryptor    domain.Encryptor
	clock        domain.Clock
	logger       *slog.Logger
}

func NewCertificateService(
	domains domain.DomainRepository,
	certificates domain.CertificateRepository,
	jobs domain.JobRepository,
	issuer domain.CertIssuer,
	secrets domain.SecretUpdater,
	encryptor domain.Encryptor,
	clock domain.Clock,
	logger *slog.Logger,
) *CertificateService {
	if clock == nil {
		clock = domain.SystemClock{}
	}
	return &CertificateService{
		domains: domains, certificates: certificates, jobs: jobs,
		issuer: issuer, secrets: secrets, encryptor: encryptor, clock: clock, logger: logger,
	}
}

// Issue runs a first-time certbot certonly for the domain.
func (s *CertificateService) Issue(ctx context.Context, domainID string, trigger domain.TriggerType) (*domain.RenewalJob, error) {
	return s.run(ctx, domainID, trigger, func(d *domain.Domain, decryptedCred []byte) (domain.CertResult, error) {
		return s.issuer.Issue(ctx, domain.IssueRequest{
			DomainName:          d.Name,
			ChallengeType:       d.ChallengeType,
			DNSProvider:         d.DNSProvider,
			DecryptedCredential: decryptedCred,
		})
	})
}

// Renew runs certbot renew for a domain that already has a certificate.
func (s *CertificateService) Renew(ctx context.Context, domainID string, trigger domain.TriggerType) (*domain.RenewalJob, error) {
	return s.run(ctx, domainID, trigger, func(d *domain.Domain, decryptedCred []byte) (domain.CertResult, error) {
		return s.issuer.Renew(ctx, domain.RenewRequest{
			DomainName:          d.Name,
			ChallengeType:       d.ChallengeType,
			DNSProvider:         d.DNSProvider,
			DecryptedCredential: decryptedCred,
		})
	})
}

// CertificateHistory returns all issued certificate snapshots for a domain,
// newest first.
func (s *CertificateService) CertificateHistory(ctx context.Context, domainID string) ([]*domain.Certificate, error) {
	return s.certificates.HistoryForDomain(ctx, domainID)
}

// JobHistory returns all renewal job records for a domain, newest first.
func (s *CertificateService) JobHistory(ctx context.Context, domainID string) ([]*domain.RenewalJob, error) {
	return s.jobs.ListForDomain(ctx, domainID)
}

// CertificateDetail is a Certificate enriched with fields parsed out of its
// X.509 payload, for display in a "view certificate" UI.
type CertificateDetail struct {
	ID           string
	DomainID     string
	IssuedAt     time.Time
	ExpiresAt    time.Time
	Status       domain.CertificateStatus
	CreatedAt    time.Time
	SerialNumber string
	Issuer       string
	SubjectCN    string
	DNSNames     []string
}

// GetCertificateDetail returns a single certificate's metadata plus fields
// parsed from its X.509 payload, scoped to the given domain.
func (s *CertificateService) GetCertificateDetail(ctx context.Context, domainID, certID string) (*CertificateDetail, error) {
	c, err := s.getOwnedCertificate(ctx, domainID, certID)
	if err != nil {
		return nil, err
	}
	detail := &CertificateDetail{
		ID: c.ID, DomainID: c.DomainID, IssuedAt: c.IssuedAt, ExpiresAt: c.ExpiresAt,
		Status: c.Status, CreatedAt: c.CreatedAt,
	}
	if block, _ := pem.Decode(c.CertPEM); block != nil {
		if x509Cert, err := x509.ParseCertificate(block.Bytes); err == nil {
			detail.SerialNumber = x509Cert.SerialNumber.String()
			detail.Issuer = x509Cert.Issuer.CommonName
			detail.SubjectCN = x509Cert.Subject.CommonName
			detail.DNSNames = x509Cert.DNSNames
		}
	}
	return detail, nil
}

// DownloadFullChain returns the public cert+chain PEM (safe to hand out) for
// a certificate, scoped to the given domain.
func (s *CertificateService) DownloadFullChain(ctx context.Context, domainID, certID string) ([]byte, error) {
	c, err := s.getOwnedCertificate(ctx, domainID, certID)
	if err != nil {
		return nil, err
	}
	fullChain := make([]byte, 0, len(c.CertPEM)+len(c.ChainPEM))
	fullChain = append(fullChain, c.CertPEM...)
	fullChain = append(fullChain, c.ChainPEM...)
	return fullChain, nil
}

// DownloadPrivateKey decrypts and returns the private key PEM for a
// certificate, scoped to the given domain. Callers should treat this as a
// sensitive, explicit opt-in action (like credential export).
func (s *CertificateService) DownloadPrivateKey(ctx context.Context, domainID, certID string) ([]byte, error) {
	c, err := s.getOwnedCertificate(ctx, domainID, certID)
	if err != nil {
		return nil, err
	}
	keyPEM, err := s.encryptor.Decrypt(c.EncryptedKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("decrypt private key: %w", err)
	}
	return keyPEM, nil
}

func (s *CertificateService) getOwnedCertificate(ctx context.Context, domainID, certID string) (*domain.Certificate, error) {
	c, err := s.certificates.Get(ctx, certID)
	if err != nil {
		return nil, err
	}
	if c.DomainID != domainID {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

// EnsureCertificate is used by the scheduler: it issues a fresh cert if the
// domain has none yet, otherwise renews the existing one.
func (s *CertificateService) EnsureCertificate(ctx context.Context, domainID string, trigger domain.TriggerType) (*domain.RenewalJob, error) {
	_, err := s.certificates.LatestForDomain(ctx, domainID)
	switch {
	case err == nil:
		return s.Renew(ctx, domainID, trigger)
	case err == domain.ErrNotFound:
		return s.Issue(ctx, domainID, trigger)
	default:
		return nil, err
	}
}

type issueFunc func(d *domain.Domain, decryptedCred []byte) (domain.CertResult, error)

func (s *CertificateService) run(ctx context.Context, domainID string, trigger domain.TriggerType, fn issueFunc) (*domain.RenewalJob, error) {
	d, err := s.domains.Get(ctx, domainID)
	if err != nil {
		return nil, err
	}

	job := &domain.RenewalJob{
		DomainID:  domainID,
		Trigger:   trigger,
		Status:    domain.JobStatusRunning,
		StartedAt: s.clock.Now(),
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}

	var decryptedCred []byte
	if d.ChallengeType == domain.ChallengeDNS01 && len(d.EncryptedCredential) > 0 {
		decryptedCred, err = s.encryptor.Decrypt(d.EncryptedCredential)
		if err != nil {
			return s.fail(ctx, d, job, fmt.Errorf("decrypt dns credential: %w", err))
		}
	}

	result, err := fn(d, decryptedCred)
	if err != nil {
		return s.fail(ctx, d, job, err)
	}

	// Dry-run mode: certbot validated the challenge but persisted nothing.
	if len(result.CertPEM) == 0 {
		return s.succeed(ctx, d, job, "dry-run validated successfully; no certificate persisted")
	}

	encryptedKey, err := s.encryptor.Encrypt(result.KeyPEM)
	if err != nil {
		return s.fail(ctx, d, job, fmt.Errorf("encrypt private key: %w", err))
	}

	cert := &domain.Certificate{
		DomainID:        d.ID,
		IssuedAt:        s.clock.Now(),
		ExpiresAt:       result.ExpiresAt,
		CertPEM:         result.CertPEM,
		ChainPEM:        result.ChainPEM,
		EncryptedKeyPEM: encryptedKey,
		Status:          domain.CertificateStatusActive,
	}
	if err := s.certificates.Create(ctx, cert); err != nil {
		return s.fail(ctx, d, job, fmt.Errorf("save certificate: %w", err))
	}

	// tls.crt must be the full chain (leaf + intermediates), not just the
	// leaf cert, or clients that don't already trust the intermediate CA
	// will fail TLS verification. This mirrors how certbot itself builds
	// fullchain.pem from cert.pem + chain.pem.
	fullChainPEM := make([]byte, 0, len(result.CertPEM)+len(result.ChainPEM))
	fullChainPEM = append(fullChainPEM, result.CertPEM...)
	fullChainPEM = append(fullChainPEM, result.ChainPEM...)

	if err := s.secrets.UpsertTLSSecret(ctx, d.K8sNamespace, d.K8sSecretName, fullChainPEM, result.KeyPEM); err != nil {
		return s.fail(ctx, d, job, fmt.Errorf("update k8s secret: %w", err))
	}

	return s.succeed(ctx, d, job, fmt.Sprintf("certificate valid until %s", result.ExpiresAt.Format("2006-01-02")))
}

func (s *CertificateService) succeed(ctx context.Context, d *domain.Domain, job *domain.RenewalJob, message string) (*domain.RenewalJob, error) {
	now := s.clock.Now()
	job.Status = domain.JobStatusSuccess
	job.Message = message
	job.FinishedAt = &now
	if err := s.jobs.Update(ctx, job); err != nil {
		s.logger.Error("update job after success", "error", err)
	}

	d.Status = domain.DomainStatusActive
	d.LastError = ""
	if err := s.domains.Update(ctx, d); err != nil {
		s.logger.Error("update domain status after success", "error", err)
	}
	return job, nil
}

func (s *CertificateService) fail(ctx context.Context, d *domain.Domain, job *domain.RenewalJob, cause error) (*domain.RenewalJob, error) {
	now := s.clock.Now()
	job.Status = domain.JobStatusFailed
	job.Message = cause.Error()
	job.FinishedAt = &now
	if err := s.jobs.Update(ctx, job); err != nil {
		s.logger.Error("update job after failure", "error", err)
	}

	d.Status = domain.DomainStatusFailed
	d.LastError = cause.Error()
	if err := s.domains.Update(ctx, d); err != nil {
		s.logger.Error("update domain status after failure", "error", err)
	}
	return job, cause
}
