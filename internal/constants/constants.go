package constants

import "time"

// ==== Docker / Network ====
const (
	DockerNetworkName = "proxy-net" // network bersama Nginx Proxy Manager, tunnel, dll

	// Builder image Paketo untuk buildpack
	PaketoBuilderImage = "paketobuildpacks/builder-jammy-base:latest"
	PaketoRunImage     = "paketobuildpacks/run-jammy-base:latest"

	// Helper image
	AlpineImage = "alpine:3.20"
	NginxImage  = "nginx:alpine"
)

// ==== Port default ====
const (
	DefaultServerPort = 8080
	DefaultAppPort    = 3000 // port default aplikasi user (Node, dsb)
	DefaultPythonPort = 8000 // Django/FastAPI
	DefaultGoPort     = 8080
)

// ==== Container naming ====
const (
	ContainerPrefix = "paas-app-" // paas-app-<slug>
	NginxConfPrefix = "paas-"     // file konfigurasi nginx
	LabelAppID      = "paas.app.id"
	LabelManaged    = "paas.managed"
)

// ==== Build / Deploy ====
const (
	BuildTimeout     = 20 * time.Minute
	DeployTimeout    = 10 * time.Minute
	HealthCheckPath  = "/"
	MaxBuildLogLines = 5000
)

// ==== Deployment status ====
type DeploymentStatus string

const (
	StatusPending   DeploymentStatus = "pending"
	StatusBuilding  DeploymentStatus = "building"
	StatusDeploying DeploymentStatus = "deploying"
	StatusRunning   DeploymentStatus = "running"
	StatusFailed    DeploymentStatus = "failed"
	StatusStopped   DeploymentStatus = "stopped"
)

// ==== Domain status ====
type DomainStatus string

const (
	DomainPending  DomainStatus = "pending"
	DomainActive   DomainStatus = "active"
	DomainFailed   DomainStatus = "failed"
	DomainDisabled DomainStatus = "disabled"
)

// ==== App source type ====
type SourceType string

const (
	SourceGitHub SourceType = "github"
	SourceZip    SourceType = "zip"
	SourceGit    SourceType = "git"
)

// ==== Cloudflare ====
const (
	CFAPIBaseURL           = "https://api.cloudflare.com/client/v4"
	CFTunnelConfigFile     = "config.yml"
	CFTunnelCredentialsDir = "/etc/cloudflared"
)

// ==== Nginx Proxy Manager ====
const (
	NPMNetwork = "proxy-net"
	NPMDataDir = "/var/lib/paas/npm"
)

// ==== Paths ====
const (
	DefaultWorkspaceRoot = "/var/lib/paas/workspace"
	DefaultDataRoot      = "/var/lib/paas/data"
	ReposDirName         = "repos"
	ArtifactsDirName     = "artifacts"
	LogsDirName          = "logs"
)
