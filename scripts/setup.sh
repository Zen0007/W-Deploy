#!/usr/bin/env bash
set -euo pipefail

ENV_FILE=".env"
ENV_EXAMPLE=".env.example"

err()  { echo "❌ $*" >&2; exit 1; }
warn() { echo "⚠️  $*" >&2; }
info() { echo "ℹ️  $*"; }
ok()   { echo "✅ $*"; }

# --- preflight ---
command -v docker  >/dev/null || err "docker not installed"
docker compose version >/dev/null 2>&1 || err "docker compose plugin not installed"
command -v openssl >/dev/null || err "openssl not installed"
command -v curl    >/dev/null || warn "curl not installed — public IP auto-detection will be skipped"

# --- bootstrap .env ---
if [ ! -f "$ENV_FILE" ]; then
  [ -f "$ENV_EXAMPLE" ] || err "$ENV_EXAMPLE missing"
  cp "$ENV_EXAMPLE" "$ENV_FILE"
  ok ".env created from $ENV_EXAMPLE"
fi

# --- safe .env helpers (no sed, safe for any value) ---
get_env() {
  awk -v k="$1" 'index($0, k"=") == 1 { sub(/^[^=]*=/, ""); print; exit }' "$ENV_FILE"
}

set_env() {
  local key="$1" val="$2" tmp
  tmp="$(mktemp)"
  KEY="$key" VAL="$val" awk '
    BEGIN { k=ENVIRON["KEY"]; v=ENVIRON["VAL"]; done=0 }
    index($0, k"=") == 1 { print k"="v; done=1; next }
    { print }
    END { if (!done) print k"="v }
  ' "$ENV_FILE" > "$tmp"
  mv "$tmp" "$ENV_FILE"
}

ensure_secret() {
  local key="$1" len="${2:-32}" cur
  cur="$(get_env "$key")"
  if [ -z "$cur" ] || [ "$cur" = "change-me" ]; then
    set_env "$key" "$(openssl rand -hex "$len")"
    info "generated $key"
  fi
}

require_nonempty() {
  local key="$1"
  [ -n "$(get_env "$key")" ] || err "$key is required (set it in .env before re-running)"
}

detect_public_ip() {
  command -v curl >/dev/null || return 1
  local url ip
  for url in https://api.ipify.org https://ifconfig.me/ip https://icanhazip.com; do
    ip="$(curl -sf --max-time 5 "$url" 2>/dev/null | tr -d '[:space:]' || true)"
    case "$ip" in
      ""|*[!0-9.]*) continue ;;
    esac
    echo "$ip"
    return 0
  done
  return 1
}

# --- mode: env var wins, then .env, then local ---
MODE="${INGRESS_MODE:-$(get_env INGRESS_MODE)}"
MODE="${MODE:-local}"
set_env INGRESS_MODE "$MODE"

# --- auth mode ---
AUTH="$(get_env AUTH_METHOD)"; AUTH="${AUTH:-local}"
set_env AUTH_METHOD "$AUTH"

# --- idempotent secrets ---
ensure_secret JWT_SECRET 32
ensure_secret DB_PASSWORD 16
ensure_secret GITHUB_WEBHOOK_SECRET 16

# --- auth: bootstrap admin for local / both ---
case "$AUTH" in
  local|both)
    [ -n "$(get_env BOOTSTRAP_ADMIN_EMAIL)" ] \
      || err "BOOTSTRAP_ADMIN_EMAIL is required for AUTH_METHOD=$AUTH"
    if [ -z "$(get_env BOOTSTRAP_ADMIN_PASSWORD)" ]; then
      set_env BOOTSTRAP_ADMIN_PASSWORD \
        "$(openssl rand -base64 24 | tr -d '/+=' | head -c 24)"
      info "generated BOOTSTRAP_ADMIN_PASSWORD"
    fi
    ;;
esac

