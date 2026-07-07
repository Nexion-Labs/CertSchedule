// Package httpapi is the inbound HTTP adapter: it exposes the application
// services over a JSON REST API using the stdlib net/http (Go 1.22+ pattern
// based ServeMux, so no external router dependency is needed).
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"certschedule/internal/application"
	"certschedule/internal/domain"
	"certschedule/internal/platform/jwtutil"
)

// Server wires application services to HTTP handlers.
type Server struct {
	domainSvc         *application.DomainService
	certSvc           *application.CertificateService
	authSvc           *application.AuthService
	certbotArchiveSvc *application.CertbotArchiveService
	jwtIssuer         *jwtutil.Issuer
	logger            *slog.Logger
}

func NewServer(
	domainSvc *application.DomainService,
	certSvc *application.CertificateService,
	authSvc *application.AuthService,
	certbotArchiveSvc *application.CertbotArchiveService,
	jwtIssuer *jwtutil.Issuer,
	logger *slog.Logger,
) *Server {
	return &Server{
		domainSvc: domainSvc, certSvc: certSvc, authSvc: authSvc,
		certbotArchiveSvc: certbotArchiveSvc, jwtIssuer: jwtIssuer, logger: logger,
	}
}

// Routes builds the full API mux (unauthenticated + authenticated routes),
// wrapped with logging/recover/CORS middleware. Mount it under "/api/v1/" or
// serve it directly; SPA static-file serving is layered on top in spa.go.
func (s *Server) Routes() http.Handler {
	api := http.NewServeMux()

	api.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	api.HandleFunc("GET /healthz", s.handleHealth)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/domains", s.handleListDomains)
	protected.HandleFunc("POST /api/v1/domains", s.handleCreateDomain)
	protected.HandleFunc("GET /api/v1/domains/export", s.handleExportDomains)
	protected.HandleFunc("POST /api/v1/domains/import", s.handleImportDomains)
	protected.HandleFunc("GET /api/v1/domains/{id}", s.handleGetDomain)
	protected.HandleFunc("PUT /api/v1/domains/{id}", s.handleUpdateDomain)
	protected.HandleFunc("DELETE /api/v1/domains/{id}", s.handleDeleteDomain)
	protected.HandleFunc("POST /api/v1/domains/{id}/issue", s.handleIssue)
	protected.HandleFunc("POST /api/v1/domains/{id}/renew", s.handleRenew)
	protected.HandleFunc("GET /api/v1/domains/{id}/certificates", s.handleListCertificates)
	protected.HandleFunc("GET /api/v1/domains/{id}/certificates/{certID}", s.handleGetCertificate)
	protected.HandleFunc("GET /api/v1/domains/{id}/certificates/{certID}/download", s.handleDownloadCertificate)
	protected.HandleFunc("GET /api/v1/domains/{id}/certificates/{certID}/download-key", s.handleDownloadCertificateKey)
	protected.HandleFunc("GET /api/v1/domains/{id}/jobs", s.handleListJobs)
	protected.HandleFunc("GET /api/v1/certbot/archive", s.handleDownloadCertbotArchive)

	api.Handle("/api/v1/", s.authMiddleware(protected))

	return s.recoverMiddleware(s.loggingMiddleware(s.corsMiddleware(api)))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

// respondError maps domain sentinel errors to the appropriate HTTP status.
func respondError(w http.ResponseWriter, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInvalidCredential):
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrAlreadyExists):
		writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
	default:
		logger.Error("internal server error", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
