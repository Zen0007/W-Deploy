package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"W-dev/internal/constants"
	"W-dev/internal/model"
	"W-dev/internal/repository"
	"W-dev/pkg/util"
)

var (
	ErrNotFound  = errors.New("app tidak ditemukan")
	ErrForbidden = errors.New("anda tidak memiliki akses ke app ini")
	ErrSlugTaken = errors.New("slug sudah dipakai oleh app lain")
	ErrInvalid   = errors.New("input tidak valid")
	ErrQueueFull = errors.New("antrean deploy penuh, coba lagi sebentar")
)

// reservedSlugs tidak boleh dipakai sebagai subdomain app.
var reservedSlugs = map[string]bool{
	"api": true, "app": true, "www": true, "admin": true, "traefik": true,
	"dashboard": true, "mail": true, "ftp": true, "ns1": true, "ns2": true,
}

// AppInput adalah data untuk membuat/menimpa konfigurasi app sebelum deploy.
type AppInput struct {
	Owner      string
	Name       string
	Slug       string
	SourceType constants.SourceType
	RepoFull   string // owner/repo (source github)
	Branch     string
	Port       int
	Buildpack  string
	AutoDeploy *bool
	Env        map[string]string
	Database   string
}

// DeployOptions mengatur satu eksekusi deploy.
type DeployOptions struct {
	TriggeredBy string // user | webhook
	ZipPath     string
	DBEngine    string
	CommitSHA   string // opsional (dari webhook)
	CommitMsg   string
}

// AppService mengorkestrasi siklus hidup app (create, deploy, hapus).
type AppService struct {
	apps    *repository.AppRepository
	deploys *repository.DeploymentRepository
	envs    *repository.EnvVarRepository
	dbs     *repository.AppDatabaseRepository
	gh      *GitHubService
	tokens  *TokenService
	traefik *TraefikService
	cleanup *CleanupService
	queue   DeployEnqueuer
}

func NewAppService(
	apps *repository.AppRepository,
	deploys *repository.DeploymentRepository,
	envs *repository.EnvVarRepository,
	dbs *repository.AppDatabaseRepository,
	gh *GitHubService,
	tokens *TokenService,
	traefik *TraefikService,
	cleanup *CleanupService,
	queue DeployEnqueuer,
) *AppService {
	return &AppService{
		apps: apps, deploys: deploys, envs: envs, dbs: dbs,
		gh: gh, tokens: tokens, traefik: traefik, cleanup: cleanup, queue: queue,
	}
}

// View membungkus app dengan URL publik.
func (s *AppService) View(a *repository.App) model.AppView {
	return model.AppView{App: *a, URL: s.traefik.PublicURL(a.Slug)}
}

// Get mengambil app dan memastikan pemiliknya sesuai.
func (s *AppService) Get(ctx context.Context, id uint, owner string) (*repository.App, error) {
	app, err := s.apps.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, ErrNotFound
	}
	if app.OwnerLogin != owner {
		return nil, ErrForbidden
	}
	return app, nil
}

// List mengembalikan semua app milik user.
func (s *AppService) List(ctx context.Context, owner string) ([]repository.App, error) {
	return s.apps.List(ctx, owner)
}

// Deployments mengembalikan riwayat deploy app.
func (s *AppService) Deployments(ctx context.Context, appID uint, limit int) ([]repository.Deployment, error) {
	return s.deploys.ListByApp(ctx, appID, limit)
}

