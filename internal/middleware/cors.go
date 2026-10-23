package middleware

import (
	"net/http"
	"strings"
)

// CORS mengizinkan request lintas-origin dengan credentials (cookie).
//
// origins: daftar origin dipisah koma, atau "*" untuk reflect origin apa pun.
// Meskipun "*", kita SELALU echo back Origin request (bukan literal "*")
// karena Access-Control-Allow-Credentials=true tidak kompatibel dengan "*".
func CORS(origins string) func(http.Handler) http.Handler {
	allowAll := strings.TrimSpace(origins) == "*"

	allowed := make(map[string]bool)
	if !allowAll {
		for _, o := range strings.Split(origins, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				allowed[o] = true
			}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Vary: Origin — cegah cache menyajikan respons dengan origin salah
			w.Header().Add("Vary", "Origin")

			isAllowed := origin != "" && (allowAll || allowed[origin])
			if isAllowed {
				// WAJIB echo back origin persis — jangan pakai "*"
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, X-Requested-With")
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
