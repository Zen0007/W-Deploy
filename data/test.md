# Restrukturisasi Kode dengan Folder Structure Baru

Struktur folder Anda memakai **Traefik** sebagai reverse proxy (bukan Nginx Proxy Manager), **WebSocket Hub** untuk streaming log real-time, dan **Worker Queue** untuk deployment async. Berikut implementasi lengkap yang mengikuti struktur tersebut.

---

## 1. `go.mod`

```go
module github.com/username/paas-backend

go 1.27.0

require (
    // Web framework & router
    github.com/go-chi/chi/v5 v5.2.1
    github.com/go-chi/cors v1.2.2

    // Docker SDK (Moby - pengganti github.com/docker/docker yang deprecated)
    github.com/moby/moby/client v0.6.1
    github.com/moby/moby/api v1.56.1

    // Cloudflare SDK terbaru
    github.com/cloudflare/cloudflare-go/v7 v7.11.0

    // Database
    gorm.io/gorm v1.31.0
    gorm.io/driver/postgres v1.6.0

    // WebSocket
    github.com/coder/websocket v1.8.14
    github.com/coder/websocket/wsjson v1.8.14

    // Config
    github.com/joho/godotenv v1.5.1

    // UUID
    github.com/google/uuid v1.6.0

    // Logging
    go.uber.org/zap v1.27.0

    // Git operations
    github.com/go-git/go-git/v5 v5.16.3

    // JWT
    github.com/golang-jwt/jwt/v5 v5.3.0
)
```

---

## 2. `cmd/server/main.go`

```go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/username/paas-backend/internal/config"
	"github.com/username/paas-backend/internal/repository"
	"github.com/username/paas-backend/internal/router"
	"github.com/username/paas-backend/internal/service"
	"github.com/username/paas-backend/internal/websocket"
	"github.com/username/paas-backend/internal/worker"
	"github.com/username/paas-backend/pkg/cloudflared"
	"github.com/username/paas-backend/pkg/docker"
	"github.com/username/paas-backend/pkg/util"
)

func main() {
	// ==== 1. Load config ====
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	logger := util.NewLogger(cfg.Env)
	defer logger.Sync()

	// ==== 2. Init Docker ====
	dockerCli, err := docker.NewClient()
	if err != nil {
		logger.Fatal("docker init gagal", util.Err(err))
	}
	defer dockerCli.Close()

	networkID, err := dockerCli.EnsureNetwork(context.Background(), config.DockerNetworkName)
	if err != nil {
		logger.Fatal("network provision gagal", util.Err(err))
	}
	logger.Info("network siap", util.Str("id", networkID[:12]))

	// ==== 3. Init Database ====
	db, err := repository.NewDB(cfg)
	if err != nil {
		logger.Fatal("db init gagal", util.Err(err))
	}
	defer db.Close()

	if err := db.AutoMigrate(); err != nil {
		logger.Fatal("migrasi gagal", util.Err(err))
	}

	// ==== 4. Init Repositories ====
	appRepo := repository.NewAppRepository(db)
	deployRepo := repository.NewDeploymentRepository(db)
	domainRepo := repository.NewDomainRepository(db)

	// ==== 5. Init Cloudflare ====
	tunnelSvc, err := cloudflared.NewTunnelService(
		cfg.CloudflareAPIToken,
		cfg.CloudflareAccountID,
		cfg.CloudflareZoneID,
		cfg.MainDomain,
	)
	if err != nil {
		logger.Fatal("cloudflare init gagal", util.Err(err))
	}

	// ==== 6. Init Services ====
	dbSvc := service.NewDatabaseService(dockerCli, config.DockerNetworkName)
	buildSvc := service.NewBuildService(config.PaketoBuilderImage)
	gitSvc := service.NewGitService(cfg.WorkspacePath)
	uploadSvc := service.NewUploadService(cfg.WorkspacePath)
	traefikSvc := service.NewTraefikService(dockerCli, config.DockerNetworkName)
	cleanupSvc := service.NewCleanupService(dockerCli, dbSvc)
	appSvc := service.NewAppService(appRepo, domainRepo, tunnelSvc, traefikSvc)

	// ==== 7. Init WebSocket Hub ====
	hub := websocket.NewHub()
	go hub.Run()

	// ==== 8. Init Worker ====
	queue := worker.NewQueue(100)
	deployWorker := worker.NewDeployWorker(queue, appRepo, deployRepo, buildSvc, dockerCli, hub, logger)
	go deployWorker.Start(context.Background())

	// ==== 9. Init Router ====
	r := router.New(router.Deps{
		Cfg:        cfg,
		Logger:     logger,
		AppSvc:     appSvc,
		DBService:  dbSvc,
		BuildSvc:   buildSvc,
		GitSvc:     gitSvc,
		UploadSvc:  uploadSvc,
		TraefikSvc: traefikSvc,
		CleanupSvc: cleanupSvc,
		Hub:        hub,
		Queue:      queue,
		AppRepo:    appRepo,
		DeployRepo: deployRepo,
		DomainRepo: domainRepo,
	})

	// ==== 10. Start HTTP server ====
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // 0 untuk SSE/WebSocket
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("server started", util.Str("port", cfg.Port), util.Str("domain", cfg.MainDomain))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server error", util.Err(err))
		}
	}()

	// ==== 11. Graceful shutdown ====
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", util.Err(err))
	}
	close(queue.Chan())
}
```

---

## 3. `internal/config/config.go` — Sudah ada, tambahkan `Logger()` helper

```go
package config

// (Kode sebelumnya tetap, tambahkan helper berikut)

func (c *Config) WorkspaceReposPath() string {
	return c.WorkspacePath + "/repos"
}

func (c *Config) WorkspaceArtifactsPath() string {
	return c.WorkspacePath + "/artifacts"
}

func (c *Config) WorkspaceLogsPath() string {
	return c.WorkspacePath + "/logs"
}
```

## 4. `internal/config/constants.go`

