// Package domain contains the core business entities and port interfaces.
// It has zero dependencies on outer layers (application, adapters).
package domain

import "time"

// ChallengeType is the ACME challenge mechanism used to prove domain ownership.
type ChallengeType string

const (
	ChallengeDNS01  ChallengeType = "dns-01"
	ChallengeHTTP01 ChallengeType = "http-01"
)

// DNSProvider identifies which DNS plugin certbot should use for dns-01 challenges.
type DNSProvider string

const (
	DNSProviderNone       DNSProvider = ""
	DNSProviderCloudflare DNSProvider = "cloudflare"
	DNSProviderRoute53    DNSProvider = "route53"
)

// DomainStatus reflects the current lifecycle state of a managed domain.
type DomainStatus string

const (
	DomainStatusPending DomainStatus = "pending"
	DomainStatusActive  DomainStatus = "active"
	DomainStatusFailed  DomainStatus = "failed"
	DomainStatusExpired DomainStatus = "expired"
)

// Domain represents a hostname managed by the app for cert issuance/renewal
// and the k8s Secret it should be synced into.
type Domain struct {
	ID                 string
	Name               string
	ChallengeType      ChallengeType
	DNSProvider        DNSProvider
	EncryptedCredential []byte // encrypted DNS provider credentials (nil for http-01)
	K8sNamespace       string
	K8sSecretName      string
	AutoRenew          bool
	RenewBeforeDays    int
	Status             DomainStatus
	LastError          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