// Prepare membuat app baru atau memperbarui app milik user yang sama (idempotent per slug),
// lalu (untuk source github) memastikan webhook terpasang. Warning non-fatal dikembalikan.
func (s *AppService) Prepare(ctx context.Context, in AppInput) (*repository.App, []string, error) {
	var warnings []string

	if in.Owner == "" {
		return nil, nil, fmt.Errorf("%w: owner kosong", ErrInvalid)
	}
	slug := in.Slug
	if slug == "" {
		slug = util.Slugify(in.Name)
		if slug == "" && in.RepoFull != "" {
			slug = util.Slugify(in.RepoFull[strings.LastIndex(in.RepoFull, "/")+1:])
		}
	}
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	if !util.IsValidSlug(slug) || reservedSlugs[slug] {
		return nil, nil, fmt.Errorf("%w: slug %q tidak valid atau dicadangkan", ErrInvalid, slug)
	}
	if in.SourceType == "" {
		in.SourceType = constants.SourceGitHub
	}
	if in.SourceType != constants.SourceGitHub && in.SourceType != constants.SourceZip {
		return nil, nil, fmt.Errorf("%w: source_type harus github atau zip", ErrInvalid)
	}
	if in.SourceType == constants.SourceGitHub {
		parts := strings.Split(in.RepoFull, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, nil, fmt.Errorf("%w: repo_full_name harus format owner/repo", ErrInvalid)
		}
	}
	if in.Port <= 0 {
		in.Port = constants.DefaultAppPort
	}
	if in.Port > 65535 {
		return nil, nil, fmt.Errorf("%w: port di luar jangkauan", ErrInvalid)
	}
	if in.Branch == "" {
		in.Branch = "main"
	}
	name := in.Name
	if name == "" {
		name = slug
	}
	auto := true
	if in.AutoDeploy != nil {
		auto = *in.AutoDeploy
	}

	app, err := s.apps.FindBySlug(ctx, slug)
	if err != nil {
		return nil, nil, err
	}
	if app != nil && app.OwnerLogin != in.Owner {
		return nil, nil, ErrSlugTaken
	}

	repoURL := ""
	if in.SourceType == constants.SourceGitHub {
		repoURL = "https://github.com/" + in.RepoFull + ".git"
	}

	if app == nil {
		app = &repository.App{
			Slug: slug, Name: name, SourceType: string(in.SourceType),
			RepoURL: repoURL, Branch: in.Branch, Buildpack: in.Buildpack,
			Port: in.Port, Status: string(constants.StatusPending),
			GitHubFullName: in.RepoFull, AutoDeploy: auto, OwnerLogin: in.Owner,
		}
		if err := s.apps.Create(ctx, app); err != nil {
			return nil, nil, fmt.Errorf("simpan app: %w", err)
		}
	} else {
		fields := map[string]any{
			"name": name, "source_type": string(in.SourceType), "repo_url": repoURL,
			"branch": in.Branch, "buildpack": in.Buildpack, "port": in.Port,
			"github_full_name": in.RepoFull, "auto_deploy": auto,
		}
		if err := s.apps.Update(ctx, app.ID, fields); err != nil {
			return nil, nil, fmt.Errorf("update app: %w", err)
		}
		app, err = s.apps.FindByID(ctx, app.ID)
		if err != nil || app == nil {
			return nil, nil, fmt.Errorf("muat ulang app: %w", err)
		}
	}

	for k, v := range in.Env {
		if err := validateEnvKey(k); err != nil {
			return nil, nil, err
		}
		if err := s.envs.Set(ctx, app.ID, k, v); err != nil {
			return nil, nil, fmt.Errorf("simpan env %s: %w", k, err)
		}
	}

	// Auto-webhook (hanya github, hanya jika belum terpasang).
	if in.SourceType == constants.SourceGitHub && auto && app.WebhookID == 0 {
		if w := s.ensureWebhook(ctx, app); w != "" {
			warnings = append(warnings, w)
		}
	}
	return app, warnings, nil
}

// ensureWebhook memasang webhook di repo; mengembalikan teks warning jika gagal (non-fatal).
func (s *AppService) ensureWebhook(ctx context.Context, app *repository.App) string {
	token, err := s.tokens.Get(ctx, app.OwnerLogin)
	if err != nil || token == "" {
		return "webhook tidak dipasang: token GitHub tidak tersedia, login ulang dengan GitHub"
	}
	res, err := s.gh.SetupAutoWebhook(ctx, token, app.GitHubFullName, []string{"push"})
	if err != nil {
		log.Printf("[app] webhook %s gagal: %v", app.GitHubFullName, err)
		return "webhook otomatis gagal dipasang: " + err.Error()
	}
	if err := s.apps.Update(ctx, app.ID, map[string]any{"webhook_id": res.WebhookID}); err != nil {
		return "webhook terpasang tetapi gagal disimpan: " + err.Error()
	}
	app.WebhookID = res.WebhookID
	return ""
}

// Deploy membuat record deployment lalu mengantrekannya ke worker.
func (s *AppService) Deploy(ctx context.Context, app *repository.App, opt DeployOptions) (*repository.Deployment, error) {
	if opt.TriggeredBy == "" {
		opt.TriggeredBy = "user"
	}
	d := &repository.Deployment{
		AppID: app.ID, Status: string(constants.StatusPending),
		TriggeredBy: opt.TriggeredBy, CommitSHA: opt.CommitSHA, CommitMsg: opt.CommitMsg,
	}
	if err := s.deploys.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("simpan deployment: %w", err)
	}

	job := DeployJob{AppID: app.ID, DeploymentID: d.ID, ZipPath: opt.ZipPath, DBEngine: opt.DBEngine}
	if err := s.queue.Enqueue(job); err != nil {
		_ = s.deploys.Finish(ctx, d.ID, string(constants.StatusFailed), "", err.Error(), "")
		return nil, err
	}
	return d, nil
}

// SetEnv menyimpan variabel environment (berlaku pada deploy berikutnya).
func (s *AppService) SetEnv(ctx context.Context, appID uint, env map[string]string) error {
	for k, v := range env {
		if err := validateEnvKey(k); err != nil {
			return err
		}
		if err := s.envs.Set(ctx, appID, k, v); err != nil {
			return err
		}
	}
	return nil
}

func (s *AppService) ListEnv(ctx context.Context, appID uint) ([]repository.EnvVar, error) {
	return s.envs.ListByApp(ctx, appID)
}

func (s *AppService) DeleteEnv(ctx context.Context, appID uint, key string) error {
	return s.envs.Delete(ctx, appID, key)
}

// Delete menghapus app beserta seluruh resource-nya.
func (s *AppService) Delete(ctx context.Context, app *repository.App) error {
	return s.cleanup.DeleteApp(ctx, app)
}

func validateEnvKey(k string) error {
	if k == "" || len(k) > 200 {
		return fmt.Errorf("%w: nama env kosong/terlalu panjang", ErrInvalid)
	}
	for i, r := range k {
		ok := r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9')
		if !ok {
			return fmt.Errorf("%w: nama env %q tidak valid", ErrInvalid, k)
		}
	}
	return nil
}
