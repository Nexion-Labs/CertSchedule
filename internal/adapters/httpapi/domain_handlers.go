package httpapi

import (
	"net/http"
	"time"

	"certschedule/internal/application"
)

func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := s.domainSvc.List(r.Context())
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	out := make([]domainResponse, 0, len(domains))
	for _, d := range domains {
		out = append(out, newDomainResponse(d))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	var req domainRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	d, err := s.domainSvc.Create(r.Context(), req.toCreateInput())
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, newDomainResponse(d))
}

func (s *Server) handleGetDomain(w http.ResponseWriter, r *http.Request) {
	d, err := s.domainSvc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, newDomainResponse(d))
}

func (s *Server) handleUpdateDomain(w http.ResponseWriter, r *http.Request) {
	var req domainRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	d, err := s.domainSvc.Update(r.Context(), r.PathValue("id"), req.toUpdateInput())
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, newDomainResponse(d))
}

func (s *Server) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	if err := s.domainSvc.Delete(r.Context(), r.PathValue("id")); err != nil {
		respondError(w, s.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExportDomains(w http.ResponseWriter, r *http.Request) {
	includeCredentials := r.URL.Query().Get("include_credentials") == "true"

	configs, err := s.domainSvc.Export(r.Context(), includeCredentials)
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, newDomainExportResponse(configs, time.Now().UTC()))
}

func (s *Server) handleImportDomains(w http.ResponseWriter, r *http.Request) {
	var req domainImportRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	configs := make([]application.DomainConfig, 0, len(req.Domains))
	for _, item := range req.Domains {
		configs = append(configs, item.toDomainConfig())
	}

	results, err := s.domainSvc.Import(r.Context(), configs)
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, newDomainImportResponse(results))
}
