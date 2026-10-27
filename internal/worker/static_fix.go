package worker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// prepareStaticProject memeriksa apakah source code adalah web statis frontend murni tanpa server.
// Jika tidak ada package.json atau server backend tapi ada file HTML (misal blog.html),
// fungsi ini otomatis:
// 1. Memastikan file index.html ada (copy dari file HTML pertama yang ditemukan jika belum ada index.html).
// 2. Menghasilkan package.json yang menjalankan static web server (serve) pada port yang diminta.
func prepareStaticProject(dir string, port int, logf func(string, ...any)) {
	pkgPath := filepath.Join(dir, "package.json")
	if _, err := os.Stat(pkgPath); err == nil {
		return // Sudah ada package.json
	}

	// Cek apakah ada file server backend lain
	backendIndicators := []string{"go.mod", "requirements.txt", "pom.xml", "build.gradle", "Gemfile", "composer.json", "Cargo.toml"}
	for _, f := range backendIndicators {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return // Ada file project backend
		}
	}

	// Cari file HTML di dalam direktori
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var htmlFiles []string
	hasIndexHTML := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".html") {
			htmlFiles = append(htmlFiles, name)
			if strings.EqualFold(name, "index.html") {
				hasIndexHTML = true
			}
		}
	}

	// Jika tidak ada file HTML, jangan ubah
	if len(htmlFiles) == 0 {
		return
	}

	// 1. Jika belum ada index.html, salin file HTML pertama (misal blog.html) menjadi index.html
	if !hasIndexHTML {
		firstHTML := htmlFiles[0]
		srcPath := filepath.Join(dir, firstHTML)
		dstPath := filepath.Join(dir, "index.html")
		if data, err := os.ReadFile(srcPath); err == nil {
			if err := os.WriteFile(dstPath, data, 0o644); err == nil {
				logf("==> [auto-setup] menyalin %s ke index.html untuk web statis", firstHTML)
			}
		}
	}

	// 2. Auto-generate package.json dengan static server bawaan (serve)
	pkg := map[string]any{
		"name":    "static-web-app",
		"version": "1.0.0",
		"scripts": map[string]string{
			"start": fmt.Sprintf("npx serve -s . -l %d", port),
		},
		"dependencies": map[string]string{
			"serve": "^14.2.4",
		},
	}
	data, err := json.MarshalIndent(pkg, "", "  ")
	if err == nil {
		if err := os.WriteFile(pkgPath, data, 0o644); err == nil {
			logf("==> [auto-setup] mendeteksi web statis; otomatis men-setup web server (port %d)", port)
		}
	}
}