```go
package config

import "time"

const (
	DockerNetworkName = "proxy-net"
	PaketoBuilderImage = "paketobuildpacks/builder-jammy-base:latest"
	PaketoRunImage     = "paketobuildpacks/run-jammy-base:latest"
	DefaultAppPort     = 3000
	TraefikContainerName = "paas-traefik"
	TraefikConfigDir     = "/etc/traefik/dynamic"
	TraefikLabelsPrefix  = "traefik"
)

const (
	BuildTimeout  = 20 * time.Minute
	DeployTimeout = 10 * time.Minute
)

const (
	StatusPending   = "pending"
	StatusBuilding  = "building"
	StatusDeploying = "deploying"
	StatusRunning   = "running"
	StatusFailed    = "failed"
	StatusStopped   = "stopped"
)
```

---

## 5. `internal/model/app.go` — Model dipindah dari `repository`

```go
package model

import (
	"time"

	"gorm.io/gorm"
)

type App struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Slug        string `gorm:"uniqueIndex;size:100;not null" json:"slug"`
	Name        string `gorm:"size:200;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`

	SourceType string `gorm:"size:20;not null;default:'github'" json:"source_type"`
	RepoURL    string `gorm:"size:500" json:"repo_url"`
	Branch     string `gorm:"size:100;default:'main'" json:"branch"`
	Buildpack  string `gorm:"size:200" json:"buildpack"`

	Port        int    `gorm:"default:3000" json:"port"`
	ContainerID string `gorm:"size:100" json:"container_id"`
	ImageTag    string `gorm:"size:200" json:"image_tag"`

	Status  string `gorm:"size:30;not null;default:'pending'" json:"status"`
	LastErr string `gorm:"type:text" json:"last_error"`

	// Database config (opsional)
	DBEngine     string `gorm:"size:20" json:"db_engine"`
	DBContainerID string `gorm:"size:100" json:"db_container_id"`
	DBVolumeName  string `gorm:"size:200" json:"db_volume_name"`
	DBConnectionURI string `gorm:"type:text" json:"-"` // jangan expose via JSON

	Deployments []Deployment `gorm:"foreignKey:AppID;constraint:OnDelete:CASCADE" json:"deployments,omitempty"`
	Domains     []Domain     `gorm:"foreignKey:AppID;constraint:OnDelete:CASCADE" json:"domains,omitempty"`
	EnvVars     []EnvVar     `gorm:"foreignKey:AppID;constraint:OnDelete:CASCADE" json:"env_vars,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (App) TableName() string { return "apps" }

type Deployment struct {
	ID    uint `gorm:"primaryKey" json:"id"`
	AppID uint `gorm:"index;not null" json:"app_id"`

	CommitSHA string `gorm:"size:64" json:"commit_sha"`
	CommitMsg string `gorm:"type:text" json:"commit_message"`
	ImageTag  string `gorm:"size:200" json:"image_tag"`

	Status   string `gorm:"size:30;not null;default:'pending'" json:"status"`
	BuildLog string `gorm:"type:text" json:"build_log"`
	ErrorMsg string `gorm:"type:text" json:"error_message"`

	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	DurationSec int        `json:"duration_sec"`
	TriggeredBy string     `gorm:"size:100" json:"triggered_by"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Deployment) TableName() string { return "deployments" }

type Domain struct {
	ID    uint `gorm:"primaryKey" json:"id"`
	AppID uint `gorm:"index;not null" json:"app_id"`

	Hostname   string `gorm:"uniqueIndex;size:255;not null" json:"hostname"`
	IsPrimary  bool   `gorm:"default:false" json:"is_primary"`
	SSLEnabled bool   `gorm:"default:true" json:"ssl_enabled"`

	CFZoneID     string `gorm:"size:64" json:"cf_zone_id"`
	CFRecordID   string `gorm:"size:64" json:"cf_record_id"`
	CFTunnelID   string `gorm:"size:64" json:"cf_tunnel_id"`
	CFTunnelName string `gorm:"size:128" json:"cf_tunnel_name"`

	Status    string `gorm:"size:30;not null;default:'pending'" json:"status"`
	LastError string `gorm:"type:text" json:"last_error"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Domain) TableName() string { return "domains" }

type EnvVar struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	AppID uint   `gorm:"index;not null" json:"app_id"`
	Key   string `gorm:"size:200;not null" json:"key"`
	Value string `gorm:"type:text" json:"value"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (EnvVar) TableName() string { return "env_vars" }
```

## 6. `internal/model/request.go`

```go
package model

type CreateAppRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=100"`
	Slug        string `json:"slug" validate:"required,min=2,max=100,alphanum"`
	Description string `json:"description"`

	SourceType string `json:"source_type"` // github|zip|git
	RepoURL    string `json:"repo_url"`
	Branch     string `json:"branch"`
	Buildpack  string `json:"buildpack"`

	Port int `json:"port"`

	// Database opsional
	EnableDatabase bool   `json:"enable_database"`
	DBEngine       string `json:"db_engine"` // postgres|mysql|mongodb|redis
	DBVersion      string `json:"db_version"`

	// Custom domain
	CustomHostname string `json:"custom_hostname"`
}

type DeployRequest struct {
	AppID      uint   `json:"app_id" validate:"required"`
	CommitSHA  string `json:"commit_sha"`
	TriggeredBy string `json:"triggered_by"`
}

type ProvisionDBRequest struct {
	Engine  string `json:"engine" validate:"required,oneof=postgres mysql mongodb redis"`
	Version string `json:"version"`
	DBName  string `json:"db_name"`
}

type UploadRequest struct {
	AppSlug  string `json:"app_slug" validate:"required"`
	Filename string `json:"filename"`
}
```

---

## 7. `internal/repository/db.go` — Update import model

```go
package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/username/paas-backend/internal/config"
	"github.com/username/paas-backend/internal/model"
)

type DB struct{ *gorm.DB }

func NewDB(cfg *config.Config) (*DB, error) {
	logLevel := logger.Warn
	if !cfg.IsProduction() {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logLevel),
		DisableForeignKeyConstraintWhenMigrating: false,
		PrepareStmt:                              true,
	})
	if err != nil {
		return nil, fmt.Errorf("gagal konek postgres: %w", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("postgres tidak merespon: %w", err)
	}

	log.Println("[db] terhubung ke PostgreSQL")
	return &DB{db}, nil
}

