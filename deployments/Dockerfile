# ---- Build stage ----
FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/paas-server ./cmd/server

# ---- Runtime stage ----
# Butuh: git (clone), docker-cli (rmi image), pack (Cloud Native Buildpacks)
FROM alpine:3.20
ARG PACK_VERSION=
ARG TARGETARCH=amd64
RUN apk add --no-cache ca-certificates git docker-cli curl tzdata \
 && PACK_ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "linux-arm64" || echo "linux") \
 && if [ -z "$PACK_VERSION" ]; then PACK_VERSION=$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/buildpacks/pack/releases/latest | sed 's|.*/||'); fi \
 && curl -fsSL "https://github.com/buildpacks/pack/releases/download/${PACK_VERSION}/pack-${PACK_VERSION}-${PACK_ARCH}.tgz" \
    | tar -xz -C /usr/local/bin pack

# Salin binary tunggal yang sudah berisi embedded frontend
COPY --from=builder /out/paas-server /usr/local/bin/paas-server

WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/paas-server"]