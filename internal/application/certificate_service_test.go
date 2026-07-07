package application

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"testing"
	"time"

	"certschedule/internal/domain"
)

// generateTestCertPEM builds a minimal self-signed certificate for a given
// CN, for tests that need to exercise real X.509 parsing.
func generateTestCertPEM(t *testing.T, cn string) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		Issuer:       pkix.Name{CommonName: "Test CA"},
		DNSNames:     []string{cn},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(90 * 24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func seedDomain(t *testing.T, repo *fakeDomainRepo, d *domain.Domain) *domain.Domain {
	t.Helper()
	if err := repo.Create(context.Background(), d); err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	return d
}

func TestCertificateService_Issue_Success(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	issuer := &fakeCertIssuer{issueResult: domain.CertResult{
		CertPEM: []byte("cert"), ChainPEM: []byte("chain"), KeyPEM: []byte("key"),
		ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
	}}
	secrets := &fakeSecretUpdater{}

	d := seedDomain(t, domains, &domain.Domain{Name: "example.com", ChallengeType: domain.ChallengeHTTP01, K8sNamespace: "default", K8sSecretName: "tls"})

	svc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	job, err := svc.Issue(context.Background(), d.ID, domain.TriggerManual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Status != domain.JobStatusSuccess {
		t.Errorf("expected job success, got %s: %s", job.Status, job.Message)
	}
	if issuer.issueCalls != 1 {
		t.Errorf("expected 1 issue call, got %d", issuer.issueCalls)
	}
	if secrets.calls != 1 {
		t.Errorf("expected 1 secret update call, got %d", secrets.calls)
	}
	if string(secrets.lastCertPEM) != "certchain" {
		t.Errorf("expected tls.crt to be the full chain (cert+chain), got %q", secrets.lastCertPEM)
	}

	updated, _ := domains.Get(context.Background(), d.ID)
	if updated.Status != domain.DomainStatusActive {
		t.Errorf("expected domain active, got %s", updated.Status)
	}

	latest, err := certs.LatestForDomain(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("expected certificate to be saved: %v", err)
	}
	if string(latest.CertPEM) != "cert" {
		t.Errorf("expected cert pem to be saved")
	}
}

func TestCertificateService_Issue_Failure_MarksDomainFailed(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	issuer := &fakeCertIssuer{issueErr: errTest("acme challenge failed")}
	secrets := &fakeSecretUpdater{}

	d := seedDomain(t, domains, &domain.Domain{Name: "example.com", ChallengeType: domain.ChallengeHTTP01})

	svc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	job, err := svc.Issue(context.Background(), d.ID, domain.TriggerManual)
	if err == nil {
		t.Fatal("expected error")
	}
	if job.Status != domain.JobStatusFailed {
		t.Errorf("expected job failed, got %s", job.Status)
	}

	updated, _ := domains.Get(context.Background(), d.ID)
	if updated.Status != domain.DomainStatusFailed {
		t.Errorf("expected domain failed, got %s", updated.Status)
	}
	if updated.LastError == "" {
		t.Errorf("expected last_error to be set")
	}
}

func TestCertificateService_Issue_DryRun_SkipsPersistAndSecret(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	issuer := &fakeCertIssuer{issueResult: domain.CertResult{}} // empty = dry-run signal
	secrets := &fakeSecretUpdater{}

	d := seedDomain(t, domains, &domain.Domain{Name: "example.com", ChallengeType: domain.ChallengeHTTP01})

	svc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	job, err := svc.Issue(context.Background(), d.ID, domain.TriggerManual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Status != domain.JobStatusSuccess {
		t.Errorf("expected job success for dry run, got %s", job.Status)
	}
	if secrets.calls != 0 {
		t.Errorf("expected no secret update on dry run, got %d calls", secrets.calls)
	}
	if _, err := certs.LatestForDomain(context.Background(), d.ID); err != domain.ErrNotFound {
		t.Errorf("expected no certificate persisted on dry run")
	}
}

func TestCertificateService_EnsureCertificate_IssuesWhenNoCertExists(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	issuer := &fakeCertIssuer{issueResult: domain.CertResult{CertPEM: []byte("c"), KeyPEM: []byte("k"), ExpiresAt: time.Now().Add(time.Hour)}}
	secrets := &fakeSecretUpdater{}

	d := seedDomain(t, domains, &domain.Domain{Name: "example.com", ChallengeType: domain.ChallengeHTTP01, AutoRenew: true})

	svc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	if _, err := svc.EnsureCertificate(context.Background(), d.ID, domain.TriggerScheduled); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issuer.issueCalls != 1 || issuer.renewCalls != 0 {
		t.Errorf("expected issue to be called, not renew: issue=%d renew=%d", issuer.issueCalls, issuer.renewCalls)
	}
}

func TestCertificateService_EnsureCertificate_RenewsWhenCertExists(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	issuer := &fakeCertIssuer{renewResult: domain.CertResult{CertPEM: []byte("c"), KeyPEM: []byte("k"), ExpiresAt: time.Now().Add(time.Hour)}}
	secrets := &fakeSecretUpdater{}

	d := seedDomain(t, domains, &domain.Domain{Name: "example.com", ChallengeType: domain.ChallengeHTTP01, AutoRenew: true})
	if err := certs.Create(context.Background(), &domain.Certificate{DomainID: d.ID, ExpiresAt: time.Now().Add(24 * time.Hour)}); err != nil {
		t.Fatalf("seed cert: %v", err)
	}

	svc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	if _, err := svc.EnsureCertificate(context.Background(), d.ID, domain.TriggerScheduled); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issuer.renewCalls != 1 || issuer.issueCalls != 0 {
		t.Errorf("expected renew to be called, not issue: issue=%d renew=%d", issuer.issueCalls, issuer.renewCalls)
	}
}

func TestCertificateService_GetCertificateDetail_ParsesX509Fields(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	certPEM, keyPEM := generateTestCertPEM(t, "example.com")
	issuer := &fakeCertIssuer{issueResult: domain.CertResult{
		CertPEM: certPEM, KeyPEM: keyPEM, ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
	}}
	secrets := &fakeSecretUpdater{}

	d := seedDomain(t, domains, &domain.Domain{Name: "example.com", ChallengeType: domain.ChallengeHTTP01, K8sNamespace: "default", K8sSecretName: "tls"})

	svc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	if _, err := svc.Issue(context.Background(), d.ID, domain.TriggerManual); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	latest, err := certs.LatestForDomain(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("expected certificate to be saved: %v", err)
	}

	detail, err := svc.GetCertificateDetail(context.Background(), d.ID, latest.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.SubjectCN != "example.com" {
		t.Errorf("expected subject CN example.com, got %q", detail.SubjectCN)
	}
	if detail.SerialNumber == "" {
		t.Errorf("expected serial number to be parsed")
	}
}

func TestCertificateService_GetCertificateDetail_RejectsCrossDomainAccess(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	certPEM, keyPEM := generateTestCertPEM(t, "example.com")
	issuer := &fakeCertIssuer{issueResult: domain.CertResult{
		CertPEM: certPEM, KeyPEM: keyPEM, ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
	}}
	secrets := &fakeSecretUpdater{}

	d1 := seedDomain(t, domains, &domain.Domain{Name: "example.com", ChallengeType: domain.ChallengeHTTP01, K8sNamespace: "default", K8sSecretName: "tls"})
	d2 := seedDomain(t, domains, &domain.Domain{Name: "other.com", ChallengeType: domain.ChallengeHTTP01, K8sNamespace: "default", K8sSecretName: "tls2"})

	svc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	if _, err := svc.Issue(context.Background(), d1.ID, domain.TriggerManual); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	latest, err := certs.LatestForDomain(context.Background(), d1.ID)
	if err != nil {
		t.Fatalf("expected certificate to be saved: %v", err)
	}

	if _, err := svc.GetCertificateDetail(context.Background(), d2.ID, latest.ID); err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound for cross-domain access, got %v", err)
	}
}

func TestCertificateService_DownloadFullChain_ReturnsCertPlusChain(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	issuer := &fakeCertIssuer{issueResult: domain.CertResult{
		CertPEM: []byte("cert"), ChainPEM: []byte("chain"), KeyPEM: []byte("key"),
		ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
	}}
	secrets := &fakeSecretUpdater{}

	d := seedDomain(t, domains, &domain.Domain{Name: "example.com", ChallengeType: domain.ChallengeHTTP01, K8sNamespace: "default", K8sSecretName: "tls"})

	svc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	if _, err := svc.Issue(context.Background(), d.ID, domain.TriggerManual); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	latest, err := certs.LatestForDomain(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("expected certificate to be saved: %v", err)
	}

	fullChain, err := svc.DownloadFullChain(context.Background(), d.ID, latest.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(fullChain) != "certchain" {
		t.Errorf("expected fullchain to be cert+chain, got %q", fullChain)
	}

	keyPEM, err := svc.DownloadPrivateKey(context.Background(), d.ID, latest.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(keyPEM) != "key" {
		t.Errorf("expected decrypted private key %q, got %q", "key", keyPEM)
	}
}

func TestCertificateService_DownloadFullChain_ReturnsRenewedCertNotStaleOne(t *testing.T) {
	domains := newFakeDomainRepo()
	certs := newFakeCertificateRepo()
	jobs := newFakeJobRepo()
	issuer := &fakeCertIssuer{
		issueResult: domain.CertResult{
			CertPEM: []byte("cert-v1"), ChainPEM: []byte("chain-v1"), KeyPEM: []byte("key-v1"),
			ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
		},
		renewResult: domain.CertResult{
			CertPEM: []byte("cert-v2"), ChainPEM: []byte("chain-v2"), KeyPEM: []byte("key-v2"),
			ExpiresAt: time.Now().Add(180 * 24 * time.Hour),
		},
	}
	secrets := &fakeSecretUpdater{}

	d := seedDomain(t, domains, &domain.Domain{Name: "example.com", ChallengeType: domain.ChallengeHTTP01, K8sNamespace: "default", K8sSecretName: "tls"})

	svc := NewCertificateService(domains, certs, jobs, issuer, secrets, fakeEncryptor{}, nil, testLogger())
	if _, err := svc.Issue(context.Background(), d.ID, domain.TriggerManual); err != nil {
		t.Fatalf("unexpected error on issue: %v", err)
	}
	original, err := certs.LatestForDomain(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("expected certificate to be saved: %v", err)
	}

	if _, err := svc.Renew(context.Background(), d.ID, domain.TriggerManual); err != nil {
		t.Fatalf("unexpected error on renew: %v", err)
	}

	history, err := svc.CertificateHistory(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 certificates in history after renewal, got %d", len(history))
	}

	renewed, err := certs.LatestForDomain(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("expected renewed certificate to be saved: %v", err)
	}
	if renewed.ID == original.ID {
		t.Fatalf("expected renewal to create a new certificate row, got same ID %q", renewed.ID)
	}

	renewedChain, err := svc.DownloadFullChain(context.Background(), d.ID, renewed.ID)
	if err != nil {
		t.Fatalf("unexpected error downloading renewed cert: %v", err)
	}
	if string(renewedChain) != "cert-v2chain-v2" {
		t.Errorf("expected renewed fullchain %q, got %q", "cert-v2chain-v2", renewedChain)
	}

	renewedKey, err := svc.DownloadPrivateKey(context.Background(), d.ID, renewed.ID)
	if err != nil {
		t.Fatalf("unexpected error downloading renewed key: %v", err)
	}
	if string(renewedKey) != "key-v2" {
		t.Errorf("expected renewed key %q, got %q", "key-v2", renewedKey)
	}

	// The original (pre-renewal) certificate must remain downloadable too -
	// history retains every issued/renewed certificate, not just the latest.
	staleChain, err := svc.DownloadFullChain(context.Background(), d.ID, original.ID)
	if err != nil {
		t.Fatalf("unexpected error downloading original cert: %v", err)
	}
	if string(staleChain) != "cert-v1chain-v1" {
		t.Errorf("expected original fullchain %q, got %q", "cert-v1chain-v1", staleChain)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