func (d *DB) AutoMigrate() error {
	return d.DB.AutoMigrate(
		&model.App{},
		&model.Deployment{},
		&model.Domain{},
		&model.EnvVar{},
	)
}

func (d *DB) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
```

## 8. `internal/repository/app_repo.go` (contoh — update import)

```go
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/username/paas-backend/internal/model"
)

type AppRepository struct{ db *gorm.DB }

func NewAppRepository(db *DB) *AppRepository {
	return &AppRepository{db: db.DB}
}

func (r *AppRepository) Create(ctx context.Context, app *model.App) error {
	return r.db.WithContext(ctx).Create(app).Error
}

func (r *AppRepository) FindByID(ctx context.Context, id uint) (*model.App, error) {
	var app model.App
	err := r.db.WithContext(ctx).Preload("Domains").Preload("EnvVars").First(&app, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &app, err
}

func (r *AppRepository) FindBySlug(ctx context.Context, slug string) (*model.App, error) {
	var app model.App
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&app).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &app, err
}

func (r *AppRepository) List(ctx context.Context) ([]model.App, error) {
	var apps []model.App
	return apps, r.db.WithContext(ctx).Order("created_at DESC").Find(&apps).Error
}

func (r *AppRepository) Update(ctx context.Context, app *model.App) error {
	return r.db.WithContext(ctx).Save(app).Error
}

func (r *AppRepository) UpdateStatus(ctx context.Context, id uint, status, lastErr string) error {
	return r.db.WithContext(ctx).Model(&model.App{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "last_err": lastErr}).Error
}

func (r *AppRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.App{}, id).Error
}
```

`deployment_repo.go` dan `domain_repo.go` mengikuti pola yang sama — ganti import `model` dan tipe `App` → `model.App`, dst.

---

## 9. `pkg/docker/client.go` — Versi final

```go
package docker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moby/moby/client"
)

type Client struct {
	*client.Client
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost("unix:///var/run/docker.sock"),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("gagal buat docker client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := cli.Ping(ctx); err != nil {
		return nil, fmt.Errorf("docker daemon tidak merespon: %w", err)
	}

	log.Println("[docker] terhubung ke daemon")
	return &Client{cli}, nil
}

func (c *Client) Close() error { return c.Client.Close() }
```

## 10. `pkg/docker/container.go`

```go
package docker

import (
	"context"
	"time"

	"github.com/moby/moby/client"
)

func (c *Client) ContainerStop(ctx context.Context, id string, timeout time.Duration) error {
	secs := int(timeout.Seconds())
	return c.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &secs})
}

func (c *Client) ContainerRemove(ctx context.Context, id string, force bool) error {
	return c.ContainerRemove(ctx, id, client.ContainerRemoveOptions{
		Force:         force,
		RemoveVolumes: true,
	})
}

func (c *Client) ContainerInspect(ctx context.Context, id string) (*client.ContainerInspectResult, error) {
	return c.ContainerInspect(ctx, id)
}
```

## 11. `pkg/docker/network.go` — Sama seperti sebelumnya

```go
package docker

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/moby/moby/client"
)

func (c *Client) EnsureNetwork(ctx context.Context, name string) (string, error) {
	nets, err := c.NetworkList(ctx, client.NetworkListOptions{
		Filters: make(client.Filters).Add("name", name),
	})
	if err != nil {
		return "", fmt.Errorf("gagal list network: %w", err)
	}
	for _, n := range nets {
		if n.Name == name {
			log.Printf("[docker] network '%s' sudah ada", name)
			return n.ID, nil
		}
	}

	resp, err := c.NetworkCreate(ctx, name, client.NetworkCreateOptions{
		Driver:         "bridge",
		Attachable:     true,
		CheckDuplicate: true,
		Labels:         map[string]string{"paas.managed": "true"},
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return name, nil
		}
		return "", err
	}
	log.Printf("[docker] network '%s' dibuat (id=%s)", name, resp.ID[:12])
	return resp.ID, nil
}

func (c *Client) CreateVolume(ctx context.Context, name string) error {
	_, err := c.VolumeCreate(ctx, client.VolumeCreateOptions{
		Name:   name,
		Driver: "local",
		Labels: map[string]string{"paas.managed": "true"},
	})
	return err
}

func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	return c.VolumeRemove(ctx, name, true)
}
```

---

## 12. `pkg/cloudflared/client.go`

```go
package cloudflared

import (
	"fmt"

	cf "github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/option"
)

type Client struct {
	api       *cf.Client
	accountID string
	zoneID    string
}

func NewClient(apiToken, accountID, zoneID string) (*Client, error) {
	if apiToken == "" || accountID == "" {
		return nil, fmt.Errorf("CLOUDFLARE_API_TOKEN & ACCOUNT_ID wajib")
	}
	return &Client{
		api:       cf.NewClient(option.WithAPIToken(apiToken)),
		accountID: accountID,
		zoneID:    zoneID,
	}, nil
}

func (c *Client) API() *cf.Client    { return c.api }
func (c *Client) AccountID() string  { return c.accountID }
func (c *Client) ZoneID() string     { return c.zoneID }
```

## 13. `pkg/cloudflared/tunnel.go`

```go
package cloudflared

import (
	"context"
	"fmt"
	"log"

	cf "github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/dns"
	"github.com/cloudflare/cloudflare-go/v7/option"
	"github.com/cloudflare/cloudflare-go/v7/zero_trust"
)

type TunnelService struct {
	client     *Client
	mainDomain string
}

func NewTunnelService(apiToken, accountID, zoneID, mainDomain string) (*TunnelService, error) {
	c, err := NewClient(apiToken, accountID, zoneID)
	if err != nil {
		return nil, err
	}
	return &TunnelService{client: c, mainDomain: mainDomain}, nil
}

