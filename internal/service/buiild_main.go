package service

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// BuildRequest adalah input untuk build image.
type BuildRequest struct {
	AppSlug    string            `json:"app_slug"`
	SourcePath string            `json:"source_path"` // path ke source code
	ImageName  string            `json:"image_name"`  // contoh: paas/myapp:latest
	Builder    string            `json:"builder"`     // default: paketobuildpacks/builder-jammy-base
	Env        map[string]string `json:"env"`         // build-time env
	Buildpacks []string          `json:"buildpacks"`  // opsional
	ClearCache bool              `json:"clear_cache"`
	LogChannel chan<- string     `json:"-"` // opsional: untuk streaming log
}

// BuildMainResult dikembalikan setelah build selesai.
type BuildMainResult struct {
	ImageName string        `json:"image_name"`
	Success   bool          `json:"success"`
	Duration  time.Duration `json:"duration"`
	Log       string        `json:"log"`
	Error     string        `json:"error,omitempty"`
}

// BuildService membungkus eksekusi `pack build`.
type BuildService struct {
	defaultBuilder string
	packBin        string // path ke binary pack, default: "pack"
}

func NewBuildService(defaultBuilder string) *BuildService {
	if defaultBuilder == "" {
		defaultBuilder = "paketobuildpacks/builder-jammy-base:latest"
	}
	return &BuildService{
		defaultBuilder: defaultBuilder,
		packBin:        "pack",
	}
}

// Build mengeksekusi `pack build` dan menangkap output secara real-time.
func (s *BuildService) Build(ctx context.Context, req BuildRequest) (*BuildMainResult, error) {
	start := time.Now()

	// 1. Validasi source path
	absSource, err := filepath.Abs(req.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("invalid source path: %w", err)
	}
	if fi, err := os.Stat(absSource); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("source path tidak valid: %s", absSource)
	}

	// 2. Tentukan builder
	builder := req.Builder
	if builder == "" {
		builder = s.defaultBuilder
	}

	// 3. Bangun argumen
	args := []string{
		"build", req.ImageName,
		"--path", absSource,
		"--builder", builder,
		"--creation-time", "now",
		"--verbose", // verbose agar log lebih lengkap
	}

	if req.ClearCache {
		args = append(args, "--clear-cache")
	}

	// Build-time env
	for k, v := range req.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}

	// Custom buildpacks
	for _, bp := range req.Buildpacks {
		args = append(args, "--buildpack", bp)
	}

	log.Printf("[build] menjalankan: %s %s", s.packBin, strings.Join(args, " "))

	// 4. Siapkan command
	cmd := exec.CommandContext(ctx, s.packBin, args...)
	cmd.Dir = absSource
	cmd.Env = append(os.Environ(),
		"DOCKER_HOST=unix:///var/run/docker.sock",
		"PACK_HOME="+filepath.Join(os.TempDir(), "pack"),
	)

	// 5. Setup custom writer untuk stdout & stderr
	logWriter := NewLogWriter(req.LogChannel, 5*1024*1024) // max 5 MB
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter

	// 6. Jalankan
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("gagal start pack build: %w", err)
	}

	// 7. Tunggu selesai
	waitErr := cmd.Wait()
	duration := time.Since(start)

	logStr := logWriter.String()

	result := &BuildMainResult{
		ImageName: req.ImageName,
		Duration:  duration,
		Log:       logStr,
	}

	if waitErr != nil {
		result.Success = false
		result.Error = waitErr.Error()
		log.Printf("[build] gagal setelah %s: %v", duration, waitErr)
		return result, fmt.Errorf("pack build gagal: %w", waitErr)
	}

	result.Success = true
	log.Printf("[build] sukses: %s (%s)", req.ImageName, duration)
	return result, nil
}

// BuildWithStream menjalankan build dan mengirim setiap baris log ke channel.
// Berguna untuk SSE / WebSocket handler.
func (s *BuildService) BuildWithStream(
	ctx context.Context, req BuildRequest,
) (<-chan string, <-chan *BuildMainResult, error) {
	logCh := make(chan string, 100)
	resultCh := make(chan *BuildMainResult, 1)
	req.LogChannel = logCh

	go func() {
		defer close(logCh)
		defer close(resultCh)

		result, err := s.Build(ctx, req)
		if err != nil && result == nil {
			result = &BuildMainResult{
				ImageName: req.ImageName,
				Success:   false,
				Error:     err.Error(),
			}
		}
		resultCh <- result
	}()

	return logCh, resultCh, nil
}

// StreamLogsToWriter menjalankan build dan menyalin log ke io.Writer (misal HTTP response).
func (s *BuildService) StreamLogsToWriter(
	ctx context.Context, req BuildRequest, w interface{ Write([]byte) (int, error) },
) error {
	cmd := exec.CommandContext(ctx, s.packBin, s.buildArgs(req)...)
	cmd.Dir = req.SourcePath
	cmd.Env = append(os.Environ(), "DOCKER_HOST=unix:///var/run/docker.sock")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // gabung stderr ke stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		// Tulis ke writer + flush
		if _, err := fmt.Fprintln(w, line); err != nil {
			_ = cmd.Process.Kill()
			return err
		}
	}

	// Check if the loop terminated due to a read error rather than a clean EOF
	if err := scanner.Err(); err != nil {
		_ = cmd.Process.Kill()
		return err
	}

	return cmd.Wait()
}

func (s *BuildService) buildArgs(req BuildRequest) []string {
	builder := req.Builder
	if builder == "" {
		builder = s.defaultBuilder
	}
	args := []string{"build", req.ImageName, "--path", req.SourcePath, "--builder", builder, "--verbose"}
	for k, v := range req.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}
	for _, bp := range req.Buildpacks {
		args = append(args, "--buildpack", bp)
	}
	return args
}

// RemoveImage menghapus image hasil build (misal saat deployment gagal).
func (s *BuildService) RemoveImage(ctx context.Context, imageName string) error {
	cmd := exec.CommandContext(ctx, "docker", "rmi", "-f", imageName)
	cmd.Env = append(os.Environ(), "DOCKER_HOST=unix:///var/run/docker.sock")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal hapus image %s: %w (output: %s)", imageName, err, string(out))
	}
	return nil
}
