package domain

import (
	"context"
	"io"
	"time"
)

// DomainRepository persists Domain aggregates.
type DomainRepository interface {
	Create(ctx context.Context, d *Domain) error
	Update(ctx context.Context, d *Domain) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*Domain, error)
	List(ctx context.Context) ([]*Domain, error)
	// ListDueForRenewal returns domains with AutoRenew=true whose latest
	// certificate expires within RenewBeforeDays of now.
	ListDueForRenewal(ctx context.Context, now time.Time) ([]*Domain, error)
}

// CertificateRepository persists issued Certificate snapshots.
type CertificateRepository interface {
	Create(ctx context.Context, c *Certificate) error
	Get(ctx context.Context, id string) (*Certificate, error)
	LatestForDomain(ctx context.Context, domainID string) (*Certificate, error)
	HistoryForDomain(ctx context.Context, domainID string) ([]*Certificate, error)
}

// JobRepository persists RenewalJob history entries.
type JobRepository interface {
	Create(ctx context.Context, j *RenewalJob) error
	Update(ctx context.Context, j *RenewalJob) error
	ListForDomain(ctx context.Context, domainID string) ([]*RenewalJob, error)
}

// UserRepository looks up Users for authentication.
type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*User, error)
}

// IssueRequest carries everything the CertIssuer needs to run `certbot certonly`.
type IssueRequest struct {
	DomainName          string
	ChallengeType       ChallengeType
	DNSProvider         DNSProvider
	DecryptedCredential []byte // decrypted DNS provider credential material (ini contents / json)
	Email               string
}

// RenewRequest carries everything the CertIssuer needs to run `certbot renew`.
// certbot renew re-reads the saved renewal params for the cert, but DNS-01
// plugins still need their credentials file present on disk at the path
// recorded during the original Issue call, so the same fields are supplied
// again to let the adapter re-materialize it before renewing.
type RenewRequest struct {
	DomainName          string
	ChallengeType       ChallengeType
	DNSProvider         DNSProvider
	DecryptedCredential []byte
}

// CertResult is the parsed output of a successful certbot invocation.
type CertResult struct {
	CertPEM   []byte
	ChainPEM  []byte
	KeyPEM    []byte
	ExpiresAt time.Time
}

// CertIssuer wraps the certbot CLI to issue and renew certificates.
type CertIssuer interface {
	Issue(ctx context.Context, req IssueRequest) (CertResult, error)
	Renew(ctx context.Context, req RenewRequest) (CertResult, error)
}

// SecretUpdater pushes cert material into a Kubernetes TLS Secret.
type SecretUpdater interface {
	UpsertTLSSecret(ctx context.Context, namespace, name string, certPEM, keyPEM []byte) error
}

// Encryptor protects sensitive material (DNS credentials, private keys) at rest.
type Encryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// CertbotArchiver produces a full backup (tar.gz) of everything certbot has
// generated on disk (config/live, config/archive, config/renewal,
// config/accounts, DNS credential files) - a complete on-disk snapshot for
// migration/disaster-recovery, distinct from the app's own encrypted-at-rest
// Domain/Certificate database records.
type CertbotArchiver interface {
	WriteFullArchive(ctx context.Context, w io.Writer) error
}

// Clock is an injectable time source, primarily for deterministic tests.
type Clock interface {
	Now() time.Time
}

// SystemClock is the production Clock backed by time.Now.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
