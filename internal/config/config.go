package config

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds every environment-driven setting.
type Config struct {
	// Server
	Port     string
	Env      string // development | production
	LogLevel string

	// Public URL used for OAuth callbacks, webhook URLs, emails, redirects.
	// Always normalized without a trailing slash.
	AppURL string

	// Ingress
	IngressMode string // local | direct | tunnel
	MainDomain  string
	ServerIP    string

	// TLS (direct mode only)
	ACMEEmail       string
	ACMEDNSProvider string // cloudflare | route53 | digitalocean | ...
	ACMEDNSAPIToken string

	// Cloudflare tunnel (tunnel mode only)
	CloudflareTunnelToken string

	// Cloudflare API — only used by per-app DNS auto-provisioning.
	CloudflareAPIToken  string
	CloudflareAccountID string
	CloudflareZoneID    string
	DNSProvider         string // cloudflare | none

	// Workspace
	WorkspacePath string
	DataPath      string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Auth
	AuthMethod             string // local | github | both
	BootstrapAdminEmail    string
	BootstrapAdminPassword string

	// GitHub (only required when AuthMethod ∈ {github,both})
	GitHubClientID       string
	GitHubClientSecret   string
	GitHubWebhookSecret  string
	GitHubWebhookBaseURL string
	GitHubRedirectURL    string
	FrontendURL          string // kept for compat; derived from AppURL when empty

	// Security
	JWTSecret       string
	SessionTTL      time.Duration
	RefreshTokenTTL time.Duration

	// Worker & Build
	WorkerCount  int
	QueueSize    int
	BuilderImage string
	CORSOrigins  string

	// Traefik
	TraefikTLS          bool
	TraefikCertResolver string

	// Cookie / session scoping (derived from AppURL when unset)
	CookieDomain string // "" for localhost / IP; ".domain.tld" for real domains
	CookieSecure bool

	// Docker network shared with the compose stack
	DockerNetworkName string
}

