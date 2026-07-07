package certbot

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WriteFullArchive tars+gzips the entire certbot data directory (config,
// credentials, work, logs) to w - everything certbot has generated on disk,
// for migration/disaster-recovery. This is separate from and complements the
// app's own encrypted-at-rest Certificate rows, which don't capture
// certbot-specific renewal metadata (ACME account linkage, authenticator,
// key type) or the DNS credential files certbot needs on disk to renew.
func (e *Executor) WriteFullArchive(ctx context.Context, w io.Writer) error {
	if _, err := os.Stat(e.cfg.DataDir); err != nil {
		return fmt.Errorf("certbot data directory: %w", err)
	}

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	if err := addDirToTar(ctx, tw, e.cfg.DataDir); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}
	return gz.Close()
}

// addDirToTar walks baseDir and writes every entry into tw with paths
// relative to baseDir, preserving symlinks as symlinks (certbot's live/
// directory is entirely symlinks into archive/, and must round-trip that way
// for a restored data dir to work with `certbot renew`).
func addDirToTar(ctx context.Context, tw *tar.Writer, baseDir string) error {
	return filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", path, err)
			}
			return tw.WriteHeader(&tar.Header{
				Name:     rel,
				Linkname: target,
				Typeflag: tar.TypeSymlink,
				Mode:     0o777,
			})
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("build tar header for %s: %w", path, err)
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write tar header for %s: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		defer f.Close()
		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("write tar body for %s: %w", path, err)
		}
		return nil
	})
}
