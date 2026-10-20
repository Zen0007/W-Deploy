package service

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"W-dev/internal/constants"
	"W-dev/internal/repository"
)

// CleanupService membersihkan semua resource milik sebuah app.
type CleanupService struct {
	containers *ContainerService
	databases  *DatabaseService
	build      *BuildService
	gh         *GitHubService
	tokens     *TokenService
	apps       *repository.AppRepository
	appDBs     *repository.AppDatabaseRepository
	workspace  string
}

func NewCleanupService(
	containers *ContainerService,
	databases *DatabaseService,
	build *BuildService,
	gh *GitHubService,
	tokens *TokenService,
	apps *repository.AppRepository,
	appDBs *repository.AppDatabaseRepository,
	workspace string,
) *CleanupService {
	return &CleanupService{
		containers: containers, databases: databases, build: build, gh: gh,
		tokens: tokens, apps: apps, appDBs: appDBs, workspace: workspace,
	}
}

// DeleteApp menghapus container, database, image, webhook, workspace, lalu baris DB.
// Kegagalan tiap langkah hanya dicatat (best-effort) agar penghapusan tidak macet di tengah.
func (s *CleanupService) DeleteApp(ctx context.Context, app *repository.App) error {
	// 1. Container app
	if err := s.containers.Remove(ctx, ContainerName(app.Slug)); err != nil {
		log.Printf("[cleanup] container %s: %v", app.Slug, err)
	}

	// 2. Database terisolasi + volume
	dbs, err := s.appDBs.ListByApp(ctx, app.ID)
	if err != nil {
		log.Printf("[cleanup] list database %s: %v", app.Slug, err)
	}
	for _, d := range dbs {
		if err := s.databases.Deprovision(ctx, d.ContainerID, d.VolumeName); err != nil {
			log.Printf("[cleanup] database %s: %v", d.ContainerName, err)
		}
	}

	// 3. Image
	if app.ImageTag != "" {
		if err := s.build.RemoveImage(ctx, app.ImageTag); err != nil {
			log.Printf("[cleanup] image: %v", err)
		}
	}

	// 4. Webhook GitHub
	if app.WebhookID != 0 && app.GitHubFullName != "" {
		if token, err := s.tokens.Get(ctx, app.OwnerLogin); err == nil && token != "" {
			if err := s.gh.DeleteWebhook(ctx, token, app.GitHubFullName, app.WebhookID); err != nil {
				log.Printf("[cleanup] webhook: %v", err)
			}
		}
	}

	// 5. Workspace (source code)
	for _, dir := range []string{
		filepath.Join(s.workspace, constants.ReposDirName, app.Slug),
		filepath.Join(s.workspace, constants.ArtifactsDirName, app.Slug),
	} {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("[cleanup] workspace %s: %v", dir, err)
		}
	}

	// 6. Baris DB (hard delete; relasi ikut terhapus via FK cascade)
	return s.apps.Delete(ctx, app.ID)
}
