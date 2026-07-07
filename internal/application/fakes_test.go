package application

import (
	"context"
	"time"

	"certschedule/internal/domain"
)

// --- fakeDomainRepo ---

type fakeDomainRepo struct {
	byID map[string]*domain.Domain
	seq  int
}

func newFakeDomainRepo() *fakeDomainRepo {
	return &fakeDomainRepo{byID: map[string]*domain.Domain{}}
}

func (r *fakeDomainRepo) Create(ctx context.Context, d *domain.Domain) error {
	r.seq++
	if d.ID == "" {
		d.ID = "domain-" + itoa(r.seq)
	}
	cp := *d
	r.byID[d.ID] = &cp
	return nil
}

func (r *fakeDomainRepo) Update(ctx context.Context, d *domain.Domain) error {
	if _, ok := r.byID[d.ID]; !ok {
		return domain.ErrNotFound
	}
	cp := *d
	r.byID[d.ID] = &cp
	return nil
}

func (r *fakeDomainRepo) Delete(ctx context.Context, id string) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}

func (r *fakeDomainRepo) Get(ctx context.Context, id string) (*domain.Domain, error) {
	d, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *d
	return &cp, nil
}

func (r *fakeDomainRepo) List(ctx context.Context) ([]*domain.Domain, error) {
	out := make([]*domain.Domain, 0, len(r.byID))
	for _, d := range r.byID {
		cp := *d
		out = append(out, &cp)
	}
	return out, nil
}

func (r *fakeDomainRepo) ListDueForRenewal(ctx context.Context, now time.Time) ([]*domain.Domain, error) {
	var out []*domain.Domain
	for _, d := range r.byID {
		if d.AutoRenew {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

// --- fakeCertificateRepo ---

type fakeCertificateRepo struct {
	byDomain map[string][]*domain.Certificate
	seq      int
}

func newFakeCertificateRepo() *fakeCertificateRepo {
	return &fakeCertificateRepo{byDomain: map[string][]*domain.Certificate{}}
}

func (r *fakeCertificateRepo) Create(ctx context.Context, c *domain.Certificate) error {
	r.seq++
	if c.ID == "" {
		c.ID = "cert-" + itoa(r.seq)
	}
	cp := *c
	r.byDomain[c.DomainID] = append(r.byDomain[c.DomainID], &cp)
	return nil
}

func (r *fakeCertificateRepo) Get(ctx context.Context, id string) (*domain.Certificate, error) {
	for _, certs := range r.byDomain {
		for _, c := range certs {
			if c.ID == id {
				return c, nil
			}
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeCertificateRepo) LatestForDomain(ctx context.Context, domainID string) (*domain.Certificate, error) {
	certs := r.byDomain[domainID]
	if len(certs) == 0 {
		return nil, domain.ErrNotFound
	}
	return certs[len(certs)-1], nil
}

func (r *fakeCertificateRepo) HistoryForDomain(ctx context.Context, domainID string) ([]*domain.Certificate, error) {
	return r.byDomain[domainID], nil
}

// --- fakeJobRepo ---

type fakeJobRepo struct {
	byID map[string]*domain.RenewalJob
	seq  int
}

func newFakeJobRepo() *fakeJobRepo {
	return &fakeJobRepo{byID: map[string]*domain.RenewalJob{}}
}

func (r *fakeJobRepo) Create(ctx context.Context, j *domain.RenewalJob) error {
	r.seq++
	if j.ID == "" {
		j.ID = "job-" + itoa(r.seq)
	}
	r.byID[j.ID] = j
	return nil
}

func (r *fakeJobRepo) Update(ctx context.Context, j *domain.RenewalJob) error {
	if _, ok := r.byID[j.ID]; !ok {
		return domain.ErrNotFound
	}
	r.byID[j.ID] = j
	return nil
}

func (r *fakeJobRepo) ListForDomain(ctx context.Context, domainID string) ([]*domain.RenewalJob, error) {
	var out []*domain.RenewalJob
	for _, j := range r.byID {
		if j.DomainID == domainID {
			out = append(out, j)
		}
	}
	return out, nil
}

// --- fakeUserRepo ---

type fakeUserRepo struct {
	byUsername map[string]*domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byUsername: map[string]*domain.User{}}
}

func (r *fakeUserRepo) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	u, ok := r.byUsername[username]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

// --- fakeCertIssuer ---

type fakeCertIssuer struct {
	issueResult domain.CertResult
	issueErr    error
	renewResult domain.CertResult
	renewErr    error
	issueCalls  int
	renewCalls  int
}

func (f *fakeCertIssuer) Issue(ctx context.Context, req domain.IssueRequest) (domain.CertResult, error) {
	f.issueCalls++
	return f.issueResult, f.issueErr
}

func (f *fakeCertIssuer) Renew(ctx context.Context, req domain.RenewRequest) (domain.CertResult, error) {
	f.renewCalls++
	return f.renewResult, f.renewErr
}

// --- fakeSecretUpdater ---

type fakeSecretUpdater struct {
	calls       int
	err         error
	lastCertPEM []byte
	lastKeyPEM  []byte
}

func (f *fakeSecretUpdater) UpsertTLSSecret(ctx context.Context, namespace, name string, certPEM, keyPEM []byte) error {
	f.calls++
	f.lastCertPEM = certPEM
	f.lastKeyPEM = keyPEM
	return f.err
}

// --- fakeEncryptor (identity, for test simplicity) ---

type fakeEncryptor struct{}

func (fakeEncryptor) Encrypt(plaintext []byte) ([]byte, error) { return plaintext, nil }
func (fakeEncryptor) Decrypt(ciphertext []byte) ([]byte, error) { return ciphertext, nil }

// --- fakeClock ---

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

// --- fakeTokenIssuer ---

type fakeTokenIssuer struct {
	token string
	err   error
}

func (f *fakeTokenIssuer) Issue(userID, username string) (string, error) {
	return f.token, f.err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
