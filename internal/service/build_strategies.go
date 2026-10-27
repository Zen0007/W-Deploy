package service

import (
	"W-dev/internal/projectconfig"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════
// STATIC — nginx serves the source as-is, no buildpack.
// ═══════════════════════════════════════════════════════════════
type StaticStrategy struct{}

func (s *StaticStrategy) Name() string { return "static" }

func (s *StaticStrategy) Build(ctx context.Context, in BuildInput) (*BuildPlanResult, error) {
	start := time.Now()

	if !hasAnyHTML(in.SourcePath) {
		return nil, errors.New("static strategry :tidak ada file .html di source")
	}

	nginxCfg := renderStaticNginx(in.Port)
	dockerfile := renderStaticDockerfile(in.Port)

	tmpDir := filepath.Join(in.SourcePath, ".paas-build")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, err
	}

	defer os.RemoveAll(tmpDir)

	dfPath := filepath.Join(tmpDir, "Dockerffile")
	cfPath := filepath.Join(tmpDir, "nginx.conf")
	_ = os.WriteFile(dfPath, []byte(dockerfile), 0o644)
	_ = os.WriteFile(cfPath, []byte(nginxCfg), 0o644)

	// Dockerfile menggunakan source langsung - .paas-build di-ignore via .dockerignore
	_ = os.WriteFile(filepath.Join(in.SourcePath, ".dockerignore"), []byte(".paas-build\n"), 0o644)

	if err := dockerBuild(ctx, in, dfPath, in.LogChannel); err != nil {
		return &BuildPlanResult{
			ImageName: in.ImageName,
			Success:   false,
			Duration:  time.Since(start),
			Strategy:  s.Name(),
			Error:     err.Error(),
		}, err
	}

	return &BuildPlanResult{
		ImageName:   in.ImageName,
		RuntimePort: in.Port,
		Success:     true,
		Duration:    time.Since(start),
		Strategy:    s.Name(),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// SPA — build Node di stage pertama, Nginx di stage kedua.
// ═══════════════════════════════════════════════════════════════
type SPAStrategy struct{}

func (s *SPAStrategy) Name() string { return "spa" }

func (s *SPAStrategy) Build(ctx context.Context, in BuildInput) (*BuildPlanResult, error) {
	start := time.Now()

	cfg, err := ParseFrontendProject(in.SourcePath, in.Config)
	if err != nil {
		return nil, fmt.Errorf("spa strategy: %w", err)
	}

	envLines := buildEnvLines(in.BuildEnv)
	dockerfile := renderSPADockerfile(cfg, in.Port, envLines)
	nginxConf := renderStaticNginx(in.Port)

	tmpDir := filepath.Join(in.SourcePath, ".paas-build")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	dfPath := filepath.Join(tmpDir, "Dockerfile")
	cfPath := filepath.Join(tmpDir, "nginx.conf")
	_ = os.WriteFile(dfPath, []byte(dockerfile), 0o644)
	_ = os.WriteFile(cfPath, []byte(nginxConf), 0o644)
	_ = os.WriteFile(filepath.Join(in.SourcePath, ".dockerignore"), []byte(
		".paas-build\nnode_modules\n.git\n"), 0o644)

	if err := dockerBuild(ctx, in, dfPath, in.LogChannel); err != nil {
		return &BuildPlanResult{
			ImageName: in.ImageName,
			Success:   false,
			Duration:  time.Since(start),
			Strategy:  s.Name(),
			Error:     err.Error(),
		}, err
	}

	return &BuildPlanResult{
		ImageName:   in.ImageName,
		RuntimePort: in.Port,
		Success:     true,
		Duration:    time.Since(start),
		Strategy:    s.Name(),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// NODE — use existing pack build flow.
// ═══════════════════════════════════════════════════════════════
type NodeStrategy struct {
	build *BuildService // untuk memakai pack build yang sudah ada
}

func (s *NodeStrategy) Name() string { return "node" }

func (s *NodeStrategy) Build(ctx context.Context, in BuildInput) (*BuildPlanResult, error) {
	// Delegasi ke build_service.Build
	res, err := s.build.Build(ctx, BuildRequest{
		AppSlug:    in.AppSlug,
		SourcePath: in.SourcePath,
		ImageName:  in.ImageName,
		Env:        in.BuildEnv,
		LogChannel: in.LogChannel,
	})
	if err != nil {
		return &BuildPlanResult{
			ImageName:   in.ImageName,
			RuntimePort: in.Port,
			Success:     false,
			Duration:    res.Duration,
			Strategy:    s.Name(),
			Error:       err.Error(),
		}, err
	}

	return &BuildPlanResult{
		ImageName:   in.ImageName,
		RuntimePort: in.Port,
		Success:     true,
		Duration:    res.Duration,
		Strategy:    s.Name(),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// CUSTOM — pakai Dockerfile dari user jika ada; jika tidak, fallback ke node.
// ═══════════════════════════════════════════════════════════════
type CustomStrategy struct {
	node *NodeStrategy
}

func NewCustomStrategy(node *NodeStrategy) *CustomStrategy { return &CustomStrategy{node: node} }

func (s *CustomStrategy) Name() string { return "custom" }

func (s *CustomStrategy) Build(ctx context.Context, in BuildInput) (*BuildPlanResult, error) {
	start := time.Now()

	userDockerfile := filepath.Join(in.SourcePath, "Dockerfile")
	if _, err := os.Stat(userDockerfile); err != nil {
		// Validasi: Dockerfile harus di dalam source path
		abs, _ := filepath.Abs(userDockerfile)
		srcAbs, _ := filepath.Abs(in.SourcePath)
		if !strings.HasPrefix(abs, srcAbs) {
			return nil, errors.New("Dockerfile di luar source path (indikasi path traversal)")
		}

		if err := dockerBuild(ctx, in, userDockerfile, in.LogChannel); err != nil {
			return &BuildPlanResult{
				ImageName: in.ImageName,
				Success:   false,
				Duration:  time.Since(start),
				Strategy:  s.Name(),
				Error:     err.Error(),
			}, err
		}
		return &BuildPlanResult{
			ImageName:   in.ImageName,
			RuntimePort: in.Port,
			Success:     true,
			Duration:    time.Since(start),
			Strategy:    s.Name(),
		}, nil
	}
	// Tidak ada Dockerfile — fallback ke Node/Pack.
	if in.LogChannel != nil {
		in.LogChannel <- "custom: Dockerfile tidak ditemukan, fallback ke Buildpacks"
	}

	return s.node.Build(ctx, in)
}

// ═══════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════

func hasAnyHTML(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.html"))
	return len(matches) > 0
}

type frontendCfg struct {
	BuildCmd   string
	OutputDir  string
	LockFile   string // "package-lock.json" | "yarn.lock" | "pnpm-lock.yaml" | ""
	InstallCmd string
}

func ParseFrontendProject(src string, resolved *projectconfig.ResolveConfig) (*frontendCfg, error) {
	cfg := &frontendCfg{}

	// Prioritas: wdeploy.json build.command / build.output
	if resolved != nil && resolved.Build != nil {
		cfg.BuildCmd = resolved.Build.Command
		cfg.OutputDir = resolved.Build.Output
	}

	// Baca package.json untuk default + deteksi lock file
	pkgData, err := os.ReadFile(filepath.Join(src, "package.json"))
	if err != nil {
		if cfg.BuildCmd == "" {
			return nil, errors.New("tidak ada package.json dan wdeploy.json tidak mendefinisikan build.command")
		}
	} else {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		_ = json.Unmarshal(pkgData, &pkg)
		if cfg.BuildCmd == "" {
			if _, ok := pkg.Scripts["build"]; !ok {
				return nil, errors.New("package.json tidak memiliki script 'build'")
			}
			cfg.BuildCmd = "npm run build"
		}
	}

	// Lock file
	switch {
	case fileExists(filepath.Join(src, "pnpm-lock.yaml")):
		cfg.LockFile = "pnpm-lock.yaml"
		cfg.InstallCmd = "pnpm install --frozen-lockfile"
		cfg.BuildCmd = strings.Replace(cfg.BuildCmd, "npm run", "pnpm", 1)
	case fileExists(filepath.Join(src, "yarn.lock")):
		cfg.LockFile = "yarn.lock"
		cfg.InstallCmd = "yarn install --frozen-lockfile"
		cfg.BuildCmd = strings.Replace(cfg.BuildCmd, "npm run", "yarn", 1)
	case fileExists(filepath.Join(src, "package-lock.json")):
		cfg.LockFile = "package-lock.json"
		cfg.InstallCmd = "npm ci"
	default:
		cfg.InstallCmd = "npm install"
	}

	// Output dir
	if cfg.OutputDir == "" {
		for _, c := range []string{"dist", "build", "out", ".output/public"} {
			if dirExists(filepath.Join(src, c)) {
				cfg.OutputDir = c
				break
			}
		}
		if cfg.OutputDir == "" {
			cfg.OutputDir = "dist" // default paling umum
		}
	}

	return cfg, nil
}

func buildEnvLines(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	var b strings.Builder
	for k, v := range env {
		// Filter hanya build-time prefix yang aman
		if !strings.HasPrefix(k, "VITE_") && !strings.HasPrefix(k, "REACT_APP_") &&
			!strings.HasPrefix(k, "NEXT_PUBLIC_") && !strings.HasPrefix(k, "PUBLIC_") &&
			!strings.HasPrefix(k, "NUXT_PUBLIC_") {
			continue
		}
		b.WriteString("ENV ")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(EscapeEnvValue(v))
		b.WriteString("\n")
	}
	return b.String()
}

func EscapeEnvValue(v string) string {
	// Dockerfile ENV value: escape backslash & quotes
	v = strings.ReplaceAll(v, "\\", "\\\\")
	v = strings.ReplaceAll(v, "\"", "\\\"")
	return `"` + v + `"`
}

func renderStaticDockerfile(port int) string {
	return fmt.Sprintf(`FROM nginx:1.27-alpine

# Config
COPY .paas-build/nginx.conf /etc/nginx/conf.d/default.conf

# Source: seluruh folder (tanpa .paas-build karena .dockerignore)
WORKDIR /usr/share/nginx/html
COPY . .

# Pastikan ada index.html (auto-alias jika hanya ada .html lain)
RUN if [ ! -f index.html ]; then \
      first_html=$$(ls *.html 2>/dev/null | head -n1); \
      if [ -n "$$first_html" ]; then cp "$$first_html" index.html; fi; \
    fi

EXPOSE %d
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -q --spider http://localhost:%d/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
`, port, port)
}

func renderSPADockerfile(cfg *frontendCfg, port int, envLines string) string {
	lockCopy := ""
	if cfg.LockFile != "" {
		lockCopy = "COPY " + cfg.LockFile + " ./\n"
	}
	return fmt.Sprintf(`# ── Stage 1: build ──
FROM node:20-alpine AS builder
WORKDIR /app

COPY package*.json ./
%sRUN %s

COPY . .
%s
RUN %s

# ── Stage 2: serve ──
FROM nginx:1.27-alpine

COPY .paas-build/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/%s /usr/share/nginx/html

EXPOSE %d
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -q --spider http://localhost:%d/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
`, lockCopy, cfg.InstallCmd, envLines, cfg.BuildCmd, cfg.OutputDir, port, port)
}

func renderStaticNginx(port int) string {
	return fmt.Sprintf(`server {
    listen %d;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location ~* \.(js|css|png|jpg|jpeg|gif|svg|ico|woff2?|ttf)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    location = /index.html {
        expires -1;
        add_header Cache-Control "no-cache, no-store, must-revalidate";
    }

    gzip on;
    gzip_types text/plain text/css application/javascript application/json;
}
`, port)
}

func dockerBuild(ctx context.Context, in BuildInput, dockerfilePath string, logCh chan<- string) error {
	cmd := exec.CommandContext(ctx, "docker", "build",
		"-t", in.ImageName,
		"-f", dockerfilePath,
		in.SourcePath,
	)
	cmd.Env = append(os.Environ(), "DOCKER_HOST=unix:///var/run/docker.sock")

	if logCh != nil {
		w := NewLogWriter(logCh, 5*1024*1024)
		cmd.Stdout = w
		cmd.Stderr = w
	}
	return cmd.Run()
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
func dirExists(p string) bool  { fi, err := os.Stat(p); return err == nil && fi.IsDir() }