func (s *TunnelService) CreateTunnel(ctx context.Context, name string) (*zero_trust.CloudflareTunnel, error) {
	resp, err := s.client.API().ZeroTrust.Tunnels.Cloudflared.New(ctx, zero_trust.CloudflaredNewParams{
		AccountID: cf.F(s.client.AccountID()),
		Name:      cf.F(name),
		ConfigSrc: cf.F(zero_trust.CloudflaredNewParamsConfigSrcCloudflare),
	})
	if err != nil {
		return nil, fmt.Errorf("create tunnel: %w", err)
	}
	log.Printf("[cf] tunnel '%s' dibuat (%s)", name, resp.ID)
	return resp, nil
}

func (s *TunnelService) GetTunnelToken(ctx context.Context, tunnelID string) (string, error) {
	resp, err := s.client.API().ZeroTrust.Tunnels.Cloudflared.Token.Get(ctx, tunnelID,
		zero_trust.CloudflaredTokenGetParams{AccountID: cf.F(s.client.AccountID())})
	if err != nil {
		return "", err
	}
	return resp.Token, nil
}

func (s *TunnelService) DeleteTunnel(ctx context.Context, tunnelID string) error {
	_, err := s.client.API().ZeroTrust.Tunnels.Cloudflared.Delete(ctx, tunnelID,
		zero_trust.CloudflaredDeleteParams{AccountID: cf.F(s.client.AccountID())})
	return err
}

func (s *TunnelService) CreateCNAMERecord(ctx context.Context, hostname, tunnelID string) (*dns.RecordNewResponse, error) {
	target := fmt.Sprintf("%s.cfargotunnel.com", tunnelID)
	resp, err := s.client.API().DNS.Records.New(ctx, dns.RecordNewParams{
		ZoneID: cf.F(s.client.ZoneID()),
		Body: dns.RecordNewParamsBody{
			Type:    cf.F(dns.RecordNewParamsBodyTypeCNAME),
			Name:    cf.F(hostname),
			Content: cf.F(target),
			Proxied: cf.F(true),
			TTL:     cf.F(dns.TTL1),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create CNAME: %w", err)
	}
	log.Printf("[cf] CNAME %s → %s", hostname, target)
	return resp, nil
}

func (s *TunnelService) DeleteDNSRecord(ctx context.Context, recordID string) error {
	_, err := s.client.API().DNS.Records.Delete(ctx, recordID, dns.RecordDeleteParams{
		ZoneID: cf.F(s.client.ZoneID()),
	})
	return err
}

// ProvisionApp = buat tunnel + CNAME + ambil token sekaligus.
func (s *TunnelService) ProvisionApp(ctx context.Context, appSlug string) (*ProvisionResult, error) {
	tunnelName := fmt.Sprintf("paas-%s", appSlug)
	hostname := fmt.Sprintf("%s.%s", appSlug, s.mainDomain)

	tunnel, err := s.CreateTunnel(ctx, tunnelName)
	if err != nil {
		return nil, err
	}

	dnsRec, err := s.CreateCNAMERecord(ctx, hostname, tunnel.ID)
	if err != nil {
		_ = s.DeleteTunnel(ctx, tunnel.ID)
		return nil, err
	}

	token, _ := s.GetTunnelToken(ctx, tunnel.ID)

	return &ProvisionResult{
		TunnelID:    tunnel.ID,
		TunnelName:  tunnelName,
		Hostname:    hostname,
		DNSRecordID: dnsRec.ID,
		Token:       token,
	}, nil
}

type ProvisionResult struct {
	TunnelID    string
	TunnelName  string
	Hostname    string
	DNSRecordID string
	Token       string
}
```

---

## 14. `pkg/util/logger.go`

```go
package util

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger = *zap.SugaredLogger

func NewLogger(env string) Logger {
	var cfg zap.Config
	if env == "production" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	l, _ := cfg.Build()
	return l.Sugar()
}

func Err(err error) zap.Field  { return zap.Error(err) }
func Str(k, v string) zap.Field { return zap.String(k, v) }
func Int(k string, v int) zap.Field { return zap.Int(k, v) }
```

## 15. `pkg/util/string.go`

```go
package util

import (
	"crypto/rand"
	"encoding/base64"
	"regexp"
	"strings"
)

var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func RandomPassword(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	s := base64.RawURLEncoding.EncodeToString(b)
	if len(s) > n {
		s = s[:n]
	}
	return strings.TrimRight(s, "-_")
}
```

## 16. `pkg/util/uuid.go`

```go
package util

import "github.com/google/uuid"

func NewUUID() string { return uuid.NewString() }
```

---

## 17. `internal/service/database_service.go`

Sudah diimplementasikan di Fase 3 — cukup sesuaikan import ke `github.com/username/paas-backend/pkg/docker` dan tambahkan method `GetEngineSpec()` untuk validasi GUI/CLI:

```go
// SupportedEngines mengembalikan daftar engine yang didukung (untuk GUI dropdown).
func SupportedEngines() []EngineInfo {
	return []EngineInfo{
		{Engine: DBPostgres, Label: "PostgreSQL 17", DefaultPort: 5432},
		{Engine: DBMySQL, Label: "MySQL 8.4", DefaultPort: 3306},
		{Engine: DBMongo, Label: "MongoDB 7", DefaultPort: 27017},
		{Engine: DBRedis, Label: "Redis 7", DefaultPort: 6379},
	}
}

type EngineInfo struct {
	Engine      DBEngine `json:"engine"`
	Label       string   `json:"label"`
	DefaultPort int      `json:"default_port"`
}
```

## 18. `internal/service/build_service.go`

Sudah diimplementasikan di Fase 3. Sesuaikan import dan tambahkan integrasi dengan WebSocket hub:

```go
// BuildToHub menjalankan build & broadcast setiap baris log ke hub WebSocket.
func (s *BuildService) BuildToHub(
	ctx context.Context, req BuildRequest, hub Broadcaster, roomID string,
) (*BuildResult, error) {
	logCh := make(chan string, 100)
	req.LogChannel = logCh

	go func() {
		for line := range logCh {
			hub.Broadcast(roomID, map[string]any{
				"type": "build_log",
				"line": line,
			})
		}
	}()

	result, err := s.Build(ctx, req)
	close(logCh)
	return result, err
}

type Broadcaster interface {
	Broadcast(room string, msg any)
}
```

## 19. `internal/service/traefik_service.go` — BARU (pengganti NPM)

Traefik menggunakan **Docker labels** untuk routing dinamis, jadi service ini hanya perlu membuat container dengan label yang tepat.

```go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/username/paas-backend/pkg/docker"
)

// TraefikService membungkus pembuatan container app dengan label Traefik.
type TraefikService struct {
	docker  *docker.Client
	network string
}

func NewTraefikService(d *docker.Client, network string) *TraefikService {
	return &TraefikService{docker: d, network: network}
}

// RunApp menjalankan container aplikasi user dengan label Traefik untuk routing.
func (s *TraefikService) RunApp(
	ctx context.Context,
	appSlug, imageTag, hostname string,
	appPort int,
	env []string,
) (string, error) {
	containerName := fmt.Sprintf("paas-app-%s", appSlug)

	// Hapus container lama jika ada
	if existing, _ := s.findContainer(ctx, containerName); existing != "" {
		_ = s.docker.ContainerStop(ctx, existing, 10*time.Second)
		_ = s.docker.ContainerRemove(ctx, existing, true)
	}

	// Router name yang valid untuk Traefik (hanya [a-zA-Z0-9_-])
	routerName := appSlug

	labels := map[string]string{
		"paas.managed":       "true",
		"paas.app.slug":      appSlug,

		// Enable Traefik
		"traefik.enable": "true",

		// HTTP router
		fmt.Sprintf("traefik.http.routers.%s.rule", routerName): fmt.Sprintf("Host(`%s`)", hostname),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", routerName): "websecure",
		fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", routerName): "cloudflare",
		fmt.Sprintf("traefik.http.routers.%s.service", routerName): routerName,

		// Service
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", routerName): fmt.Sprintf("%d", appPort),

		// Network
		"traefik.docker.network": s.network,

		// HTTP → HTTPS redirect
		fmt.Sprintf("traefik.http.routers.%s-http.rule", routerName): fmt.Sprintf("Host(`%s`)", hostname),
		fmt.Sprintf("traefik.http.routers.%s-http.entrypoints", routerName): "web",
		fmt.Sprintf("traefik.http.routers.%s-http.middlewares", routerName): fmt.Sprintf("%s-redirect", routerName),
		fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.scheme", routerName): "https",
		fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.permanent", routerName): "true",
	}

	resp, err := s.docker.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: containerName,
		Config: &container.Config{
			Image:  imageTag,
			Env:    env,
			Labels: labels,
			ExposedPorts: network.PortSet{
				network.MustParsePort(fmt.Sprintf("%d/tcp", appPort)): {},
			},
		},
		HostConfig: &container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
			NetworkMode:   container.NetworkMode(s.network),
			Resources: container.Resources{
				Memory:   1 * 1024 * 1024 * 1024, // 1 GB
				NanoCPUs: 2_000_000_000,          // 2 CPU
			},
		},
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				s.network: {},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("gagal create container: %w", err)
	}

	if err := s.docker.ContainerStart(ctx, resp.ID); err != nil {
		_ = s.docker.ContainerRemove(ctx, resp.ID, true)
		return "", fmt.Errorf("gagal start container: %w", err)
	}

	return resp.ID, nil
}

