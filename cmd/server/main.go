package main

import (
	webembed "W-dev"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"W-dev/internal/config"
	"W-dev/internal/constants"
	"W-dev/internal/handler"
	"W-dev/internal/repository"
	"W-dev/internal/router"
	"W-dev/internal/service"
	"W-dev/internal/websocket"
	"W-dev/internal/worker"
	"W-dev/pkg/docker"
)

func main() {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if !cfg.IsProduction() && cfg.JWTSecret == "change-me" {
		log.Println("[peringatan] JWT_SECRET masih default; ganti sebelum production")
	}

	// 2. Workspace
	for _, d := range []string{
		filepath.Join(cfg.WorkspacePath, constants.ReposDirName),
		filepath.Join(cfg.WorkspacePath, constants.ArtifactsDirName),
		filepath.Join(cfg.WorkspacePath, constants.LogsDirName),
		filepath.Join(cfg.WorkspacePath, "uploads"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			log.Fatalf("buat direktori %s: %v", d, err)
		}
	}

	// 3. Database
	db, err := repository.NewDb(cfg)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()
	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("migrasi: %v", err)
	}

	// 4. Docker
	dockerCli, err := docker.NewClient()
	if err != nil {
		log.Fatalf("docker: %v", err)
	}
	defer dockerCli.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if _, err := dockerCli.EnsureNetwork(ctx, constants.DockerNetworkName); err != nil {
		log.Fatalf("network: %v", err)
	}

	// 5. Repository
	appRepo := repository.NewAppRepository(db)
	deployRepo := repository.NewDeploymentRepository(db)
	envRepo := repository.NewEnvVarRepository(db)
	appDBRepo := repository.NewAppDatabaseRepository(db)
	tokenRepo := repository.NewUserTokenRepository(db)

	// 6. Service
	ghSvc := service.NewGitHubService(cfg.GitHubClientID, cfg.GitHubClientSecret,
		cfg.GitHubWebhookSecret, cfg.GitHubWebhookBaseURL)
	if !ghSvc.Configured() {
		log.Println("[peringatan] GitHub OAuth belum dikonfigurasi (GITHUB_CLIENT_ID/SECRET)")
	}
	tokenSvc := service.NewTokenService(tokenRepo, cfg.JWTSecret)
	traefikSvc := service.NewTraefikService(constants.DockerNetworkName, cfg.MainDomain, cfg.TraefikTLS, cfg.TraefikCertResolver)
	containerSvc := service.NewContainerService(dockerCli, constants.DockerNetworkName, traefikSvc)
	dbSvc := service.NewDatabaseService(dockerCli, constants.DockerNetworkName)
	buildSvc := service.NewBuildService(cfg.BuilderImage)
	gitSvc := service.NewGitService()
	uploadSvc := service.NewUploadService()
	cleanupSvc := service.NewCleanupService(containerSvc, dbSvc, buildSvc, ghSvc, tokenSvc, appRepo, appDBRepo, cfg.WorkspacePath)

	// 7. WebSocket hub, queue, worker
	hub := websocket.NewHub()
	queue := worker.NewQueue(cfg.QueueSize)
	resourceSvc := service.NewResourceService(hub)
	appSvc := service.NewAppService(appRepo, deployRepo, envRepo, appDBRepo, ghSvc, tokenSvc, traefikSvc, cleanupSvc, queue)

	dw := worker.NewDeployWorker(worker.Deps{
		Queue: queue, Apps: appRepo, Deploys: deployRepo, Envs: envRepo, AppDBs: appDBRepo,
		Git: gitSvc, Upload: uploadSvc, Build: buildSvc, Containers: containerSvc,
		Databases: dbSvc, Tokens: tokenSvc, Traefik: traefikSvc, Sink: hub, Workspace: cfg.WorkspacePath,
	})
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		dw.Start(ctx, cfg.WorkerCount)
	}()

	// 8. HTTP
	handlers := router.Handlers{
		Auth:    handler.NewAuthHandler(cfg, ghSvc, tokenSvc),
		GitHub:  handler.NewGitHubHandler(ghSvc, tokenSvc),
		Deploy:  handler.NewDeployHandler(appSvc, cfg.WorkspacePath),
		App:     handler.NewAppHandler(appSvc),
		Webhook: handler.NewWebhookHandler(ghSvc, appSvc, appRepo),
		WS:      handler.NewWSHandler(hub, appSvc, containerSvc, resourceSvc, cfg.CORSOrigins),
	}
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router.New(webembed.EmbeddedFiles, cfg, handlers, "web/dist"),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout sengaja tidak diset agar WebSocket & upload besar tidak terputus.
	}

	go func() {
		log.Printf("[server] mendengarkan di %s (env=%s)", srv.Addr, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	// 9. Graceful shutdown
	<-ctx.Done()
	log.Println("[server] shutdown...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)

	queue.Close() // worker berhenti setelah job berjalan selesai (ctx sudah dibatalkan → build dihentikan)
	select {
	case <-workerDone:
	case <-time.After(20 * time.Second):
		log.Println("[server] worker belum selesai, keluar paksa")
	}
}
