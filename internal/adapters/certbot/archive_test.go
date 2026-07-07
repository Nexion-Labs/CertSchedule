package certbot

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestExecutor_WriteFullArchive_PreservesSymlinksAndContent(t *testing.T) {
	dataDir := t.TempDir()

	mustMkdir(t, filepath.Join(dataDir, "config", "archive", "example.com"))
	mustMkdir(t, filepath.Join(dataDir, "config", "live", "example.com"))
	mustWrite(t, filepath.Join(dataDir, "config", "archive", "example.com", "fullchain1.pem"), "fullchain-bytes")
	mustWrite(t, filepath.Join(dataDir, "config", "renewal", "example.com.conf"), "ini-bytes")
	if err := os.Symlink("../../archive/example.com/fullchain1.pem", filepath.Join(dataDir, "config", "live", "example.com", "fullchain.pem")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	mustWrite(t, filepath.Join(dataDir, "credentials", "example.com.ini"), "dns_cloudflare_api_token = secret")

	e := New(Config{DataDir: dataDir})

	var buf bytes.Buffer
	if err := e.WriteFullArchive(context.Background(), &buf); err != nil {
		t.Fatalf("WriteFullArchive: %v", err)
	}

	gz, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	tr := tar.NewReader(gz)

	found := map[string]*tar.Header{}
	contents := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		found[hdr.Name] = hdr
		if hdr.Typeflag == tar.TypeReg {
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read tar body for %s: %v", hdr.Name, err)
			}
			contents[hdr.Name] = string(b)
		}
	}

	linkHdr, ok := found[filepath.Join("config", "live", "example.com", "fullchain.pem")]
	if !ok {
		t.Fatalf("expected live symlink entry in archive, got entries: %v", keys(found))
	}
	if linkHdr.Typeflag != tar.TypeSymlink {
		t.Errorf("expected symlink typeflag, got %v", linkHdr.Typeflag)
	}
	if linkHdr.Linkname != "../../archive/example.com/fullchain1.pem" {
		t.Errorf("unexpected symlink target: %q", linkHdr.Linkname)
	}

	if got := contents[filepath.Join("config", "archive", "example.com", "fullchain1.pem")]; got != "fullchain-bytes" {
		t.Errorf("expected archive file content %q, got %q", "fullchain-bytes", got)
	}
	if got := contents[filepath.Join("config", "renewal", "example.com.conf")]; got != "ini-bytes" {
		t.Errorf("expected renewal conf content %q, got %q", "ini-bytes", got)
	}
	if got := contents[filepath.Join("credentials", "example.com.ini")]; got != "dns_cloudflare_api_token = secret" {
		t.Errorf("expected credentials content preserved, got %q", got)
	}
}

func TestExecutor_WriteFullArchive_MissingDataDirReturnsError(t *testing.T) {
	e := New(Config{DataDir: filepath.Join(t.TempDir(), "does-not-exist")})
	var buf bytes.Buffer
	if err := e.WriteFullArchive(context.Background(), &buf); err == nil {
		t.Fatal("expected error for missing data directory, got nil")
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func keys(m map[string]*tar.Header) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
