# ---- Build ----
FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/paas-server ./cmd/server

# ---- Runtime ----
FROM alpine:3.20
ARG PACK_VERSION=0.35.1
ARG TARGETARCH

RUN apk add --no-cache ca-certificates git docker-cli curl tzdata \
    && case "$TARGETARCH" in \
    amd64) PACK_ARCH=linux ;; \
    arm64) PACK_ARCH=linux-arm64 ;; \
    *) echo "unsupported TARGETARCH=$TARGETARCH" >&2; exit 1 ;; \
    esac \
    && curl -fsSL "https://github.com/buildpacks/pack/releases/download/v${PACK_VERSION}/pack-v${PACK_VERSION}-${PACK_ARCH}.tgz" \
    | tar -xz -C /usr/local/bin pack

COPY --from=builder /out/paas-server /usr/local/bin/paas-server

WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/paas-server"]