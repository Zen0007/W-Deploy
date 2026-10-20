package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"W-dev/pkg/util"
)

// SessionCookie adalah nama cookie session.
const SessionCookie = "paas_session"

type ctxKey struct{}

// RequireAuth memvalidasi session token dari header Authorization: Bearer, cookie,
// atau (hanya jika allowQuery) parameter ?token= — dipakai untuk WebSocket karena
// browser tidak bisa mengirim header kustom saat membuka WS.
func RequireAuth(secret string, allowQuery bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r, allowQuery)
			if token == "" {
				unauthorized(w, "token tidak ada")
				return
			}
			claims, err := util.VerifySession(secret, token)
			if err != nil {
				unauthorized(w, err.Error())
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// User mengambil claims user dari context (nil jika tidak ada).
func User(r *http.Request) *util.SessionClaims {
	c, _ := r.Context().Value(ctxKey{}).(*util.SessionClaims)
	return c
}

func extractToken(r *http.Request, allowQuery bool) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
		return c.Value
	}
	if allowQuery {
		return r.URL.Query().Get("token")
	}
	return ""
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
