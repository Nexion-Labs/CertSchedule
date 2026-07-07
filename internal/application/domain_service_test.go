package application

import (
	"context"
	"errors"
	"testing"

	"certschedule/internal/domain"
)

func TestDomainService_Create_HTTP01_NoCredentialNeeded(t *testing.T) {
	svc := NewDomainService(newFakeDomainRepo(), fakeEncryptor{})

	d, err := svc.Create(context.Background(), CreateDomainInput{
		Name:          "example.com",
		ChallengeType: domain.ChallengeHTTP01,
		K8sNamespace:  "default",
		K8sSecretName: "example-tls",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != domain.DomainStatusPending {
		t.Errorf("expected pending status, got %s", d.Status)
	}
	if len(d.EncryptedCredential) != 0 {
		t.Errorf("expected no credential for http-01, got %v", d.EncryptedCredential)
	}
}

func TestDomainService_Create_DNS01_Cloudflare_RequiresToken(t *testing.T) {
	svc := NewDomainService(newFakeDomainRepo(), fakeEncryptor{})

	_, err := svc.Create(context.Background(), CreateDomainInput{
		Name:          "example.com",
		ChallengeType: domain.ChallengeDNS01,
		DNSProvider:   domain.DNSProviderCloudflare,
	})
	if err == nil {
		t.Fatal("expected error when cloudflare token is missing")
	}
}

func TestDomainService_Create_DNS01_Cloudflare_EncryptsToken(t *testing.T) {
	svc := NewDomainService(newFakeDomainRepo(), fakeEncryptor{})

	d, err := svc.Create(context.Background(), CreateDomainInput{
		Name:               "example.com",
		ChallengeType:      domain.ChallengeDNS01,
		DNSProvider:        domain.DNSProviderCloudflare,
		CloudflareAPIToken: "secret-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.EncryptedCredential) == 0 {
		t.Fatal("expected credential to be stored")
	}
}

func TestDomainService_Update_PreservesCredentialWhenNotSupplied(t *testing.T) {
	repo := newFakeDomainRepo()
	svc := NewDomainService(repo, fakeEncryptor{})

	d, err := svc.Create(context.Background(), CreateDomainInput{
		Name:               "example.com",
		ChallengeType:      domain.ChallengeDNS01,
		DNSProvider:        domain.DNSProviderCloudflare,
		CloudflareAPIToken: "secret-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	original := d.EncryptedCredential

	updated, err := svc.Update(context.Background(), d.ID, UpdateDomainInput{
		K8sNamespace:  "prod",
		K8sSecretName: "example-tls",
		AutoRenew:     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(updated.EncryptedCredential) != string(original) {
		t.Errorf("expected credential to be preserved, got different value")
	}
	if updated.K8sNamespace != "prod" || !updated.AutoRenew {
		t.Errorf("expected updated fields to apply, got %+v", updated)
	}
}

func TestDomainService_Update_AllowsRenamingDomain(t *testing.T) {
	repo := newFakeDomainRepo()
	svc := NewDomainService(repo, fakeEncryptor{})
	ctx := context.Background()

	d, err := svc.Create(ctx, CreateDomainInput{
		Name:          "old-name.example.com",
		ChallengeType: domain.ChallengeHTTP01,
		K8sNamespace:  "default",
		K8sSecretName: "tls",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := svc.Update(ctx, d.ID, UpdateDomainInput{
		Name:          "new-name.example.com",
		K8sNamespace:  "default",
		K8sSecretName: "tls",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "new-name.example.com" {
		t.Errorf("expected name to be updated, got %q", updated.Name)
	}
}

func TestDomainService_Update_RejectsRenamingToExistingName(t *testing.T) {
	repo := newFakeDomainRepo()
	svc := NewDomainService(repo, fakeEncryptor{})
	ctx := context.Background()

	_, err := svc.Create(ctx, CreateDomainInput{
		Name:          "taken.example.com",
		ChallengeType: domain.ChallengeHTTP01,
		K8sNamespace:  "default",
		K8sSecretName: "taken-tls",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d2, err := svc.Create(ctx, CreateDomainInput{
		Name:          "other.example.com",
		ChallengeType: domain.ChallengeHTTP01,
		K8sNamespace:  "default",
		K8sSecretName: "other-tls",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.Update(ctx, d2.ID, UpdateDomainInput{
		Name:          "taken.example.com",
		K8sNamespace:  "default",
		K8sSecretName: "other-tls",
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestDomainService_Export_ExcludesCredentialsByDefault(t *testing.T) {
	svc := NewDomainService(newFakeDomainRepo(), fakeEncryptor{})
	ctx := context.Background()

	_, err := svc.Create(ctx, CreateDomainInput{
		Name:               "cf.example.com",
		ChallengeType:      domain.ChallengeDNS01,
		DNSProvider:        domain.DNSProviderCloudflare,
		CloudflareAPIToken: "super-secret-token",
		K8sNamespace:       "default",
		K8sSecretName:      "cf-tls",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configs, err := svc.Export(ctx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].CloudflareAPIToken != "" {
		t.Errorf("expected credential to be omitted, got %q", configs[0].CloudflareAPIToken)
	}
}

func TestDomainService_Export_IncludesCredentialsWhenRequested(t *testing.T) {
	svc := NewDomainService(newFakeDomainRepo(), fakeEncryptor{})
	ctx := context.Background()

	_, err := svc.Create(ctx, CreateDomainInput{
		Name:               "cf.example.com",
		ChallengeType:      domain.ChallengeDNS01,
		DNSProvider:        domain.DNSProviderCloudflare,
		CloudflareAPIToken: "super-secret-token",
		K8sNamespace:       "default",
		K8sSecretName:      "cf-tls",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configs, err := svc.Export(ctx, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].CloudflareAPIToken != "super-secret-token" {
		t.Errorf("expected decrypted token round-trip, got %q", configs[0].CloudflareAPIToken)
	}
}

func TestDomainService_Import_CreatesNewAndUpdatesExisting(t *testing.T) {
	svc := NewDomainService(newFakeDomainRepo(), fakeEncryptor{})
	ctx := context.Background()

	existing, err := svc.Create(ctx, CreateDomainInput{
		Name:          "existing.example.com",
		ChallengeType: domain.ChallengeHTTP01,
		K8sNamespace:  "default",
		K8sSecretName: "old-secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results, err := svc.Import(ctx, []DomainConfig{
		{
			Name:          "existing.example.com",
			ChallengeType: domain.ChallengeHTTP01,
			K8sNamespace:  "default",
			K8sSecretName: "new-secret",
			AutoRenew:     true,
		},
		{
			Name:          "new.example.com",
			ChallengeType: domain.ChallengeHTTP01,
			K8sNamespace:  "default",
			K8sSecretName: "new-tls",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Action != ImportActionUpdated {
		t.Errorf("expected updated action for existing domain, got %s (%s)", results[0].Action, results[0].Error)
	}
	if results[1].Action != ImportActionCreated {
		t.Errorf("expected created action for new domain, got %s (%s)", results[1].Action, results[1].Error)
	}

	updated, err := svc.Get(ctx, existing.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.K8sSecretName != "new-secret" || !updated.AutoRenew {
		t.Errorf("expected existing domain to be updated in place, got %+v", updated)
	}

	all, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 domains after import, got %d", len(all))
	}
}

func TestDomainService_Import_RejectsMissingName(t *testing.T) {
	svc := NewDomainService(newFakeDomainRepo(), fakeEncryptor{})

	results, err := svc.Import(context.Background(), []DomainConfig{{ChallengeType: domain.ChallengeHTTP01}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Action != ImportActionError {
		t.Fatalf("expected a single error result, got %+v", results)
	}
}
