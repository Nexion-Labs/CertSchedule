// Package application contains the use cases (business logic orchestration).
// Services here depend only on domain ports, never on concrete adapters.
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"certschedule/internal/domain"
)

// CreateDomainInput is the application-layer request to register a new
// managed domain. Exactly one of the credential fields should be populated,
// matching the chosen DNSProvider (neither for http-01).
type CreateDomainInput struct {
	Name            string
	ChallengeType   domain.ChallengeType
	DNSProvider     domain.DNSProvider
	K8sNamespace    string
	K8sSecretName   string
	AutoRenew       bool
	RenewBeforeDays int

	CloudflareAPIToken     string
	Route53AccessKeyID     string
	Route53SecretAccessKey string
}

// UpdateDomainInput mirrors CreateDomainInput for edits. Credential fields
// are only applied if non-empty, so callers can update other settings
// without re-supplying secrets.
type UpdateDomainInput struct {
	Name            string
	K8sNamespace    string
	K8sSecretName   string
	AutoRenew       bool
	RenewBeforeDays int

	CloudflareAPIToken     string
	Route53AccessKeyID     string
	Route53SecretAccessKey string
}

type route53Credentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

// DomainService implements domain create/update/delete/list use cases.
type DomainService struct {
	repo      domain.DomainRepository
	encryptor domain.Encryptor
}

func NewDomainService(repo domain.DomainRepository, encryptor domain.Encryptor) *DomainService {
	return &DomainService{repo: repo, encryptor: encryptor}
}

// encryptCredential builds the canonical plaintext credential blob expected
// by the certbot adapter for the given provider, then encrypts it.
func (s *DomainService) encryptCredential(provider domain.DNSProvider, cloudflareToken, route53AccessKey, route53SecretKey string) ([]byte, error) {
	var plaintext []byte
	switch provider {
	case domain.DNSProviderCloudflare:
		if cloudflareToken == "" {
			return nil, fmt.Errorf("%w: cloudflare api token is required", domain.ErrInvalidInput)
		}
		plaintext = []byte(fmt.Sprintf("dns_cloudflare_api_token = %s\n", cloudflareToken))
	case domain.DNSProviderRoute53:
		if route53AccessKey == "" || route53SecretKey == "" {
			return nil, fmt.Errorf("%w: route53 access key id and secret access key are required", domain.ErrInvalidInput)
		}
		b, err := json.Marshal(route53Credentials{AccessKeyID: route53AccessKey, SecretAccessKey: route53SecretKey})
		if err != nil {
			return nil, err
		}
		plaintext = b
	case domain.DNSProviderNone:
		return nil, nil
	default:
		return nil, fmt.Errorf("%w: unknown dns provider %q", domain.ErrInvalidInput, provider)
	}
	return s.encryptor.Encrypt(plaintext)
}

