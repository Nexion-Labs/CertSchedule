// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the server.
type Config struct {
	// HTTP
	HTTPAddr string

	// Database
	SQLitePath string

	// Auth
	JWTSecret     string
	JWTTTL        time.Duration
	AdminUsername string
	AdminPassword string

	// Encryption key for stored DNS provider credentials (32 bytes, base64 or raw)
	EncryptionKey string

	// Certbot
	CertbotBinary   string
	CertbotDataDir  string
	CertbotStaging  bool
	CertbotDryRun   bool
	CertbotWebroot  string
	CertbotEmail    string

	// Scheduler
	SchedulerInterval time.Duration

	// Kubernetes
	KubeconfigPath string
}

// Load reads configuration from environment variables, applying sane defaults.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		SQLitePath:        getEnv("SQLITE_PATH", "./data/certschedule.db"),
		JWTSecret:         getEnv("JWT_SECRET", ""),
		JWTTTL:            getEnvDuration("JWT_TTL", 24*time.Hour),
		AdminUsername:     getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:     getEnv("ADMIN_PASSWORD", ""),
		EncryptionKey:     getEnv("ENCRYPTION_KEY", ""),
		CertbotBinary:     getEnv("CERTBOT_BINARY", "certbot"),
		CertbotDataDir:    getEnv("CERTBOT_DATA_DIR", "./data/certbot"),
		CertbotStaging:    getEnvBool("CERTBOT_STAGING", true),
		CertbotDryRun:     getEnvBool("CERTBOT_DRY_RUN", false),
		CertbotWebroot:    getEnv("CERTBOT_WEBROOT", "./data/webroot"),
		CertbotEmail:      getEnv("CERTBOT_EMAIL", ""),
		SchedulerInterval: getEnvDuration("SCHEDULER_INTERVAL", 12*time.Hour),
		KubeconfigPath:    getEnv("KUBECONFIG", ""),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required (32 bytes for AES-256)")
	}
	if cfg.AdminPassword == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
