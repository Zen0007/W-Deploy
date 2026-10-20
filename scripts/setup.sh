#!/usr/bin/env bash
# Menyiapkan lingkungan: network Docker, direktori workspace, .env, dan CLI `pack`.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "==> Memeriksa Docker..."
command -v docker >/dev/null || { echo "Docker belum terpasang"; exit 1; }
docker info >/dev/null 2>&1 || { echo "Docker daemon tidak berjalan / tidak ada izin akses"; exit 1; }

echo "==> Membuat network proxy-net..."
docker network inspect proxy-net >/dev/null 2>&1 \
  || docker network create proxy-net --driver bridge --attachable

echo "==> Membuat direktori workspace..."
mkdir -p data/workspace/{repos,artifacts,logs,uploads}

echo "==> Menyiapkan .env..."
if [ ! -f .env ]; then
  cp .env.example .env
  # Isi secret acak agar tidak memakai default
  sed -i "s|^JWT_SECRET=.*|JWT_SECRET=$(openssl rand -hex 32)|" .env
  sed -i "s|^GITHUB_WEBHOOK_SECRET=.*|GITHUB_WEBHOOK_SECRET=$(openssl rand -hex 32)|" .env
  sed -i "s|^DB_PASSWORD=.*|DB_PASSWORD=$(openssl rand -hex 16)|" .env
  echo "    .env dibuat. Lengkapi CLOUDFLARE_*, GITHUB_CLIENT_ID/SECRET, MAIN_DOMAIN."
else
  echo "    .env sudah ada, dilewati."
fi

echo "==> Memasang pack CLI (Cloud Native Buildpacks)..."
if command -v pack >/dev/null; then
  echo "    pack sudah ada: $(pack --version)"
else
  ARCH=$(uname -m); case "$ARCH" in x86_64) P=linux ;; aarch64|arm64) P=linux-arm64 ;; *) echo "arsitektur $ARCH tidak didukung"; exit 1 ;; esac
  VER=$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/buildpacks/pack/releases/latest | sed 's|.*/||')
  TMP=$(mktemp -d)
  curl -fsSL "https://github.com/buildpacks/pack/releases/download/${VER}/pack-${VER}-${P}.tgz" -o "$TMP/pack.tgz"
  tar -xzf "$TMP/pack.tgz" -C "$TMP"
  sudo install -m 0755 "$TMP/pack" /usr/local/bin/pack
  rm -rf "$TMP"
  echo "    pack terpasang: $(pack --version)"
fi

echo "==> Pre-pull builder image..."
docker pull paketobuildpacks/builder-jammy-base:latest >/dev/null

cat <<MSG

Selesai. Langkah berikutnya:
  1. Edit .env  (MAIN_DOMAIN, CLOUDFLARE_*, GITHUB_CLIENT_ID/SECRET)
  2. Jalankan stack lengkap :  docker compose -f deployments/docker-compose.yml --env-file .env up -d --build
     atau backend saja (dev):  go run ./cmd/server
MSG