# --- auth: GitHub OAuth (interactive) ---
if [ "$AUTH" = "github" ] || [ "$AUTH" = "both" ]; then
  if [ -z "$(get_env GITHUB_CLIENT_ID)" ] && [ -t 0 ]; then
    echo ""
    echo "━━━ GitHub OAuth ━━━"
    echo "Create app: https://github.com/settings/applications/new"
    echo "Callback URL: $(get_env APP_URL)/api/auth/github/callback"
    read -r  -p "Client ID     : " cid
    read -rs -p "Client Secret : " csec; echo ""
    set_env GITHUB_CLIENT_ID "$cid"
    set_env GITHUB_CLIENT_SECRET "$csec"
  fi
  require_nonempty GITHUB_CLIENT_ID
  require_nonempty GITHUB_CLIENT_SECRET
fi

# --- ingress: per-mode wiring ---
case "$MODE" in
  local)
    set_env BIND_ADDR   "127.0.0.1"
    set_env HTTP_PORT   "${HTTP_PORT:-8000}"
    [ -n "$(get_env MAIN_DOMAIN)" ] || set_env MAIN_DOMAIN "localhost"
    set_env APP_URL     "http://localhost:8080"
    ;;

  direct)
    set_env BIND_ADDR   "0.0.0.0"
    set_env HTTP_PORT   "80"                    # ACME HTTP-01 must use port 80
    [ -n "$(get_env HTTPS_PORT)" ] || set_env HTTPS_PORT "443"

    # Auto-fill MAIN_DOMAIN from public IP via sslip.io if unset/localhost
    CUR_DOMAIN="$(get_env MAIN_DOMAIN)"
    if [ -z "$CUR_DOMAIN" ] || [ "$CUR_DOMAIN" = "localhost" ]; then
      info "detecting public IP for sslip.io domain…"
      if PUBIP="$(detect_public_ip)"; then
        set_env MAIN_DOMAIN "${PUBIP}.sslip.io"
        ok "MAIN_DOMAIN=${PUBIP}.sslip.io (via sslip.io)"
      else
        err "could not detect public IP. Set MAIN_DOMAIN manually in .env:
     MAIN_DOMAIN=203.0.113.42.sslip.io   ← any subdomain resolves to the IP
     MAIN_DOMAIN=yourdomain.com          ← if you own a real domain"
      fi
    fi

    set_env APP_URL "https://$(get_env MAIN_DOMAIN)"

    # Default ACME_EMAIL from the domain — LE only validates syntax
    if [ -z "$(get_env ACME_EMAIL)" ]; then
      set_env ACME_EMAIL "admin@$(get_env MAIN_DOMAIN)"
      info "ACME_EMAIL defaulted to $(get_env ACME_EMAIL)"
    fi
    ;;

  tunnel)
    require_nonempty MAIN_DOMAIN
    require_nonempty CLOUDFLARE_TUNNEL_TOKEN
    set_env BIND_ADDR "127.0.0.1"
    [ -n "$(get_env HTTP_PORT)" ] || set_env HTTP_PORT "8000"
    set_env APP_URL   "https://$(get_env MAIN_DOMAIN)"
    ;;

  *) err "unknown INGRESS_MODE=$MODE (expected local|direct|tunnel)" ;;
esac

# --- lock down & validate ---
chmod 600 "$ENV_FILE"

info "validating docker compose config (mode=$MODE)…"
docker compose \
  -f deployments/docker-compose.yml \
  -f "deployments/docker-compose.$MODE.yml" \
  config >/dev/null || err "compose validation failed"

ok "setup complete (mode=$MODE, auth=$AUTH)"

# --- summary ---
echo ""
echo "    Platform URL : $(get_env APP_URL)"
case "$MODE" in
  direct|tunnel)
    echo "    App URLs     : https://<slug>.$(get_env MAIN_DOMAIN)"
    ;;
esac
case "$AUTH" in
  local|both)
    echo "    Admin email  : $(get_env BOOTSTRAP_ADMIN_EMAIL)"
    echo "    Admin pass   : $(get_env BOOTSTRAP_ADMIN_PASSWORD)"
    ;;
esac
case "$MODE" in
  direct)
    echo ""
    echo "    Make sure ports 80 and 443 are open from the internet:"
    echo "      curl -I $(get_env APP_URL)"
    ;;
esac
echo ""
echo "    Next: make up MODE=$MODE"