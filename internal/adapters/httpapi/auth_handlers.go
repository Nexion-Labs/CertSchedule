package httpapi

import "net/http"

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "username and password are required"})
		return
	}

	token, err := s.authSvc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		respondError(w, s.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{Token: token})
}