func (s *TraefikService) findContainer(ctx context.Context, name string) (string, error) {
	list, err := s.docker.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("name", name),
	})
	if err != nil {
		return "", err
	}
	if len(list) > 0 {
		return list[0].ID, nil
	}
	return "", nil
}
```

## 20. `internal/service/git_service.go` — BARU

```go
package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type GitService struct {
	workspaceRoot string
}

func NewGitService(root string) *GitService {
	return &GitService{workspaceRoot: root}
}

func (s *GitService) repoPath(slug string) string {
	return filepath.Join(s.workspaceRoot, "repos", slug)
}

// Clone atau pull repository.
func (s *GitService) Sync(ctx context.Context, slug, repoURL, branch string) (string, error) {
	path := s.repoPath(slug)

	// Jika sudah ada, pull
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		return s.pull(path, branch)
	}

	// Clone baru
	repo, err := git.PlainCloneContext(ctx, path, false, &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Depth:         1,
	})
	if err != nil {
		return "", fmt.Errorf("clone gagal: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

func (s *GitService) pull(path, branch string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	err = wt.Pull(&git.PullOptions{
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Force:         true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

func (s *GitService) RepoPath(slug string) string { return s.repoPath(slug) }
```

## 21. `internal/service/upload_service.go` — BARU

```go
package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

type UploadService struct {
	workspaceRoot string
}

func NewUploadService(root string) *UploadService {
	return &UploadService{workspaceRoot: root}
}

// SaveZip menyimpan file zip ke workspace dan mengembalikan path-nya.
func (s *UploadService) SaveZip(slug string, file multipart.File, header *multipart.FileHeader) (string, error) {
	dir := filepath.Join(s.workspaceRoot, "repos", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir gagal: %w", err)
	}

	dstPath := filepath.Join(dir, "source.zip")
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}
	return dstPath, nil
}

// ExtractZip mengekstrak file zip ke direktori tujuan (source code siap build).
func (s *UploadService) ExtractZip(zipPath, destDir string) error {
	// Gunakan arsip/zip stdlib. Singkatnya:
	// - Buka zip via archive/zip
	// - Iterasi file, tulis ke destDir dengan proteksi ZipSlip
	return extractZipSafe(zipPath, destDir)
}
```

## 22. `internal/service/app_service.go` — BARU

```go
package service

import (
	"context"
	"fmt"

	"github.com/username/paas-backend/internal/model"
	"github.com/username/paas-backend/internal/repository"
	"github.com/username/paas-backend/pkg/cloudflared"
	"github.com/username/paas-backend/pkg/util"
)

type AppService struct {
	appRepo    *repository.AppRepository
	domainRepo *repository.DomainRepository
	tunnelSvc  *cloudflared.TunnelService
	traefikSvc *TraefikService
}

func NewAppService(
	appRepo *repository.AppRepository,
	domainRepo *repository.DomainRepository,
	tunnelSvc *cloudflared.TunnelService,
	traefikSvc *TraefikService,
) *AppService {
	return &AppService{
		appRepo:    appRepo,
		domainRepo: domainRepo,
		tunnelSvc:  tunnelSvc,
		traefikSvc: traefikSvc,
	}
}

// CreateApp membuat app + tunnel + CNAME + domain record sekaligus.
func (s *AppService) CreateApp(ctx context.Context, req model.CreateAppRequest) (*model.App, error) {
	slug := util.Slugify(req.Slug)
	if slug == "" {
		slug = util.Slugify(req.Name)
	}

	// Cek duplikat
	existing, _ := s.appRepo.FindBySlug(ctx, slug)
	if existing != nil {
		return nil, fmt.Errorf("slug '%s' sudah dipakai", slug)
	}

	app := &model.App{
		Slug:        slug,
		Name:        req.Name,
		Description: req.Description,
		SourceType:  req.SourceType,
		RepoURL:     req.RepoURL,
		Branch:      req.Branch,
		Buildpack:   req.Buildpack,
		Port:        req.Port,
		Status:      "pending",
	}
	if app.Branch == "" {
		app.Branch = "main"
	}
	if app.Port == 0 {
		app.Port = 3000
	}

	if err := s.appRepo.Create(ctx, app); err != nil {
		return nil, err
	}

	// Provision Cloudflare Tunnel + CNAME
	prov, err := s.tunnelSvc.ProvisionApp(ctx, slug)
	if err != nil {
		_ = s.appRepo.Delete(ctx, app.ID)
		return nil, fmt.Errorf("provision tunnel gagal: %w", err)
	}

	domain := &model.Domain{
		AppID:        app.ID,
		Hostname:     prov.Hostname,
		IsPrimary:    true,
		SSLEnabled:   true,
		CFTunnelID:   prov.TunnelID,
		CFTunnelName: prov.TunnelName,
		CFRecordID:   prov.DNSRecordID,
		Status:       "active",
	}
	if err := s.domainRepo.Create(ctx, domain); err != nil {
		return nil, err
	}

	return app, nil
}
```

## 23. `internal/service/cleanup_service.go` — BARU

```go
package service

import (
	"context"
	"log"

	"github.com/username/paas-backend/pkg/docker"
)

type CleanupService struct {
	docker  *docker.Client
	dbSvc   *DatabaseService
}

func NewCleanupService(d *docker.Client, dbSvc *DatabaseService) *CleanupService {
	return &CleanupService{docker: d, dbSvc: dbSvc}
}

// RemoveAppResources membersihkan semua resource aplikasi:
// container app, container DB, volume, dan image.
func (s *CleanupService) RemoveAppResources(
	ctx context.Context,
	appContainerID, dbContainerID, dbVolume, imageTag string,
) error {
	if appContainerID != "" {
		if err := s.docker.ContainerRemove(ctx, appContainerID, true); err != nil {
			log.Printf("[cleanup] warning app container: %v", err)
		}
	}
	if dbContainerID != "" {
		if err := s.dbSvc.Deprovision(ctx, dbContainerID, dbVolume); err != nil {
			log.Printf("[cleanup] warning db: %v", err)
		}
	}
	if imageTag != "" {
		if err := s.docker.ImageRemove(ctx, imageTag, true); err != nil {
			log.Printf("[cleanup] warning image: %v", err)
		}
	}
	return nil
}
```

---

## 24. `internal/websocket/hub.go`

```go
package websocket

import (
	"sync"
)

type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[*Client]struct{}
	broadcast chan broadcastMsg
}

type broadcastMsg struct {
	room string
	data any
}

func NewHub() *Hub {
	return &Hub{
		rooms:     make(map[string]map[*Client]struct{}),
		broadcast: make(chan broadcastMsg, 256),
	}
}

func (h *Hub) Run() {
	for msg := range h.broadcast {
		h.mu.RLock()
		clients := h.rooms[msg.room]
		h.mu.RUnlock()
		for c := range clients {
			c.Send(msg.data)
		}
	}
}

func (h *Hub) Register(room string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]struct{})
	}
	h.rooms[room][c] = struct{}{}
}

