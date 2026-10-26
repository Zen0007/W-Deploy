package router

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"W-dev/internal/config"
	"W-dev/internal/handler"
	"W-dev/internal/middleware"
)

// Handlers mengelompokkan seluruh handler HTTP.
type Handlers struct {
	Auth    *handler.AuthHandler
	GitHub  *handler.GitHubHandler
	Deploy  *handler.DeployHandler
	App     *handler.AppHandler
	Webhook *handler.WebhookHandler
	WS      *handler.WSHandler
}

// New membangun http.Handler lengkap dengan middleware. Memakai ServeMux Go 1.22+
// (pola "METHOD /path/{param}"), jadi tidak butuh framework tambahan.
func New(embeddedFiles embed.FS, cfg *config.Config, h Handlers, staticDir string) http.Handler {
	mux := http.NewServeMux()

	auth := middleware.RequireAuth(cfg.JWTSecret, false)
	authWS := middleware.RequireAuth(cfg.JWTSecret, true) // WS: token via ?token=
	protect := func(f http.HandlerFunc) http.Handler { return auth(f) }

	// Publik
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/auth/github", h.Auth.Login)
	mux.HandleFunc("GET /api/auth/github/callback", h.Auth.Callback)
	mux.HandleFunc("POST /api/auth/refresh", h.Auth.Refresh)
	mux.HandleFunc("POST /api/webhook/github", h.Webhook.GitHub) // diamankan lewat signature HMAC

	// Terproteksi
	mux.Handle("GET /api/auth/me", protect(h.Auth.Me))
	mux.Handle("POST /api/auth/logout", protect(h.Auth.Logout))
	mux.Handle("GET /api/github/repos", protect(h.GitHub.Repos))

	mux.Handle("POST /api/deploy", protect(h.Deploy.Deploy))
	mux.Handle("GET /api/apps", protect(h.App.List))
	mux.Handle("GET /api/apps/{id}", protect(h.App.Get))
	mux.Handle("DELETE /api/apps/{id}", protect(h.App.Delete))
	mux.Handle("GET /api/apps/{id}/deployments", protect(h.App.Deployments))
	mux.Handle("POST /api/apps/{id}/redeploy", protect(h.App.Redeploy))
	mux.Handle("GET /api/apps/{id}/env", protect(h.App.ListEnv))
	mux.Handle("PUT /api/apps/{id}/env", protect(h.App.SetEnv))
	mux.Handle("DELETE /api/apps/{id}/env/{key}", protect(h.App.DeleteEnv))

	mux.Handle("GET /api/ws/apps/{id}", authWS(http.HandlerFunc(h.WS.Serve)))
	// Resource monitoring WebSocket
	mux.Handle("GET /api/ws/resource/cpu", authWS(http.HandlerFunc(h.WS.ResourceCPU)))
	mux.Handle("GET /api/ws/resource/ram", authWS(http.HandlerFunc(h.WS.ResourceRAM)))
	mux.Handle("GET /api/ws/resource/disk", authWS(http.HandlerFunc(h.WS.ResourceDisk)))

	// Frontend statis (SPA) – selalu gunakan embed
	mux.Handle("/", spaEmbedded(embeddedFiles))

	return middleware.CORS(cfg.CORSOrigins)(mux)
}

// spaEmbedded menyajikan file statis dari embed.FS dan fallback ke index.html.
func spaEmbedded(embeddedFiles embed.FS) http.Handler {
	subFS, err := fs.Sub(embeddedFiles, "web")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(subFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		p := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		// 1. Baca index.html terlebih dahulu (bisa dilakukan sekali di luar handler atau di dalam handler)
		indexBytes, err := fs.ReadFile(subFS, "index.html")
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// 2. Di dalam logika fallback / error handling Anda:
		f, err := subFS.Open(p)
		if err != nil {
			// Fallback ke index.html menggunakan bytes.NewReader (implementasi io.ReadSeeker)
			reader := bytes.NewReader(indexBytes)
			http.ServeContent(w, r, "index.html", time.Time{}, reader)
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			reader := bytes.NewReader(indexBytes)
			http.ServeContent(w, r, "index.html", time.Time{}, reader)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
