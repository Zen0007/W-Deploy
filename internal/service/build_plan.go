package service

import (
	"context"
	"time"

	"W-dev/internal/projectconfig"
)

// BuildInput adalah masukan ke BuildStrategy.
type BuildInput struct {
	SourcePath string
	AppSlug    string
	ImageName  string // tag image final, mis. "paas/blog-native:42"
	Port       int    // port container harus listen (Traefik target)
	BuildEnv   map[string]string
	LogChannel chan<- string // opsional; bila nil strategy akan log internal
	// ResolvedConfig hasil Phase 2
	Config *projectconfig.ResolveConfig
}

// BuildResult adalah output dari satu strategi build.
// RuntimePort adalah port yang DI-GUNAKAN container di dalam image —
// worker memakai ini saat men-generate label Traefik.
type BuildPlanResult struct {
	ImageName   string
	RuntimePort int
	Success     bool
	Duration    time.Duration
	Strategy    string // "static" | "spa" | "node" | "custom"
	Error       string
}

// BuildStrategy adalah kontrak setiap handler kategori.
type BuildStrategy interface {
	Name() string
	Build(ctx context.Context, in BuildInput) (*BuildPlanResult, error)
}
