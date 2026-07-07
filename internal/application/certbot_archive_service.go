package application

import (
	"context"
	"io"

	"certschedule/internal/domain"
)

// CertbotArchiveService streams a full backup of certbot's on-disk state -
// everything certbot itself has generated (config, credentials, work, logs)
// across every domain - distinct from the app's own encrypted-at-rest
// Domain/Certificate database records. This is a system-level, not
// domain-scoped, backup: treat the resulting archive as a plaintext secrets
// bundle, since certbot stores private keys and DNS credentials unencrypted
// on disk.
type CertbotArchiveService struct {
	archiver domain.CertbotArchiver
}

func NewCertbotArchiveService(archiver domain.CertbotArchiver) *CertbotArchiveService {
	return &CertbotArchiveService{archiver: archiver}
}

func (s *CertbotArchiveService) WriteFullArchive(ctx context.Context, w io.Writer) error {
	return s.archiver.WriteFullArchive(ctx, w)
}
