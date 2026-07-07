// Command server is the composition root: it wires config, the SQLite
// database, all outbound adapters (certbot, k8s, crypto, jwt), the
// application services, the HTTP API, and the renewal scheduler together,
// then serves until an interrupt/terminate signal is received.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"certschedule/internal/adapters/certbot"
	"certschedule/internal/adapters/httpapi"
	k8sadapter "certschedule/internal/adapters/k8s"
	"certschedule/internal/adapters/scheduler"
	"certschedule/internal/adapters/sqlite"
	"certschedule/internal/application"
	"certschedule/internal/config"
	appcrypto "certschedule/internal/platform/crypto"
	"certschedule/internal/platform/jwtutil"
	"certschedule/internal/platform/logging"
	"certschedule/internal/webui"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load() // optional local .env; real deployments set env vars directly

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logging.New(os.Getenv("DEBUG") == "true")

	db, err := sqlite.Open(cfg.SQLitePath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	domainRepo := sqlite.NewDomainRepo(db)
	certificateRepo := sqlite.NewCertificateRepo(db)
	jobRepo := sqlite.NewJobRepo(db)
	userRepo := sqlite.NewUserRepo(db)

	encryptor, err := appcrypto.New(cfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("init encryptor: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := seedAdminUser(ctx, userRepo, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}

	certIssuer := certbot.New(certbot.Config{
		Binary:  cfg.CertbotBinary,
		DataDir: cfg.CertbotDataDir,
		Staging: cfg.CertbotStaging,
		DryRun:  cfg.CertbotDryRun,
		Webroot: cfg.CertbotWebroot,
		Email:   cfg.CertbotEmail,
	})

	secretUpdater := newSecretUpdater(cfg.KubeconfigPath, logger)

	jwtIssuer := jwtutil.NewIssuer(cfg.JWTSecret, cfg.JWTTTL)

	domainSvc := application.NewDomainService(domainRepo, encryptor)
	certSvc := application.NewCertificateService(domainRepo, certificateRepo, jobRepo, certIssuer, secretUpdater, encryptor, nil, logger)
	authSvc := application.NewAuthService(userRepo, jwtIssuer)
	certbotArchiveSvc := application.NewCertbotArchiveService(certIssuer)
	schedulerSvc := application.NewSchedulerService(domainRepo, certSvc, nil, logger)

	cronScheduler := scheduler.New(schedulerSvc, cfg.SchedulerInterval, logger)
	if err := cronScheduler.Start(ctx); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}
	defer cronScheduler.Stop()

	apiServer := httpapi.NewServer(domainSvc, certSvc, authSvc, certbotArchiveSvc, jwtIssuer, logger)
	handler := httpapi.WithSPA(apiServer.Routes(), webui.FS)

	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serveErr:
		return fmt.Errorf("http server error: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

func seedAdminUser(ctx context.Context, userRepo *sqlite.UserRepo, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	return userRepo.EnsureUser(ctx, username, string(hash))
}

// newSecretUpdater builds the k8s SecretUpdater, auto-detecting in-cluster
// vs kubeconfig access. If neither is available (e.g. plain local dev with
// no cluster configured), it falls back to a stub that fails loudly only
// when a cert issuance actually tries to push a secret, rather than
// preventing the whole app (domain CRUD, auth, UI) from starting.
func newSecretUpdater(kubeconfigPath string, logger *slog.Logger) *unavailableAwareSecretUpdater {
	client, err := k8sadapter.New(kubeconfigPath)
	if err != nil {
		logger.Warn("kubernetes access unavailable; certificate issuance will fail until a cluster is reachable", "error", err)
		return &unavailableAwareSecretUpdater{err: err}
	}
	return &unavailableAwareSecretUpdater{client: client}
}

type unavailableAwareSecretUpdater struct {
	client *k8sadapter.SecretClient
	err    error
}

func (u *unavailableAwareSecretUpdater) UpsertTLSSecret(ctx context.Context, namespace, name string, certPEM, keyPEM []byte) error {
	if u.client == nil {
		return fmt.Errorf("kubernetes client unavailable: %w", u.err)
	}
	return u.client.UpsertTLSSecret(ctx, namespace, name, certPEM, keyPEM)
}
