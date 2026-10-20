package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
func New(cfg *config.Config, h Handlers, staticDir string) http.Handler {
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

	// Frontend statis (SPA) bila tersedia
	if fi, err := os.Stat(filepath.Join(staticDir, "index.html")); err == nil && !fi.IsDir() {
		mux.Handle("/", spa(staticDir))
	}

	return middleware.CORS(cfg.CORSOrigins)(mux)
}

// spa menyajikan file statis dan fallback ke index.html untuk rute client-side.
func spa(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		p := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if fi, err := os.Stat(p); err != nil || fi.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}
