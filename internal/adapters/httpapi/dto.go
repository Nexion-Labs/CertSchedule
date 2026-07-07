package httpapi

import (
	"time"

	"certschedule/internal/application"
	"certschedule/internal/domain"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type domainRequest struct {
	Name                   string `json:"name"`
	ChallengeType          string `json:"challenge_type"`
	DNSProvider            string `json:"dns_provider"`
	K8sNamespace           string `json:"k8s_namespace"`
	K8sSecretName          string `json:"k8s_secret_name"`
	AutoRenew              bool   `json:"auto_renew"`
	RenewBeforeDays        int    `json:"renew_before_days"`
	CloudflareAPIToken     string `json:"cloudflare_api_token,omitempty"`
	Route53AccessKeyID     string `json:"route53_access_key_id,omitempty"`
	Route53SecretAccessKey string `json:"route53_secret_access_key,omitempty"`
}

func (r domainRequest) toCreateInput() application.CreateDomainInput {
	return application.CreateDomainInput{
		Name:                   r.Name,
		ChallengeType:          domain.ChallengeType(r.ChallengeType),
		DNSProvider:            domain.DNSProvider(r.DNSProvider),
		K8sNamespace:           r.K8sNamespace,
		K8sSecretName:          r.K8sSecretName,
		AutoRenew:              r.AutoRenew,
		RenewBeforeDays:        r.RenewBeforeDays,
		CloudflareAPIToken:     r.CloudflareAPIToken,
		Route53AccessKeyID:     r.Route53AccessKeyID,
		Route53SecretAccessKey: r.Route53SecretAccessKey,
	}
}

func (r domainRequest) toUpdateInput() application.UpdateDomainInput {
	return application.UpdateDomainInput{
		Name:                   r.Name,
		K8sNamespace:           r.K8sNamespace,
		K8sSecretName:          r.K8sSecretName,
		AutoRenew:              r.AutoRenew,
		RenewBeforeDays:        r.RenewBeforeDays,
		CloudflareAPIToken:     r.CloudflareAPIToken,
		Route53AccessKeyID:     r.Route53AccessKeyID,
		Route53SecretAccessKey: r.Route53SecretAccessKey,
	}
}

func (r domainRequest) toDomainConfig() application.DomainConfig {
	return application.DomainConfig{
		Name:                   r.Name,
		ChallengeType:          domain.ChallengeType(r.ChallengeType),
		DNSProvider:            domain.DNSProvider(r.DNSProvider),
		K8sNamespace:           r.K8sNamespace,
		K8sSecretName:          r.K8sSecretName,
		AutoRenew:              r.AutoRenew,
		RenewBeforeDays:        r.RenewBeforeDays,
		CloudflareAPIToken:     r.CloudflareAPIToken,
		Route53AccessKeyID:     r.Route53AccessKeyID,
		Route53SecretAccessKey: r.Route53SecretAccessKey,
	}
}

func newDomainConfigItem(c application.DomainConfig) domainRequest {
	return domainRequest{
		Name:                   c.Name,
		ChallengeType:          string(c.ChallengeType),
		DNSProvider:            string(c.DNSProvider),
		K8sNamespace:           c.K8sNamespace,
		K8sSecretName:          c.K8sSecretName,
		AutoRenew:              c.AutoRenew,
		RenewBeforeDays:        c.RenewBeforeDays,
		CloudflareAPIToken:     c.CloudflareAPIToken,
		Route53AccessKeyID:     c.Route53AccessKeyID,
		Route53SecretAccessKey: c.Route53SecretAccessKey,
	}
}

// domainExportResponse is the downloadable backup format for GET
// /domains/export. Domains omit credential fields unless
// ?include_credentials=true was requested.
type domainExportResponse struct {
	Version    int             `json:"version"`
	ExportedAt time.Time       `json:"exported_at"`
	Domains    []domainRequest `json:"domains"`
}

func newDomainExportResponse(configs []application.DomainConfig, now time.Time) domainExportResponse {
	items := make([]domainRequest, 0, len(configs))
	for _, c := range configs {
		items = append(items, newDomainConfigItem(c))
	}
	return domainExportResponse{Version: 1, ExportedAt: now, Domains: items}
}

// domainImportRequest accepts the same shape produced by domainExportResponse.
type domainImportRequest struct {
	Domains []domainRequest `json:"domains"`
}

type domainImportResultItem struct {
	Name   string `json:"name"`
	Action string `json:"action"`
	Error  string `json:"error,omitempty"`
}

type domainImportResponse struct {
	Results []domainImportResultItem `json:"results"`
}

func newDomainImportResponse(results []application.ImportResult) domainImportResponse {
	items := make([]domainImportResultItem, 0, len(results))
	for _, r := range results {
		items = append(items, domainImportResultItem{Name: r.Name, Action: string(r.Action), Error: r.Error})
	}
	return domainImportResponse{Results: items}
}

type domainResponse struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	ChallengeType   string    `json:"challenge_type"`
	DNSProvider     string    `json:"dns_provider"`
	K8sNamespace    string    `json:"k8s_namespace"`
	K8sSecretName   string    `json:"k8s_secret_name"`
	AutoRenew       bool      `json:"auto_renew"`
	RenewBeforeDays int       `json:"renew_before_days"`
	Status          string    `json:"status"`
	LastError       string    `json:"last_error,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func newDomainResponse(d *domain.Domain) domainResponse {
	return domainResponse{
		ID: d.ID, Name: d.Name,
		ChallengeType: string(d.ChallengeType), DNSProvider: string(d.DNSProvider),
		K8sNamespace: d.K8sNamespace, K8sSecretName: d.K8sSecretName,
		AutoRenew: d.AutoRenew, RenewBeforeDays: d.RenewBeforeDays,
		Status: string(d.Status), LastError: d.LastError,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

type certificateResponse struct {
	ID        string    `json:"id"`
	DomainID  string    `json:"domain_id"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func newCertificateResponse(c *domain.Certificate) certificateResponse {
	return certificateResponse{
		ID: c.ID, DomainID: c.DomainID, IssuedAt: c.IssuedAt, ExpiresAt: c.ExpiresAt,
		Status: string(c.Status), CreatedAt: c.CreatedAt,
	}
}

// certificateDetailResponse is the "view certificate" payload: base metadata
// plus fields parsed from the X.509 payload (subject, issuer, SANs, serial).
type certificateDetailResponse struct {
	ID           string    `json:"id"`
	DomainID     string    `json:"domain_id"`
	IssuedAt     time.Time `json:"issued_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	SerialNumber string    `json:"serial_number,omitempty"`
	Issuer       string    `json:"issuer,omitempty"`
	SubjectCN    string    `json:"subject_cn,omitempty"`
	DNSNames     []string  `json:"dns_names,omitempty"`
}

func newCertificateDetailResponse(d *application.CertificateDetail) certificateDetailResponse {
	return certificateDetailResponse{
		ID: d.ID, DomainID: d.DomainID, IssuedAt: d.IssuedAt, ExpiresAt: d.ExpiresAt,
		Status: string(d.Status), CreatedAt: d.CreatedAt,
		SerialNumber: d.SerialNumber, Issuer: d.Issuer, SubjectCN: d.SubjectCN, DNSNames: d.DNSNames,
	}
}

type jobResponse struct {
	ID         string     `json:"id"`
	DomainID   string     `json:"domain_id"`
	Trigger    string     `json:"trigger"`
	Status     string     `json:"status"`
	Message    string     `json:"message,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

func newJobResponse(j *domain.RenewalJob) jobResponse {
	return jobResponse{
		ID: j.ID, DomainID: j.DomainID, Trigger: string(j.Trigger), Status: string(j.Status),
		Message: j.Message, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
	}
}

type errorResponse struct {
	Error string `json:"error"`
}