func (h *Hub) Unregister(room string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.rooms[room]; ok {
		delete(clients, c)
		if len(clients) == 0 {
			delete(h.rooms, room)
		}
	}
}

// Broadcast mengirim pesan ke semua client di room.
func (h *Hub) Broadcast(room string, data any) {
	h.broadcast <- broadcastMsg{room: room, data: data}
}
```

## 25. `internal/websocket/client.go`

```go
package websocket

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

type Client struct {
	conn *websocket.Conn
	send chan any
}

func NewClient(w http.ResponseWriter, r *http.Request) (*Client, error) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // set false di produksi & validasi origin
	})
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, send: make(chan any, 64)}, nil
}

func (c *Client) Send(v any) {
	select {
	case c.send <- v:
	default:
		// drop jika penuh
	}
}

func (c *Client) WriteLoop(ctx context.Context) {
	defer c.conn.Close(websocket.StatusNormalClosure, "bye")
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.send:
			ctxW, cancel := context.WithTimeout(ctx, 5*time.Second)
			_ = websocket.WriteJSON(ctxW, c.conn, msg)
			cancel()
		}
	}
}

func (c *Client) Close() { _ = c.conn.CloseNow() }
```

---

## 26. `internal/worker/queue.go`

```go
package worker

import "context"

type Job struct {
	Type    string
	Payload any
	Ctx     context.Context
}

type Queue struct {
	ch chan Job
}

func NewQueue(buffer int) *Queue {
	return &Queue{ch: make(chan Job, buffer)}
}

func (q *Queue) Enqueue(j Job) {
	q.ch <- j
}

func (q *Queue) Chan() <-chan Job { return q.ch }