func (s *DomainService) Create(ctx context.Context, in CreateDomainInput) (*domain.Domain, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	if in.ChallengeType != domain.ChallengeHTTP01 && in.ChallengeType != domain.ChallengeDNS01 {
		return nil, fmt.Errorf("%w: challenge_type must be http-01 or dns-01", domain.ErrInvalidInput)
	}
	if in.RenewBeforeDays <= 0 {
		in.RenewBeforeDays = 30
	}

	var encryptedCred []byte
	if in.ChallengeType == domain.ChallengeDNS01 {
		cred, err := s.encryptCredential(in.DNSProvider, in.CloudflareAPIToken, in.Route53AccessKeyID, in.Route53SecretAccessKey)
		if err != nil {
			return nil, err
		}
		encryptedCred = cred
	}

	d := &domain.Domain{
		Name:                in.Name,
		ChallengeType:       in.ChallengeType,
		DNSProvider:         in.DNSProvider,
		EncryptedCredential: encryptedCred,
		K8sNamespace:        in.K8sNamespace,
		K8sSecretName:       in.K8sSecretName,
		AutoRenew:           in.AutoRenew,
		RenewBeforeDays:     in.RenewBeforeDays,
		Status:              domain.DomainStatusPending,
	}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *DomainService) Update(ctx context.Context, id string, in UpdateDomainInput) (*domain.Domain, error) {
	d, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Name != "" && in.Name != d.Name {
		existing, err := s.repo.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, other := range existing {
			if other.ID != id && other.Name == in.Name {
				return nil, fmt.Errorf("%w: domain name %q is already in use", domain.ErrAlreadyExists, in.Name)
			}
		}
		d.Name = in.Name
	}

	d.K8sNamespace = in.K8sNamespace
	d.K8sSecretName = in.K8sSecretName
	d.AutoRenew = in.AutoRenew
	if in.RenewBeforeDays > 0 {
		d.RenewBeforeDays = in.RenewBeforeDays
	}

	if in.CloudflareAPIToken != "" || in.Route53AccessKeyID != "" {
		cred, err := s.encryptCredential(d.DNSProvider, in.CloudflareAPIToken, in.Route53AccessKeyID, in.Route53SecretAccessKey)
		if err != nil {
			return nil, err
		}
		d.EncryptedCredential = cred
	}

	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *DomainService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *DomainService) Get(ctx context.Context, id string) (*domain.Domain, error) {
	return s.repo.Get(ctx, id)
}

func (s *DomainService) List(ctx context.Context) ([]*domain.Domain, error) {
	return s.repo.List(ctx)
}

// DomainConfig is the portable representation of a domain's configuration,
// used by Export/Import. Credential fields are only populated on export when
// explicitly requested, since they carry decrypted plaintext secrets.
type DomainConfig struct {
	Name            string
	ChallengeType   domain.ChallengeType
	DNSProvider     domain.DNSProvider
	K8sNamespace    string
	K8sSecretName   string
	AutoRenew       bool
	RenewBeforeDays int

	CloudflareAPIToken     string
	Route53AccessKeyID     string
	Route53SecretAccessKey string
}

// Export returns the portable config for every managed domain. Credentials
// are decrypted and included only if includeCredentials is true — callers
// must treat that output as a plaintext secrets dump.
func (s *DomainService) Export(ctx context.Context, includeCredentials bool) ([]DomainConfig, error) {
	domains, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]DomainConfig, 0, len(domains))
	for _, d := range domains {
		cfg := DomainConfig{
			Name:            d.Name,
			ChallengeType:   d.ChallengeType,
			DNSProvider:     d.DNSProvider,
			K8sNamespace:    d.K8sNamespace,
			K8sSecretName:   d.K8sSecretName,
			AutoRenew:       d.AutoRenew,
			RenewBeforeDays: d.RenewBeforeDays,
		}
		if includeCredentials && len(d.EncryptedCredential) > 0 {
			plaintext, err := s.encryptor.Decrypt(d.EncryptedCredential)
			if err != nil {
				return nil, fmt.Errorf("decrypt credential for %s: %w", d.Name, err)
			}
			switch d.DNSProvider {
			case domain.DNSProviderCloudflare:
				cfg.CloudflareAPIToken = parseCloudflareToken(plaintext)
			case domain.DNSProviderRoute53:
				var c route53Credentials
				if err := json.Unmarshal(plaintext, &c); err != nil {
					return nil, fmt.Errorf("decode route53 credential for %s: %w", d.Name, err)
				}
				cfg.Route53AccessKeyID = c.AccessKeyID
				cfg.Route53SecretAccessKey = c.SecretAccessKey
			}
		}
		out = append(out, cfg)
	}
	return out, nil
}

func parseCloudflareToken(plaintext []byte) string {
	_, token, found := strings.Cut(strings.TrimSpace(string(plaintext)), "=")
	if !found {
		return ""
	}
	return strings.TrimSpace(token)
}

// ImportAction describes what Import did with a single config entry.
type ImportAction string

