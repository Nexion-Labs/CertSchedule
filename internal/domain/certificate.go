package domain

import "time"

// CertificateStatus reflects the outcome of an issuance/renewal attempt.
type CertificateStatus string

const (
	CertificateStatusActive CertificateStatus = "active"
	CertificateStatusFailed CertificateStatus = "failed"
)

// Certificate is a snapshot of an issued/renewed TLS certificate for a Domain.
type Certificate struct {
	ID          string
	DomainID    string
	IssuedAt    time.Time
	ExpiresAt   time.Time
	CertPEM     []byte
	ChainPEM    []byte
	EncryptedKeyPEM []byte // private key, encrypted at rest
	Status      CertificateStatus
	CreatedAt   time.Time
}

// TriggerType records what initiated a renewal job.
type TriggerType string

const (
	TriggerManual    TriggerType = "manual"
	TriggerScheduled TriggerType = "scheduled"
)

// JobStatus is the outcome of a RenewalJob.
type JobStatus string

const (
	JobStatusRunning JobStatus = "running"
	JobStatusSuccess JobStatus = "success"
	JobStatusFailed  JobStatus = "failed"
)

// RenewalJob records a single issue/renew attempt for history/audit purposes.
type RenewalJob struct {
	ID         string
	DomainID   string
	Trigger    TriggerType
	Status     JobStatus
	Message    string
	StartedAt  time.Time
	FinishedAt *time.Time
}
