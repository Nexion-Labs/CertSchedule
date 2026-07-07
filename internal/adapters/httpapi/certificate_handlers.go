package httpapi

import (
	"fmt"
	"net/http"

	"certschedule/internal/domain"
)

func (s *Server) handleIssue(w http.ResponseWriter, r *http.Request) {
	job, err := s.certSvc.Issue(r.Context(), r.PathValue("id"), domain.TriggerManual)
	// A cert issuance failure (bad DNS credential, certbot error, etc.) still
	// creates and persists a job record - that's a normal outcome to report
	// back, not an HTTP-level failure. Only a nil job (domain not found, DB
	// error before/during job creation) is a real HTTP error.
	if job == nil {
		respondError(w, s.logger, err)
		return
	}
	writeJSON(w, http.StatusAccepted, newJobResponse(job))
}

func (s *Server) handleRenew(w http.ResponseWriter, r *http.Request) {
	job, err := s.certSvc.Renew(r.Context(), r.PathValue("id"), domain.TriggerManual)
	if job == nil {
		respondError(w, s.logger, err)
		return
	}
	writeJSON(w, http.StatusAccepted, newJobResponse(job))
}

func (s *Server) handleListCertificates(w http.ResponseWriter, r *http.Request) {
	certs, err := s.certSvc.CertificateHistory(r.Context(), r.PathValue("id"))
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	out := make([]certificateResponse, 0, len(certs))
	for _, c := range certs {
		out = append(out, newCertificateResponse(c))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetCertificate(w http.ResponseWriter, r *http.Request) {
	detail, err := s.certSvc.GetCertificateDetail(r.Context(), r.PathValue("id"), r.PathValue("certID"))
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, newCertificateDetailResponse(detail))
}

func (s *Server) handleDownloadCertificate(w http.ResponseWriter, r *http.Request) {
	fullChain, err := s.certSvc.DownloadFullChain(r.Context(), r.PathValue("id"), r.PathValue("certID"))
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "fullchain.pem"))
	w.Write(fullChain)
}

func (s *Server) handleDownloadCertificateKey(w http.ResponseWriter, r *http.Request) {
	keyPEM, err := s.certSvc.DownloadPrivateKey(r.Context(), r.PathValue("id"), r.PathValue("certID"))
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "privkey.pem"))
	w.Write(keyPEM)
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.certSvc.JobHistory(r.Context(), r.PathValue("id"))
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	out := make([]jobResponse, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, newJobResponse(j))
	}
	writeJSON(w, http.StatusOK, out)
}