// Load reads .env (optional) and returns a validated Config.
func Load() (*Config, error) {
	_ = godotenv.Load()

	appURL := strings.TrimRight(getEnv("APP_URL", ""), "/")

	cfg := &Config{
		Port:     getEnv("PORT", "8080"),
		Env:      getEnv("APP_ENV", "development"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
		AppURL:   appURL,

		IngressMode: strings.ToLower(getEnv("INGRESS_MODE", IngressLocal)),
		MainDomain:  getEnv("MAIN_DOMAIN", "localhost"),
		ServerIP:    getEnv("SERVER_IP", "127.0.0.1"),

		ACMEEmail:       getEnv("ACME_EMAIL", ""),
		ACMEDNSProvider: getEnv("ACME_DNS_PROVIDER", "cloudflare"),
		ACMEDNSAPIToken: getEnv("ACME_DNS_API_TOKEN", ""),

		CloudflareTunnelToken: getEnv("CLOUDFLARE_TUNNEL_TOKEN", ""),

		CloudflareAPIToken:  getEnv("CLOUDFLARE_API_TOKEN", ""),
		CloudflareAccountID: getEnv("CLOUDFLARE_ACCOUNT_ID", ""),
		CloudflareZoneID:    getEnv("CLOUDFLARE_ZONE_ID", ""),
		DNSProvider:         getEnv("DNS_PROVIDER", "none"),

		WorkspacePath: getEnv("WORKSPACE_PATH", "./data/paas/workspace"),
		DataPath:      getEnv("DATA_PATH", "./data/paas/data"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "paas"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "paas_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		AuthMethod:             strings.ToLower(getEnv("AUTH_METHOD", AuthLocal)),
		BootstrapAdminEmail:    getEnv("BOOTSTRAP_ADMIN_EMAIL", ""),
		BootstrapAdminPassword: getEnv("BOOTSTRAP_ADMIN_PASSWORD", ""),

		GitHubClientID:       getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:   getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubWebhookSecret:  getEnv("GITHUB_WEBHOOK_SECRET", ""),
		GitHubWebhookBaseURL: getEnv("GITHUB_WEBHOOK_BASE_URL", ""),

		FrontendURL: getEnv("FRONTEND_URL", ""),

		JWTSecret:       getEnv("JWT_SECRET", PlaceholderJWT),
		SessionTTL:      time.Duration(getEnvInt("SESSION_TTL_HOURS", 24*7)) * time.Hour,
		RefreshTokenTTL: time.Duration(getEnvInt("REFRESH_TOKEN_TTL_HOURS", 48)) * time.Hour,

		WorkerCount:  getEnvInt("WORKER_COUNT", 2),
		QueueSize:    getEnvInt("QUEUE_SIZE", 64),
		BuilderImage: getEnv("BUILDER_IMAGE", PaketoBuilderImage),
		CORSOrigins:  getEnv("CORS_ORIGINS", ""),

		TraefikTLS:          getEnvBool("TRAEFIK_TLS", cfgDefaultTLS(appURL)),
		TraefikCertResolver: getEnv("TRAEFIK_CERT_RESOLVER", "cloudflare"),
	}

	// Derived values
	if cfg.AppURL == "" {
		switch cfg.IngressMode {
		case IngressLocal:
			cfg.AppURL = "http://localhost:8080"
		default:
			cfg.AppURL = "https://" + cfg.MainDomain
		}
	}
	cfg.CookieSecure = strings.HasPrefix(cfg.AppURL, "https://")

	if cfg.CookieDomain == "" {
		if u, err := url.Parse(cfg.AppURL); err == nil {
			host := u.Hostname()
			if net.ParseIP(host) == nil && host != "localhost" &&
				strings.Contains(host, ".") {
				if cfg.MainDomain != "" && cfg.MainDomain != "localhost" &&
					strings.HasSuffix(host, cfg.MainDomain) {
					cfg.CookieDomain = "." + cfg.MainDomain
				} else {
					cfg.CookieDomain = host
				}
			}
		}
	}

	cfg.DockerNetworkName = getEnv("DOCKER_NETWORK", "proxy-net")
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = cfg.AppURL
	}
	if cfg.GitHubWebhookBaseURL == "" {
		cfg.GitHubWebhookBaseURL = cfg.AppURL
	}
	if cfg.GitHubRedirectURL == "" {
		cfg.GitHubRedirectURL = cfg.AppURL + "/api/auth/github/callback"
	}
	if cfg.CORSOrigins == "" {
		cfg.CORSOrigins = cfg.AppURL
	}

	if err := cfg.validate(); err != nil {
		log.Printf("[config] invalid configuration: %v", err)
		return nil, err
	}
	return cfg, nil
}

// validate runs per-mode checks: nothing unrelated to the chosen mode is required.
func (c *Config) validate() error {
	var missing []string
	require := func(name, value string) {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}

	// Always required
	require("DB_PASSWORD", c.DBPassword)

	// Mode sanity
	switch c.IngressMode {
	case IngressLocal, IngressDirect, IngressTunnel:
	default:
		return fmt.Errorf("INGRESS_MODE must be one of local|direct|tunnel, got %q", c.IngressMode)
	}
	switch c.AuthMethod {
	case AuthLocal, AuthGitHub, AuthBoth:
	default:
		return fmt.Errorf("AUTH_METHOD must be one of local|github|both, got %q", c.AuthMethod)
	}

	// Placeholder secret: reject in production, warn in dev
	if c.JWTSecret == "" || c.JWTSecret == PlaceholderJWT {
		if c.IsProduction() {
			return fmt.Errorf("JWT_SECRET must be set (run scripts/setup.sh, then restart)")
		}
		missing = append(missing, "JWT_SECRET")
	}

	// Ingress mode specific
	switch c.IngressMode {
	case IngressDirect:
		require("MAIN_DOMAIN", c.MainDomain)
		require("ACME_EMAIL", c.ACMEEmail)
		if c.ACMEDNSProvider == "cloudflare" {
			require("ACME_DNS_API_TOKEN", c.ACMEDNSAPIToken)
		}
	case IngressTunnel:
		require("MAIN_DOMAIN", c.MainDomain)
		require("CLOUDFLARE_TUNNEL_TOKEN", c.CloudflareTunnelToken)
		// In tunnel mode, Cloudflare terminates TLS — backend must not run ACME
		if c.TraefikTLS {
			return fmt.Errorf("TRAEFIK_TLS must be false when INGRESS_MODE=tunnel")
		}
	}

	// Optional DNS auto-provisioning
	if c.DNSProvider == "cloudflare" {
		require("CLOUDFLARE_API_TOKEN", c.CloudflareAPIToken)
		require("CLOUDFLARE_ZONE_ID", c.CloudflareZoneID)
	}

	// Auth mode specific
	switch c.AuthMethod {
	case AuthLocal:
		require("BOOTSTRAP_ADMIN_EMAIL", c.BootstrapAdminEmail)
		require("BOOTSTRAP_ADMIN_PASSWORD", c.BootstrapAdminPassword)
	case AuthGitHub, AuthBoth:
		require("GITHUB_CLIENT_ID", c.GitHubClientID)
		require("GITHUB_CLIENT_SECRET", c.GitHubClientSecret)
		require("GITHUB_WEBHOOK_SECRET", c.GitHubWebhookSecret)
		if c.AuthMethod == AuthBoth {
			require("BOOTSTRAP_ADMIN_EMAIL", c.BootstrapAdminEmail)
			require("BOOTSTRAP_ADMIN_PASSWORD", c.BootstrapAdminPassword)
		}
	}

	// CORS in production must not be a wildcard when cookies are used
	if c.IsProduction() && strings.Contains(c.CORSOrigins, "*") {
		return fmt.Errorf("CORS_ORIGINS must not be '*' in production")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars for mode=%s auth=%s: %s",
			c.IngressMode, c.AuthMethod, strings.Join(missing, ", "))
	}
	return nil
}

// --- helpers used by the rest of the app ---

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

func (c *Config) IsProduction() bool { return strings.EqualFold(c.Env, "production") }

func (c *Config) IsLocal() bool  { return c.IngressMode == IngressLocal }
func (c *Config) IsDirect() bool { return c.IngressMode == IngressDirect }
func (c *Config) IsTunnel() bool { return c.IngressMode == IngressTunnel }

func (c *Config) LocalAuthEnabled() bool {
	return c.AuthMethod == AuthLocal || c.AuthMethod == AuthBoth
}
func (c *Config) GitHubEnabled() bool {
	return c.AuthMethod == AuthGitHub || c.AuthMethod == AuthBoth
}
func (c *Config) CloudflareDNSEnabled() bool { return c.DNSProvider == "cloudflare" }

// --- primitive readers ---

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		if v == "" {
			return fallback
		}
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func cfgDefaultTLS(appURL string) bool {
	return strings.HasPrefix(appURL, "https://")
}
