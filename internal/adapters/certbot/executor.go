// Package certbot wraps the certbot CLI (invoked as a subprocess) to
// implement the domain.CertIssuer port.
package certbot

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"certschedule/internal/domain"
)

// Config controls how the certbot subprocess is invoked.
type Config struct {
	Binary  string // e.g. "certbot"
	DataDir string // base dir; config-dir/work-dir/logs-dir/credentials live under here
	Staging bool   // use Let's Encrypt staging ACME endpoint
	DryRun  bool   // pass --dry-run, certbot performs no persistent changes
	Webroot string // used for http-01 challenges
	Email   string // ACME account contact
}

// Executor implements domain.CertIssuer by shelling out to certbot.
type Executor struct {
	cfg Config
}

func New(cfg Config) *Executor {
	return &Executor{cfg: cfg}
}

func (e *Executor) configDir() string { return filepath.Join(e.cfg.DataDir, "config") }
func (e *Executor) workDir() string   { return filepath.Join(e.cfg.DataDir, "work") }
func (e *Executor) logsDir() string   { return filepath.Join(e.cfg.DataDir, "logs") }
func (e *Executor) credentialsDir() string {
	return filepath.Join(e.cfg.DataDir, "credentials")
}

func (e *Executor) baseArgs() []string {
	args := []string{
		"--non-interactive", "--agree-tos",
		"--config-dir", e.configDir(),
		"--work-dir", e.workDir(),
		"--logs-dir", e.logsDir(),
	}
	if e.cfg.Email != "" {
		args = append(args, "-m", e.cfg.Email)
	} else {
		args = append(args, "--register-unsafely-without-email")
	}
	if e.cfg.Staging {
		args = append(args, "--staging")
	}
	if e.cfg.DryRun {
		args = append(args, "--dry-run")
	}
	return args
}

// route53Credentials is the JSON shape expected in DecryptedCredential when
// DNSProvider is route53; the route53 certbot plugin authenticates via the
// standard AWS SDK credential chain, so these become env vars.
type route53Credentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

// challengeArgs returns the extra CLI args needed for the given challenge
// config, plus any env vars to add to the subprocess environment. For DNS-01
// it materializes a credentials file at a stable per-domain path so a later
// `certbot renew` (which does not re-supply credentials) can still find it.
func (e *Executor) challengeArgs(domainName string, challenge domain.ChallengeType, provider domain.DNSProvider, decryptedCredential []byte) (args []string, env []string, err error) {
	switch challenge {
	case domain.ChallengeHTTP01:
		return []string{"--webroot", "-w", e.cfg.Webroot}, nil, nil

	case domain.ChallengeDNS01:
		switch provider {
		case domain.DNSProviderCloudflare:
			path, err := e.writeCredentialsFile(domainName, decryptedCredential)
			if err != nil {
				return nil, nil, err
			}
			return []string{
				"--dns-cloudflare",
				"--dns-cloudflare-credentials", path,
				"--dns-cloudflare-propagation-seconds", "30",
			}, nil, nil

		case domain.DNSProviderRoute53:
			var creds route53Credentials
			if err := json.Unmarshal(decryptedCredential, &creds); err != nil {
				return nil, nil, fmt.Errorf("parse route53 credentials: %w", err)
			}
			env = []string{
				"AWS_ACCESS_KEY_ID=" + creds.AccessKeyID,
				"AWS_SECRET_ACCESS_KEY=" + creds.SecretAccessKey,
			}
			return []string{"--dns-route53"}, env, nil

		default:
			return nil, nil, fmt.Errorf("unsupported dns provider %q", provider)
		}

	default:
		return nil, nil, fmt.Errorf("unsupported challenge type %q", challenge)
	}
}

// writeCredentialsFile persists the (decrypted) cloudflare ini credentials at
// a stable path so that future `certbot renew` calls, which don't re-supply
// credentials, can still locate the file certbot recorded at issue time.
func (e *Executor) writeCredentialsFile(domainName string, contents []byte) (string, error) {
	dir := e.credentialsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create credentials dir: %w", err)
	}
	path := filepath.Join(dir, sanitizeFilename(domainName)+".ini")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return "", fmt.Errorf("write credentials file: %w", err)
	}
	return path, nil
}

