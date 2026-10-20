package middleware

import (
	"net/http"
	"strings"
)

// CORS mengizinkan origin tertentu (dipisah koma) atau "*" untuk semua.
// Dengan origin spesifik, credentials (cookie) ikut diizinkan.
func CORS(origins string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	all := false
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o == "*" {
			all = true
		} else if o != "" {
			allowed[strings.TrimRight(o, "/")] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				switch {
				case all:
					w.Header().Set("Access-Control-Allow-Origin", "*")
				case allowed[origin]:
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Add("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
