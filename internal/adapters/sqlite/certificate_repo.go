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

// CertificateRepo implements domain.CertificateRepository backed by SQLite.
type CertificateRepo struct {
	db *sql.DB
}

func NewCertificateRepo(db *sql.DB) *CertificateRepo {
	return &CertificateRepo{db: db}
}

func (r *CertificateRepo) Create(ctx context.Context, c *domain.Certificate) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	c.CreatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO certificates (id, domain_id, issued_at, expires_at, cert_pem, chain_pem,
			encrypted_key_pem, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.DomainID, c.IssuedAt, c.ExpiresAt, c.CertPEM, c.ChainPEM,
		c.EncryptedKeyPEM, string(c.Status), c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert certificate: %w", err)
	}
	return nil
}

func (r *CertificateRepo) Get(ctx context.Context, id string) (*domain.Certificate, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, domain_id, issued_at, expires_at, cert_pem, chain_pem, encrypted_key_pem, status, created_at
		FROM certificates WHERE id=?`, id)
	c, err := scanCertificate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *CertificateRepo) LatestForDomain(ctx context.Context, domainID string) (*domain.Certificate, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, domain_id, issued_at, expires_at, cert_pem, chain_pem, encrypted_key_pem, status, created_at
		FROM certificates WHERE domain_id=? ORDER BY created_at DESC LIMIT 1`, domainID)
	c, err := scanCertificate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *CertificateRepo) HistoryForDomain(ctx context.Context, domainID string) ([]*domain.Certificate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, domain_id, issued_at, expires_at, cert_pem, chain_pem, encrypted_key_pem, status, created_at
		FROM certificates WHERE domain_id=? ORDER BY created_at DESC`, domainID)
	if err != nil {
		return nil, fmt.Errorf("list certificate history: %w", err)
	}
	defer rows.Close()

	var out []*domain.Certificate
	for rows.Next() {
		c, err := scanCertificate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanCertificate(row rowScanner) (*domain.Certificate, error) {
	var c domain.Certificate
	var status string
	if err := row.Scan(&c.ID, &c.DomainID, &c.IssuedAt, &c.ExpiresAt, &c.CertPEM, &c.ChainPEM,
		&c.EncryptedKeyPEM, &status, &c.CreatedAt); err != nil {
		return nil, err
	}
	c.Status = domain.CertificateStatus(status)
	return &c, nil
}