func sanitizeFilename(name string) string {
	return strings.NewReplacer("*", "_wildcard_", "/", "_").Replace(name)
}

func (e *Executor) Issue(ctx context.Context, req domain.IssueRequest) (domain.CertResult, error) {
	args := append(e.baseArgs(), "certonly", "-d", req.DomainName)

	challengeArgs, env, err := e.challengeArgs(req.DomainName, req.ChallengeType, req.DNSProvider, req.DecryptedCredential)
	if err != nil {
		return domain.CertResult{}, err
	}
	args = append(args, challengeArgs...)

	if err := e.run(ctx, args, env); err != nil {
		return domain.CertResult{}, err
	}
	return e.loadCertResult(req.DomainName)
}

func (e *Executor) Renew(ctx context.Context, req domain.RenewRequest) (domain.CertResult, error) {
	// Re-materialize the DNS credentials file at the same stable path used
	// during Issue, in case it was cleaned up (e.g. container restart with
	// an ephemeral data dir) before certbot needs to read it again.
	if req.ChallengeType == domain.ChallengeDNS01 {
		if _, _, err := e.challengeArgs(req.DomainName, req.ChallengeType, req.DNSProvider, req.DecryptedCredential); err != nil {
			return domain.CertResult{}, err
		}
	}

	args := append(e.baseArgs(), "renew", "--cert-name", req.DomainName)

	var env []string
	if req.ChallengeType == domain.ChallengeDNS01 && req.DNSProvider == domain.DNSProviderRoute53 {
		var creds route53Credentials
		if err := json.Unmarshal(req.DecryptedCredential, &creds); err != nil {
			return domain.CertResult{}, fmt.Errorf("parse route53 credentials: %w", err)
		}
		env = []string{
			"AWS_ACCESS_KEY_ID=" + creds.AccessKeyID,
			"AWS_SECRET_ACCESS_KEY=" + creds.SecretAccessKey,
		}
	}

	if err := e.run(ctx, args, env); err != nil {
		return domain.CertResult{}, err
	}
	return e.loadCertResult(req.DomainName)
}

func (e *Executor) run(ctx context.Context, args []string, extraEnv []string) error {
	cmd := exec.CommandContext(ctx, e.cfg.Binary, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certbot %s failed: %w\noutput: %s", strings.Join(args, " "), err, out)
	}
	return nil
}

// loadCertResult reads the PEM files certbot wrote to its live directory. In
// --dry-run mode certbot never persists certs to disk, so callers get a zero
// CertResult with no error, signaling "the dry run validated successfully but
// there is nothing to store".
func (e *Executor) loadCertResult(domainName string) (domain.CertResult, error) {
	if e.cfg.DryRun {
		return domain.CertResult{}, nil
	}

	liveDir := filepath.Join(e.configDir(), "live", domainName)
	certPEM, err := os.ReadFile(filepath.Join(liveDir, "cert.pem"))
	if err != nil {
		return domain.CertResult{}, fmt.Errorf("read cert.pem: %w", err)
	}
	chainPEM, err := os.ReadFile(filepath.Join(liveDir, "chain.pem"))
	if err != nil {
		return domain.CertResult{}, fmt.Errorf("read chain.pem: %w", err)
	}
	keyPEM, err := os.ReadFile(filepath.Join(liveDir, "privkey.pem"))
	if err != nil {
		return domain.CertResult{}, fmt.Errorf("read privkey.pem: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return domain.CertResult{}, fmt.Errorf("decode cert.pem: no PEM block found")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return domain.CertResult{}, fmt.Errorf("parse certificate: %w", err)
	}

	return domain.CertResult{
		CertPEM:   certPEM,
		ChainPEM:  chainPEM,
		KeyPEM:    keyPEM,
		ExpiresAt: cert.NotAfter,
	}, nil
}