const (
	ImportActionCreated ImportAction = "created"
	ImportActionUpdated ImportAction = "updated"
	ImportActionError   ImportAction = "error"
)

// ImportResult reports the outcome of importing a single DomainConfig.
type ImportResult struct {
	Name   string
	Action ImportAction
	Error  string
}

// Import upserts domains by name: an existing domain with a matching name
// has its non-credential settings updated (credentials replaced only if
// supplied), otherwise a new domain is created. Credentials are optional on
// create too — a DNS-01 domain imported without one is left unable to issue
// until credentials are added via Update, same as a manually created domain
// left mid-edit. Failures on one entry don't stop the rest from importing.
func (s *DomainService) Import(ctx context.Context, configs []DomainConfig) ([]ImportResult, error) {
	existing, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*domain.Domain, len(existing))
	for _, d := range existing {
		byName[d.Name] = d
	}

	results := make([]ImportResult, 0, len(configs))
	for _, cfg := range configs {
		if cfg.Name == "" {
			results = append(results, ImportResult{Action: ImportActionError, Error: "name is required"})
			continue
		}
		if cfg.RenewBeforeDays <= 0 {
			cfg.RenewBeforeDays = 30
		}
		hasCredential := cfg.CloudflareAPIToken != "" || cfg.Route53AccessKeyID != ""

		if d, ok := byName[cfg.Name]; ok {
			d.K8sNamespace = cfg.K8sNamespace
			d.K8sSecretName = cfg.K8sSecretName
			d.AutoRenew = cfg.AutoRenew
			d.RenewBeforeDays = cfg.RenewBeforeDays
			if hasCredential {
				cred, err := s.encryptCredential(d.DNSProvider, cfg.CloudflareAPIToken, cfg.Route53AccessKeyID, cfg.Route53SecretAccessKey)
				if err != nil {
					results = append(results, ImportResult{Name: cfg.Name, Action: ImportActionError, Error: err.Error()})
					continue
				}
				d.EncryptedCredential = cred
			}
			if err := s.repo.Update(ctx, d); err != nil {
				results = append(results, ImportResult{Name: cfg.Name, Action: ImportActionError, Error: err.Error()})
				continue
			}
			results = append(results, ImportResult{Name: cfg.Name, Action: ImportActionUpdated})
			continue
		}

		if cfg.ChallengeType != domain.ChallengeHTTP01 && cfg.ChallengeType != domain.ChallengeDNS01 {
			results = append(results, ImportResult{Name: cfg.Name, Action: ImportActionError, Error: "challenge_type must be http-01 or dns-01"})
			continue
		}

		var encryptedCred []byte
		if cfg.ChallengeType == domain.ChallengeDNS01 && hasCredential {
			cred, err := s.encryptCredential(cfg.DNSProvider, cfg.CloudflareAPIToken, cfg.Route53AccessKeyID, cfg.Route53SecretAccessKey)
			if err != nil {
				results = append(results, ImportResult{Name: cfg.Name, Action: ImportActionError, Error: err.Error()})
				continue
			}
			encryptedCred = cred
		}

		d := &domain.Domain{
			Name:                cfg.Name,
			ChallengeType:       cfg.ChallengeType,
			DNSProvider:         cfg.DNSProvider,
			EncryptedCredential: encryptedCred,
			K8sNamespace:        cfg.K8sNamespace,
			K8sSecretName:       cfg.K8sSecretName,
			AutoRenew:           cfg.AutoRenew,
			RenewBeforeDays:     cfg.RenewBeforeDays,
			Status:              domain.DomainStatusPending,
		}
		if err := s.repo.Create(ctx, d); err != nil {
			results = append(results, ImportResult{Name: cfg.Name, Action: ImportActionError, Error: err.Error()})
			continue
		}
		byName[cfg.Name] = d
		results = append(results, ImportResult{Name: cfg.Name, Action: ImportActionCreated})
	}
	return results, nil
}
