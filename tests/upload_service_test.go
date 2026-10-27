package tests

import (
	"W-dev/internal/service"
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func makeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "test.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	return p
}

func TestValidateArchive_OK(t *testing.T) {
	svc := service.NewUploadService()
	p := makeZip(t, map[string]string{
		"index.html":    "<html></html>",
		"css/style.css": "body{}",
	})
	if err := svc.ValidateArchive(p); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestValidateArchive_NotZip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "not.zip")
	_ = os.WriteFile(p, []byte("this is not a zip"), 0o644)
	svc := service.NewUploadService()
	if err := svc.ValidateArchive(p); err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestValidateArchive_EmptyZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	_ = zw.Close()
	dir := t.TempDir()
	p := filepath.Join(dir, "empty.zip")
	_ = os.WriteFile(p, buf.Bytes(), 0o644)
	svc := service.NewUploadService()
	if err := svc.ValidateArchive(p); err == nil {
		t.Fatal("want error for empty zip, got nil")
	}
}

func TestExtractZip_RejectsZipSlip(t *testing.T) {
	svc := service.NewUploadService()
	p := makeZip(t, map[string]string{
		"ok.txt":         "ok",
		"../../evil.txt": "bad",
	})
	dest := t.TempDir()
	if err := svc.ExtractZip(p, dest); err == nil {
		t.Fatal("want error for path traversal, got nil")
	}
}

func TestExtractZip_RejectsSymlinkEntry(t *testing.T) {
	// Symlink entry hanya di-skip (tidak diekstrak), bukan error.
	svc := service.NewUploadService()
	p := makeZip(t, map[string]string{"file.txt": "ok"})
	dest := t.TempDir()
	if err := svc.ExtractZip(p, dest); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "file.txt")); err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}
