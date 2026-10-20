package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"W-dev/internal/middleware"
	"W-dev/internal/service"
	"W-dev/pkg/util"
)

func sign(secret string, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func TestWebhookRejectsBadSignatureAndAcceptsPing(t *testing.T) {
	gh := service.NewGitHubService("id", "sec", "whsec", "https://x")
	h := NewWebhookHandler(gh, nil, nil)
	body := `{"zen":"hi"}`

	// tanpa signature
	req := httptest.NewRequest(http.MethodPost, "/api/webhook/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()
	h.GitHub(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa signature: code=%d, harap 401", rec.Code)
	}

	// ping valid
	req = httptest.NewRequest(http.MethodPost, "/api/webhook/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", sign("whsec", body))
	rec = httptest.NewRecorder()
	h.GitHub(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "pong") {
		t.Fatalf("ping: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(middleware.User(r).Login))
	})
	tok, _ := util.SignSession("s", util.SessionClaims{Login: "octocat"}, 3600e9)

	call := func(mw func(http.Handler) http.Handler, target string, hdr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		if hdr != "" {
			req.Header.Set("Authorization", hdr)
		}
		rec := httptest.NewRecorder()
		mw(next).ServeHTTP(rec, req)
		return rec
	}

	if rec := call(middleware.RequireAuth("s", false), "/x", ""); rec.Code != 401 {
		t.Errorf("tanpa token: %d", rec.Code)
	}
	if rec := call(middleware.RequireAuth("s", false), "/x", "Bearer "+tok); rec.Code != 200 || rec.Body.String() != "octocat" {
		t.Errorf("bearer valid: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(middleware.RequireAuth("s", false), "/x?token="+tok, ""); rec.Code != 401 {
		t.Errorf("query token harus ditolak di rute biasa: %d", rec.Code)
	}
	if rec := call(middleware.RequireAuth("s", true), "/x?token="+tok, ""); rec.Code != 200 {
		t.Errorf("query token harus diterima di rute WS: %d", rec.Code)
	}
	if rec := call(middleware.RequireAuth("other", false), "/x", "Bearer "+tok); rec.Code != 401 {
		t.Errorf("secret berbeda: %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := middleware.CORS("https://app.example.com")(next)

	req := httptest.NewRequest(http.MethodOptions, "/api/apps", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 204 || rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Errorf("preflight salah: %d %v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("origin asing tidak boleh diizinkan")
	}
}
