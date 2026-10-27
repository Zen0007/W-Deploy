package repository

import (
	"time"

	"gorm.io/gorm"
)

// App mewakili satu aplikasi (project) yang dikelola.
type App struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Slug        string `gorm:"uniqueIndex;size:100;not null" json:"slug"` // subdomain, ex: "myapp"
	Name        string `gorm:"size:200;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`

	SourceType string `gorm:"size:20;not null;default:'github'" json:"source_type"` // github|zip|git
	RepoURL    string `gorm:"size:500" json:"repo_url"`
	Branch     string `gorm:"size:100;default:'main'" json:"branch"`
	Buildpack  string `gorm:"size:200" json:"buildpack"` // ex: paketo, nodejs, python

	// GitHub integration
	GitHubFullName string `gorm:"size:300;index" json:"github_full_name"` // owner/repo
	WebhookID      int64  `json:"webhook_id"`                             // id webhook di GitHub (0 = belum ada)
	AutoDeploy     bool   `gorm:"default:true" json:"auto_deploy"`
	OwnerLogin     string `gorm:"size:100;index" json:"owner_login"` // GitHub login pemilik app

	// Runtime
	Port        int    `gorm:"default:3000" json:"port"`
	ContainerID string `gorm:"size:100" json:"container_id"`
	ImageTag    string `gorm:"size:200" json:"image_tag"`

	// Status
	Status  string `gorm:"size:30;not null;default:'pending'" json:"status"`
	LastErr string `gorm:"type:text" json:"last_error"`

	// Relations
	Deployments []Deployment `gorm:"foreignKey:AppID;constraint:OnDelete:CASCADE" json:"deployments,omitempty"`
	Domains     []Domain     `gorm:"foreignKey:AppID;constraint:OnDelete:CASCADE" json:"domains,omitempty"`
	EnvVars     []EnvVar     `gorm:"foreignKey:AppID;constraint:OnDelete:CASCADE" json:"env_vars,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (App) TableName() string { return "apps" }

// Deployment mencatat riwayat build & deploy.
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

	TriggeredBy string `gorm:"size:100" json:"triggered_by"` // user|webhook

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Diisi saat job dibuat (hint dari FE).
	ProjectTypeHint string `gorm:"size:20" json:"project_type_hint,omitempty"`
	// Diisi oleh worker setelah source tersedia.
	ProjectType  string `gorm:"size:20" json:"project_type,omitempty"`
	ConfigSource string `gorm:"size:20" json:"config_source,omitempty"`
	ConfigJSON   string `gorm:"type:text" json:"config_json,omitempty"`
}

func (Deployment) TableName() string { return "deployments" }

// Domain mewakili custom domain / subdomain yang terhubung ke Cloudflare Tunnel.
type Domain struct {
	ID    uint `gorm:"primaryKey" json:"id"`
	AppID uint `gorm:"index;not null" json:"app_id"`

	Hostname   string `gorm:"uniqueIndex;size:255;not null" json:"hostname"` // ex: myapp.domainanda.com
	IsPrimary  bool   `gorm:"default:false" json:"is_primary"`
	SSLEnabled bool   `gorm:"default:true" json:"ssl_enabled"`

	// Cloudflare
	CFZoneID     string `gorm:"size:64" json:"cf_zone_id"`
	CFRecordID   string `gorm:"size:64" json:"cf_record_id"`
	CFTunnelID   string `gorm:"size:64" json:"cf_tunnel_id"`
	CFTunnelName string `gorm:"size:128" json:"cf_tunnel_name"`
	CFDNSRecord  string `gorm:"size:64" json:"cf_dns_record"` // UUID DNS record

	Status    string `gorm:"size:30;not null;default:'pending'" json:"status"`
	LastError string `gorm:"type:text" json:"last_error"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Domain) TableName() string { return "domains" }

// EnvVar menyimpan environment variable per aplikasi.
type EnvVar struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	AppID uint   `gorm:"index;not null" json:"app_id"`
	Key   string `gorm:"size:200;not null" json:"key"`
	Value string `gorm:"type:text" json:"value"` // disarankan dienkripsi

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (EnvVar) TableName() string { return "env_vars" }

// internal/repository/user_token_repo.go
type UserToken struct {
	ID          uint   `gorm:"primaryKey"`
	GitHubID    int64  `gorm:"column:github_id;uniqueIndex;not null"`
	Login       string `gorm:"size:100;not null"`
	AvatarURL   string `gorm:"size:500"`
	Name        string `gorm:"size:200"`
	AccessToken string `gorm:"type:text;not null"`
	TokenType   string `gorm:"size:20"`
	Scope       string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (UserToken) TableName() string { return "user_tokens" }

// AppDatabase mencatat database terisolasi yang di-provision untuk sebuah app.
type AppDatabase struct {
	ID    uint `gorm:"primaryKey" json:"id"`
	AppID uint `gorm:"index;not null" json:"app_id"`

	Engine        string `gorm:"size:20;not null" json:"engine"`
	ContainerID   string `gorm:"size:100" json:"container_id"`
	ContainerName string `gorm:"size:200" json:"container_name"`
	VolumeName    string `gorm:"size:200" json:"volume_name"`
	Host          string `gorm:"size:200" json:"host"`
	Port          int    `json:"port"`
	Database      string `gorm:"size:200" json:"database"`
	Username      string `gorm:"size:200" json:"username"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AppDatabase) TableName() string { return "app_databases" }
