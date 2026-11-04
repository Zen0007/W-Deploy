package config

// Runtime defaults — keep pinned, bump deliberately.
const (
	// Buildpacks
	PaketoBuilderImage = "paketobuildpacks/builder-jammy-base:0.4.420"
	PackVersion        = "0.35.1"

	// Ingress
	CloudflaredImage = "cloudflare/cloudflared:2024.10.0"
	TraefikImage     = "traefik:v3.6"
	PostgresImage    = "postgres:17-alpine"

	// Mode values — referenced from config.validate()
	IngressLocal  = "local"
	IngressDirect = "direct"
	IngressTunnel = "tunnel"

	AuthLocal  = "local"
	AuthGitHub = "github"
	AuthBoth   = "both"

	// Placeholders that must never survive into production
	PlaceholderJWT = "change-me"
)