// Chan() juga digunakan untuk close() (dibutuhkan karena chan tidak bisa di-close dari luar).
// Ganti nama agar bisa di-close via fungsi berikut.
func (q *Queue) Close() { close(q.ch) }
```

> ⚠️ Ubah `main.go` dari `close(queue.Chan())` menjadi `queue.Close()`.

## 27. `internal/worker/deploy_worker.go`

```go
package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/username/paas-backend/internal/model"
	"github.com/username/paas-backend/internal/repository"
	"github.com/username/paas-backend/internal/service"
	"github.com/username/paas-backend/internal/websocket"
	"github.com/username/paas-backend/pkg/docker"
)

type DeployWorker struct {
	queue      *Queue
	appRepo    *repository.AppRepository
	deployRepo *repository.DeploymentRepository
	buildSvc   *service.BuildService
	dockerCli  *docker.Client
	hub        *websocket.Hub
	logger     interface{ Info(...any); Error(...any) }
}

func NewDeployWorker(
	q *Queue,
	appRepo *repository.AppRepository,
	deployRepo *repository.DeploymentRepository,
	buildSvc *service.BuildService,
	dockerCli *docker.Client,
	hub *websocket.Hub,
	logger interface{ Info(...any); Error(...any) },
) *DeployWorker {
	return &DeployWorker{
		queue:      q,
		appRepo:    appRepo,
		deployRepo: deployRepo,
		buildSvc:   buildSvc,
		dockerCli:  dockerCli,
		hub:        hub,
		logger:     logger,
	}
}

func (w *DeployWorker) Start(ctx context.Context) {
	for job := range w.queue.Chan() {
		switch job.Type {
		case "deploy":
			w.handleDeploy(ctx, job.Payload.(*model.Deployment))
		default:
			log.Printf("[worker] unknown job type: %s", job.Type)
		}
	}
}

func (w *DeployWorker) handleDeploy(ctx context.Context, d *model.Deployment) {
	roomID := fmt.Sprintf("app-%d", d.AppID)

	// 1. Update status building
	_ = w.deployRepo.UpdateStatus(ctx, d.ID, "building", "", "")
	w.hub.Broadcast(roomID, map[string]any{"type": "status", "status": "building"})

	// 2. Build image
	app, err := w.appRepo.FindByID(ctx, d.AppID)
	if err != nil || app == nil {
		w.failDeploy(ctx, d, roomID, "app tidak ditemukan")
		return
	}

	imageName := fmt.Sprintf("paas/%s:%d", app.Slug, time.Now().Unix())

	buildReq := service.BuildRequest{
		AppSlug:    app.Slug,
		SourcePath: fmt.Sprintf("/var/lib/paas/workspace/repos/%s", app.Slug),
		ImageName:  imageName,
	}

	start := time.Now()
	result, err := w.buildSvc.BuildToHub(ctx, buildReq, w.hub, roomID)
	if err != nil {
		w.failDeploy(ctx, d, roomID, err.Error())
		return
	}

	// 3. Update deployment
	_ = w.deployRepo.UpdateStatus(ctx, d.ID, "running", result.Log, "")
	_ = w.appRepo.UpdateStatus(ctx, app.ID, "running", "")

	_ = w.hub.Broadcast // broadcast done
	w.hub.Broadcast(roomID, map[string]any{
		"type":     "done",
		"duration": time.Since(start).String(),
		"image":    imageName,
	})
}

func (w *DeployWorker) failDeploy(ctx context.Context, d *model.Deployment, roomID, msg string) {
	_ = w.deployRepo.UpdateStatus(ctx, d.ID, "failed", "", msg)
	w.hub.Broadcast(roomID, map[string]any{
		"type":  "error",
		"error": msg,
	})
}
```

---

## 28. `internal/handler/ws_handler.go`

```go
package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/username/paas-backend/internal/websocket"
)

type WSHandler struct {
	hub *websocket.Hub
}

func NewWSHandler(hub *websocket.Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

func (h *WSHandler) Serve(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "appID")
	room := fmt.Sprintf("app-%s", appID)

	client, err := websocket.NewClient(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.hub.Register(room, client)
	defer func() {
		h.hub.Unregister(room, client)
		client.Close()
	}()

	client.WriteLoop(context.Background())
}
```

## 29. `internal/handler/app_handler.go` — Contoh

```go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/username/paas-backend/internal/model"
	"github.com/username/paas-backend/internal/repository"
	"github.com/username/paas-backend/internal/service"
)

type AppHandler struct {
	appSvc  *service.AppService
	appRepo *repository.AppRepository
}

func NewAppHandler(appSvc *service.AppService, appRepo *repository.AppRepository) *AppHandler {
	return &AppHandler{appSvc: appSvc, appRepo: appRepo}
}

func (h *AppHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	app, err := h.appSvc.CreateApp(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (h *AppHandler) List(w http.ResponseWriter, r *http.Request) {
	apps, err := h.appRepo.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, apps)
}

func (h *AppHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	app, err := h.appRepo.FindByID(r.Context(), uint(id))
	if err != nil || app == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
```

## 30. `internal/router/router.go`

```go
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/username/paas-backend/internal/config"
	"github.com/username/paas-backend/internal/handler"
	"github.com/username/paas-backend/internal/repository"
	"github.com/username/paas-backend/internal/service"
	"github.com/username/paas-backend/internal/websocket"
	"github.com/username/paas-backend/internal/worker"
	appmw "github.com/username/paas-backend/internal/middleware"
)

type Deps struct {
	Cfg        *config.Config
	Logger     interface{}
	AppSvc     *service.AppService
	DBService  *service.DatabaseService
	BuildSvc   *service.BuildService
	GitSvc     *service.GitService
	UploadSvc  *service.UploadService
	TraefikSvc *service.TraefikService
	CleanupSvc *service.CleanupService
	Hub        *websocket.Hub
	Queue      *worker.Queue
	AppRepo    *repository.AppRepository
	DeployRepo *repository.DeploymentRepository
	DomainRepo *repository.DomainRepository
}

func New(d Deps) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}))

	// Handlers
	appH := handler.NewAppHandler(d.AppSvc, d.AppRepo)
	wsH := handler.NewWSHandler(d.Hub)

	// ==== API v1 ====
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(appmw.Auth(d.Cfg.JWTSecret))

		r.Route("/apps", func(r chi.Router) {
			r.Get("/", appH.List)
			r.Post("/", appH.Create)
			r.Get("/{id}", appH.Get)
			r.Post("/{id}/deploy", handler.DeployHandler(d.Queue, d.DeployRepo))
		})

		r.Get("/ws/apps/{appID}", wsH.Serve)
	})

	// Webhook (tanpa auth)
	r.Post("/api/webhooks/github", handler.GitHubWebhookHandler(d.AppRepo, d.Queue, d.DeployRepo))

	// Static web (frontend) — SPA fallback
	fs := http.FileServer(http.Dir("./web/dist"))
	r.Handle("/*", fs)

	return r
}
```

> Handler `DeployHandler` dan `GitHubWebhookHandler` diimplementasikan serupa di `deploy_handler.go` dan `webhook_handler.go` (menggunakan `queue.Enqueue`).

## 31. `internal/middleware/auth.go`

```go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const userKey ctxKey = "user"

