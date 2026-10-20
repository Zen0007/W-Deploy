package service

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxZipFiles        = 20000
	maxZipUncompressed = 1 << 30 // 1 GiB
)

// UploadService menangani source code yang diunggah sebagai .zip.
type UploadService struct{}

func NewUploadService() *UploadService { return &UploadService{} }

// ExtractZip mengekstrak zipPath ke dest (dibersihkan dulu) dengan proteksi
// zip-slip, symlink, dan zip-bomb. Jika zip hanya berisi satu folder root,
// isinya otomatis di-"naikkan" ke dest.
func (s *UploadService) ExtractZip(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("zip tidak valid: %w", err)
	}
	defer zr.Close()

	if len(zr.File) == 0 {
		return errors.New("zip kosong")
	}
	if len(zr.File) > maxZipFiles {
		return fmt.Errorf("zip berisi terlalu banyak file (>%d)", maxZipFiles)
	}

	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	prefix := commonRoot(zr.File)
	var total int64

	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, prefix)
		if name == "" || strings.HasPrefix(name, "__MACOSX/") {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			continue // abaikan symlink demi keamanan
		}

		target := filepath.Join(destAbs, filepath.FromSlash(name))
		if target != destAbs && !strings.HasPrefix(target, destAbs+string(os.PathSeparator)) {
			return fmt.Errorf("path berbahaya di dalam zip: %q", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		total += int64(f.UncompressedSize64)
		if total > maxZipUncompressed {
			return errors.New("ukuran hasil ekstrak melebihi batas 1 GiB")
		}
		if err := writeZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	mode := f.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	// Batasi byte aktual (header ukuran bisa dipalsukan).
	n, err := io.Copy(out, io.LimitReader(rc, int64(f.UncompressedSize64)+1))
	if err != nil {
		return err
	}
	if uint64(n) > f.UncompressedSize64 {
		return fmt.Errorf("ukuran file melebihi header: %s", f.Name)
	}
	return nil
}

// commonRoot mengembalikan "folder/" jika SEMUA entri berada di satu folder root.
func commonRoot(files []*zip.File) string {
	root := ""
	for _, f := range files {
		name := f.Name
		if strings.HasPrefix(name, "__MACOSX/") {
			continue
		}
		i := strings.Index(name, "/")
		if i < 0 {
			return "" // ada file di level atas
		}
		r := name[:i+1]
		if root == "" {
			root = r
		} else if root != r {
			return ""
		}
	}
	return root
}
