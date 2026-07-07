package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"certschedule/internal/domain"

	"github.com/google/uuid"
)

// DomainRepo implements domain.DomainRepository backed by SQLite.
type DomainRepo struct {
	db *sql.DB
}

func NewDomainRepo(db *sql.DB) *DomainRepo {
	return &DomainRepo{db: db}
}

func (r *DomainRepo) Create(ctx context.Context, d *domain.Domain) error {
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	d.CreatedAt, d.UpdatedAt = now, now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO domains (id, name, challenge_type, dns_provider, encrypted_credential,
			k8s_namespace, k8s_secret_name, auto_renew, renew_before_days, status, last_error,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Name, string(d.ChallengeType), string(d.DNSProvider), d.EncryptedCredential,
		d.K8sNamespace, d.K8sSecretName, d.AutoRenew, d.RenewBeforeDays, string(d.Status), d.LastError,
		d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert domain: %w", err)
	}
	return nil
}

func (r *DomainRepo) Update(ctx context.Context, d *domain.Domain) error {
	d.UpdatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE domains SET name=?, challenge_type=?, dns_provider=?, encrypted_credential=?,
			k8s_namespace=?, k8s_secret_name=?, auto_renew=?, renew_before_days=?, status=?,
			last_error=?, updated_at=?
		WHERE id=?`,
		d.Name, string(d.ChallengeType), string(d.DNSProvider), d.EncryptedCredential,
		d.K8sNamespace, d.K8sSecretName, d.AutoRenew, d.RenewBeforeDays, string(d.Status),
		d.LastError, d.UpdatedAt, d.ID,
	)
	if err != nil {
		return fmt.Errorf("update domain: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *DomainRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM domains WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete domain: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *DomainRepo) Get(ctx context.Context, id string) (*domain.Domain, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, challenge_type, dns_provider, encrypted_credential, k8s_namespace,
			k8s_secret_name, auto_renew, renew_before_days, status, last_error, created_at, updated_at
		FROM domains WHERE id=?`, id)
	d, err := scanDomain(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *DomainRepo) List(ctx context.Context) ([]*domain.Domain, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, challenge_type, dns_provider, encrypted_credential, k8s_namespace,
			k8s_secret_name, auto_renew, renew_before_days, status, last_error, created_at, updated_at
		FROM domains ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()
	return scanDomains(rows)
}

// ListDueForRenewal returns auto-renew domains whose latest certificate
// expires within RenewBeforeDays of now (or has no certificate at all yet).
func (r *DomainRepo) ListDueForRenewal(ctx context.Context, now time.Time) ([]*domain.Domain, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.id, d.name, d.challenge_type, d.dns_provider, d.encrypted_credential,
			d.k8s_namespace, d.k8s_secret_name, d.auto_renew, d.renew_before_days, d.status,
			d.last_error, d.created_at, d.updated_at
		FROM domains d
		WHERE d.auto_renew = 1
		AND (
			NOT EXISTS (SELECT 1 FROM certificates c WHERE c.domain_id = d.id)
			OR (
				SELECT c.expires_at FROM certificates c
				WHERE c.domain_id = d.id
				ORDER BY c.created_at DESC LIMIT 1
			) <= datetime(?, '+' || d.renew_before_days || ' days')
		)`, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("list due domains: %w", err)
	}
	defer rows.Close()
	return scanDomains(rows)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDomain(row rowScanner) (*domain.Domain, error) {
	var d domain.Domain
	var challengeType, dnsProvider, status string
	if err := row.Scan(&d.ID, &d.Name, &challengeType, &dnsProvider, &d.EncryptedCredential,
		&d.K8sNamespace, &d.K8sSecretName, &d.AutoRenew, &d.RenewBeforeDays, &status,
		&d.LastError, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	d.ChallengeType = domain.ChallengeType(challengeType)
	d.DNSProvider = domain.DNSProvider(dnsProvider)
	d.Status = domain.DomainStatus(status)
	return &d, nil
}

func scanDomains(rows *sql.Rows) ([]*domain.Domain, error) {
	var out []*domain.Domain
	for rows.Next() {
		d, err := scanDomain(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