func Auth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip untuk WS (bisa pakai query token)
			if strings.HasPrefix(r.URL.Path, "/api/v1/ws") {
				next.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(auth, "Bearer ")

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userKey, token.Claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

## 32. `internal/middleware/cors.go` — Sudah dipakai langsung via `go-chi/cors`

---

## 33. `deployments/docker-compose.yml`

```yaml
version: "3.9"

services:
  paas-backend:
    build:
      context: ..
      dockerfile: deployments/Dockerfile
    container_name: paas-backend
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - APP_ENV=production
      - PORT=8080
      - MAIN_DOMAIN=${MAIN_DOMAIN}
      - CLOUDFLARE_API_TOKEN=${CLOUDFLARE_API_TOKEN}
      - CLOUDFLARE_ACCOUNT_ID=${CLOUDFLARE_ACCOUNT_ID}
      - CLOUDFLARE_ZONE_ID=${CLOUDFLARE_ZONE_ID}
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=paas
      - DB_PASSWORD=${DB_PASSWORD}
      - DB_NAME=paas_db
      - WORKSPACE_PATH=/var/lib/paas/workspace
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ../data/workspace:/var/lib/paas/workspace
    networks:
      - proxy-net

  postgres:
    image: postgres:17-alpine
    container_name: paas-postgres
    restart: unless-stopped
    environment:
      - POSTGRES_USER=paas
      - POSTGRES_PASSWORD=${DB_PASSWORD}
      - POSTGRES_DB=paas_db
    volumes:
      - paas-pg-data:/var/lib/postgresql/data
    networks:
      - proxy-net

  traefik:
    image: traefik:v3.2
    container_name: paas-traefik
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik/traefik.yml:/etc/traefik/traefik.yml:ro
      - traefik-acme:/acme
    networks:
      - proxy-net

volumes:
  paas-pg-data:
  traefik-acme:

networks:
  proxy-net:
    name: proxy-net
    driver: bridge
    attachable: true
```

## 34. `deployments/traefik/traefik.yml`

```yaml
api:
  dashboard: true
  insecure: false

entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"

providers:
  docker:
    endpoint: "unix:///var/run/docker.sock"
    exposedByDefault: false
    network: proxy-net
    watch: true

certificatesResolvers:
  cloudflare:
    acme:
      email: admin@domainanda.com
      storage: /acme/acme.json
      dnsChallenge:
        provider: cloudflare
        resolvers:
          - "1.1.1.1:53"
        delayBeforeCheck: 10

log:
  level: INFO
```

## 35. `deployments/Dockerfile`

```dockerfile
# Build stage
FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/paas-server ./cmd/server

# Runtime stage — perlu Docker CLI & pack CLI untuk build aplikasi user
FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli git bash curl && \
    curl -sSL https://github.com/buildpacks/pack/releases/latest/download/pack-linux-amd64.tgz \
    | tar -xz -C /usr/local/bin pack

COPY --from=builder /out/paas-server /usr/local/bin/paas-server
COPY web/dist /app/web/dist

WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/paas-server"]
```

## 36. `scripts/setup.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "==> Membuat network proxy-net..."
docker network create proxy-net --driver bridge --attachable 2>/dev/null || echo "sudah ada"

echo "==> Membuat direktori workspace..."
mkdir -p data/workspace/{repos,artifacts,logs}

echo "==> Copy .env.example → .env"
[ -f .env ] || cp .env.example .env

echo "==> Install pack CLI (jika belum ada)..."
command -v pack >/dev/null || {
  curl -sSL https://github.com/buildpacks/pack/releases/latest/download/pack-linux-amd64.tgz \
    | tar -xz -C /usr/local/bin pack
}

echo "==> Selesai. Jalankan: go run ./cmd/server"
```

---

## Ringkasan Perubahan dari Fase Sebelumnya

| Aspek | Fase 1–3 (Lama) | Struktur Baru |
|---|---|---|
| **Reverse proxy** | Nginx Proxy Manager | **Traefik** (Docker labels) |
| **Cloudflare pkg** | `pkg/cloudflare/` | `pkg/cloudflared/` |
| **Model** | `internal/repository/models.go` | `internal/model/app.go` |
| **Config + Constants** | Dipisah di root | `internal/config/` (keduanya) |
| **Entry point** | `main.go` di root | `cmd/server/main.go` |
| **Log streaming** | Channel saja | **WebSocket Hub** + room per app |
| **Deploy async** | Sync di handler | **Worker Queue** (`internal/worker/`) |
| **Service tambahan** | — | `git_service`, `upload_service`, `traefik_service`, `cleanup_service`, `app_service` |
| **Handler** | — | `app`, `deploy`, `webhook`, `ws` |

Semua package tetap memakai versi terbaru (Moby SDK untuk Docker, Cloudflare Go v7, Go 1.27). Kalau mau saya lengkapi **`deploy_handler.go`**, **`webhook_handler.go`**, atau implementasi `upload_service.ExtractZip` yang aman dari ZipSlip, tinggal bilang.