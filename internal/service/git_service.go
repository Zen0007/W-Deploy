package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var branchRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/\-]{0,199}$`)

// GitService membungkus operasi git (clone, info commit).
type GitService struct {
	gitBin  string
	timeout time.Duration
}

func NewGitService() *GitService {
	return &GitService{gitBin: "git", timeout: 5 * time.Minute}
}

// CloneRequest adalah input untuk clone repository.
type CloneRequest struct {
	RepoURL string // https://github.com/owner/repo(.git)
	Branch  string // default: main
	Dest    string // direktori tujuan (akan dibersihkan lebih dulu)
	Token   string // opsional: access token GitHub untuk repo private
}

// CloneResult berisi info commit HEAD setelah clone.
type CloneResult struct {
	CommitSHA string
	CommitMsg string
}

// Clone melakukan shallow clone. Token disuntikkan lewat header HTTP sementara
// (bukan ke URL), sehingga tidak tersimpan di .git/config maupun muncul di log.
func (s *GitService) Clone(ctx context.Context, req CloneRequest) (*CloneResult, error) {
	if err := validateRepoURL(req.RepoURL); err != nil {
		return nil, err
	}
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}
	if !branchRe.MatchString(branch) || strings.Contains(branch, "..") {
		return nil, fmt.Errorf("nama branch tidak valid: %q", branch)
	}
	if req.Dest == "" {
		return nil, errors.New("dest kosong")
	}

	// Bersihkan target lama agar clone selalu segar.
	if err := os.RemoveAll(req.Dest); err != nil {
		return nil, fmt.Errorf("gagal membersihkan %s: %w", req.Dest, err)
	}
	if err := os.MkdirAll(filepath.Dir(req.Dest), 0o755); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	args := []string{}
	if req.Token != "" {
		cred := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + req.Token))
		args = append(args, "-c", "http.extraHeader=Authorization: Basic "+cred)
	}
	args = append(args, "clone", "--depth", "1", "--single-branch", "--branch", branch, "--", req.RepoURL, req.Dest)

	cmd := exec.CommandContext(ctx, s.gitBin, args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := scrub(string(out), req.Token)
		return nil, fmt.Errorf("git clone gagal: %w: %s", err, strings.TrimSpace(msg))
	}

	return s.headInfo(ctx, req.Dest), nil
}

// headInfo membaca SHA & pesan commit HEAD (best-effort).
func (s *GitService) headInfo(ctx context.Context, dir string) *CloneResult {
	res := &CloneResult{}
	if out, err := exec.CommandContext(ctx, s.gitBin, "-C", dir, "rev-parse", "HEAD").Output(); err == nil {
		res.CommitSHA = strings.TrimSpace(string(out))
	}
	if out, err := exec.CommandContext(ctx, s.gitBin, "-C", dir, "log", "-1", "--pretty=%s").Output(); err == nil {
		res.CommitMsg = strings.TrimSpace(string(out))
	}
	return res
}

func validateRepoURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("repo url tidak valid: %q", raw)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("hanya https yang diizinkan untuk repo url (dapat %q)", u.Scheme)
	}
	if u.User != nil {
		return errors.New("repo url tidak boleh mengandung kredensial")
	}
	return nil
}

func scrub(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "***")
}
