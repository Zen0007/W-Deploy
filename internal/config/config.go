package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"W-dev/internal/constants"

	"github.com/joho/godotenv"
)

// Config menyimpan seluruh konfigurasi aplikasi dari environment variable
type Config struct {
	// Server
	Port     string
	Env      string // development | production
	LogLevel string

	// Domain
	MainDomain string // contoh: domainanda.com
	ServerIP   string

	// Cloudflare
	CloudflareAPIToken  string
	CloudflareAccountID string
	CloudflareZoneID    string
	TunnelID            string

	// Workspace
	WorkspacePath string // path lokal untuk build & artifact
	DataPath      string // path untuk data persisten (postgres, dsb)

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// GitHub
	GitHubClientID       string
	GitHubClientSecret   string
	GitHubWebhookSecret  string
	GitHubWebhookBaseURL string // contoh: https://api.domainanda.com
	GitHubRedirectURL    string // callback OAuth, default: <GITHUB_WEBHOOK_BASE_URL>/api/auth/github/callback
	FrontendURL          string // tujuan redirect setelah login berhasil, contoh: https://app.domainanda.com

	// Security
	JWTSecret  string // dipakai menandatangani session token & mengenkripsi access token GitHub
	SessionTTL time.Duration

	// Worker & Build
	WorkerCount  int
	QueueSize    int
	BuilderImage string
	CORSOrigins  string // dipisah koma; "*" = semua

	// Traefik (reverse proxy). Jika TLS dimatikan, TLS diterminasi di Cloudflare (tunnel -> entrypoint "web").
	TraefikTLS          bool
	TraefikCertResolver string
}

// Load membaca .env (jika ada) lalu memuat semua env variable.
func Load() (*Config, error) {
	// .env opsional; jika tidak ada file, lanjut saja
	_ = godotenv.Load()

	cfg := &Config{
		Port:     getEnv("PORT", "8080"),
		Env:      getEnv("APP_ENV", "development"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		MainDomain: getEnv("MAIN_DOMAIN", ""),
		ServerIP:   getEnv("SERVER_IP", "127.0.0.1"),

		CloudflareAPIToken:  getEnv("CLOUDFLARE_API_TOKEN", ""),
		CloudflareAccountID: getEnv("CLOUDFLARE_ACCOUNT_ID", ""),
		CloudflareZoneID:    getEnv("CLOUDFLARE_ZONE_ID", ""),
		TunnelID:            getEnv("CLOUDFLARE_TUNNEL_ID", ""), // Menggantikan TunnelName

		WorkspacePath: getEnv("WORKSPACE_PATH", "./data/paas/workspace"),
		DataPath:      getEnv("DATA_PATH", "./data/paas/data"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "paas"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "paas_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		GitHubClientID:       getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:   getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubWebhookSecret:  getEnv("GITHUB_WEBHOOK_SECRET", ""),
		GitHubWebhookBaseURL: getEnv("GITHUB_WEBHOOK_BASE_URL", ""),

		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:5173"),

		JWTSecret:  getEnv("JWT_SECRET", "change-me"),
		SessionTTL: time.Duration(getEnvInt("SESSION_TTL_HOURS", 24*7)) * time.Hour,

		WorkerCount:  getEnvInt("WORKER_COUNT", 2),
		QueueSize:    getEnvInt("QUEUE_SIZE", 64),
		BuilderImage: getEnv("BUILDER_IMAGE", constants.PaketoBuilderImage),
		CORSOrigins:  getEnv("CORS_ORIGINS", "*"),

		TraefikTLS:          getEnvBool("TRAEFIK_TLS", false),
		TraefikCertResolver: getEnv("TRAEFIK_CERT_RESOLVER", "cloudflare"),
	}
	// cfg.GitHubRedirectURL = getEnv("GITHUB_REDIRECT_URL", cfg.GitHubWebhookBaseURL+"/api/auth/github/callback")

	if err := cfg.validate(); err != nil {
		log.Printf("[peringatan] konfigurasi environment variable tidak valid: %v", err)
		return nil, err
	}
	return cfg, nil
}

// validate memastikan field wajib sudah diisi
func (c *Config) validate() error {
	required := map[string]string{
		"CLOUDFLARE_API_TOKEN":  c.CloudflareAPIToken,
		"CLOUDFLARE_ACCOUNT_ID": c.CloudflareAccountID,
		// "MAIN_DOMAIN":           c.MainDomain,
		"DB_PASSWORD": c.DBPassword,
	}
	var missing []string
	for k, v := range required {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("environment variable wajib belum diisi: %s", strings.Join(missing, ", "))
	}
	if c.IsProduction() && (c.JWTSecret == "" || c.JWTSecret == "change-me") {
		return fmt.Errorf("JWT_SECRET wajib diganti di production")
	}
	return nil
}

// DSN membentuk connection string PostgreSQL
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

// IsProduction helper
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.Env, "production")
}

// --- helper ---

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
