package tests

// import (
// 	"archive/zip"
// 	"io"
// 	"os"
// 	"path/filepath"
// 	"strings"
// 	"testing"
// )

// func TestArchiveServerFolderCreatesZip(t *testing.T) {
// 	root := t.TempDir()
// 	source := filepath.Join(root, "my-app")
// 	if err := os.Mkdir(source, 0o755); err != nil {
// 		t.Fatal(err)
// 	}
// 	content := "package main\n"
// 	if err := os.WriteFile(filepath.Join(source, "main.go"), []byte(content), 0o600); err != nil {
// 		t.Fatal(err)
// 	}
// 	workspace := t.TempDir()

// 	archivePath, err := archiveServerFolder(root, source, workspace)
// 	if err != nil {
// 		t.Fatalf("archiveServerFolder() error = %v", err)
// 	}

// 	archive, err := zip.OpenReader(archivePath)
// 	if err != nil {
// 		t.Fatalf("open generated archive: %v", err)
// 	}
// 	defer archive.Close()
// 	if len(archive.File) != 1 || archive.File[0].Name != "main.go" {
// 		t.Fatalf("archive entries = %#v, want [main.go]", archive.File)
// 	}
// 	file, err := archive.File[0].Open()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer file.Close()
// 	data, err := io.ReadAll(file)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if string(data) != content {
// 		t.Fatalf("archived content = %q, want %q", data, content)
// 	}
// }

// func TestArchiveServerFolderRejectsOutsidePathAndSymlinks(t *testing.T) {
// 	root := t.TempDir()
// 	source := filepath.Join(root, "my-app")
// 	if err := os.Mkdir(source, 0o755); err != nil {
// 		t.Fatal(err)
// 	}
// 	outside := t.TempDir()
// 	if _, err := archiveServerFolder(root, outside, t.TempDir()); err == nil ||
// 		!strings.Contains(err.Error(), "di dalam") {
// 		t.Fatalf("outside path error = %v, want path-boundary error", err)
// 	}

// 	if err := os.Symlink(outside, filepath.Join(source, "linked")); err != nil {
// 		t.Skipf("cannot create symlink for test: %v", err)
// 	}
// 	if _, err := archiveServerFolder(root, source, t.TempDir()); err == nil ||
// 		!strings.Contains(err.Error(), "symbolic link") {
// 		t.Fatalf("symlink error = %v, want symlink rejection", err)
// 	}
// }
