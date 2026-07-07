package httpapi

import (
	"bytes"
	"net/http"
	"strconv"
)

// handleDownloadCertbotArchive streams a tar.gz of certbot's entire on-disk
// data directory. Buffered in memory first (certbot data is small - PEM/conf
// files, not large blobs) so a mid-archive failure can still be reported as
// a normal JSON error instead of a truncated download.
func (s *Server) handleDownloadCertbotArchive(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	if err := s.certbotArchiveSvc.WriteFullArchive(r.Context(), &buf); err != nil {
		respondError(w, s.logger, err)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="certbot-data.tar.gz"`)
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}
