package handler

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"time"

	"W-dev/internal/config"
	"W-dev/internal/middleware"
	"W-dev/internal/service"
	"W-dev/pkg/util"
)

const stateCookie = "oauth_state"

// AuthHandler menangani login GitHub OAuth.
type AuthHandler struct {
	cfg    *config.Config
	gh     *service.GitHubService
	tokens *service.TokenService
}

func NewAuthHandler(cfg *config.Config, gh *service.GitHubService, tokens *service.TokenService) *AuthHandler {
	return &AuthHandler{cfg: cfg, gh: gh, tokens: tokens}
}

// Login: GET /api/auth/github — redirect ke halaman otorisasi GitHub.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.gh.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GITHUB_CLIENT_ID/GITHUB_CLIENT_SECRET belum dikonfigurasi")
		return
	}
	state := util.RandomHex(16)
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/api/auth",
		MaxAge:   600,
		Domain:   ".wdeploy.my.id", // ← leading dot = semua subdomain
		HttpOnly: true,
		Secure:   h.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.gh.AuthURL(h.cfg.GitHubRedirectURL, state), http.StatusFound)
}

// Refresh: GET /api/auth/refresh — perpanjang session (pakai refresh token).
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("paas_refresh")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "refresh token tidak ditemukan")
		return
	}

	claims, err := util.VerifySession(h.cfg.JWTSecret, cookie.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "refresh token tidak valid")
		return
	}

	newAccessToken, err := util.SignSession(h.cfg.JWTSecret, *claims, h.cfg.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "gagal membuat session baru")
		return
	}

	// Set ulang cookie Access Token (wajib pakai Domain yang sama)
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookie,
		Value:    newAccessToken,
		Path:     "/",
		Domain:   ".wdeploy.my.id",
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "refreshed",
		"token":  newAccessToken,
	})
}

// Callback: GET /api/auth/github/callback — tukar code, simpan token, buat session.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		h.redirectFront(w, r, "error", e)
		return
	}

	c, err := r.Cookie(stateCookie)
	state := q.Get("state")

	if err != nil {
		log.Printf("[auth] state cookie MISSING — headers=%v", r.Header)
		writeError(w, http.StatusBadRequest, "state cookie tidak ditemukan")
		return
	}
	if state == "" {
		log.Printf("[auth] state query param MISSING — url=%s", r.URL.String())
		writeError(w, http.StatusBadRequest, "state query param tidak ada")
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		log.Printf("[auth] state MISMATCH — cookie=%q query=%q", c.Value, state)
		writeError(w, http.StatusBadRequest, "state cookie tidak cocok")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: "", Path: "/api/auth", MaxAge: -1})

	tok, err := h.gh.ExchangeOAuthToken(r.Context(), q.Get("code"))
	if err != nil {
		log.Printf("[auth] exchange: %v", err)
		writeError(w, http.StatusBadGateway, "gagal menukar kode login GitHub")
		return
	}
	user, err := h.gh.FetchUser(r.Context(), tok.AccessToken)
	if err != nil {
		log.Printf("[auth] fetch user: %v", err)
		writeError(w, http.StatusBadGateway, "gagal membaca profil GitHub")
		return
	}
	if err := h.tokens.Save(r.Context(), user, tok); err != nil {
		log.Printf("[auth] save token: %v", err)
		writeError(w, http.StatusInternalServerError, "gagal menyimpan token")
		return
	}

	// 1. Generate Access Token (umur pendek, misal 15 menit)
	accessToken, err := util.SignSession(h.cfg.JWTSecret,
		util.SessionClaims{Login: user.Login, GitHubID: user.ID}, h.cfg.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "gagal membuat access session")
		return
	}

	// 2. Generate Refresh Token (umur panjang, misal 7 hari)
	refreshToken, err := util.SignSession(h.cfg.JWTSecret,
		util.SessionClaims{Login: user.Login, GitHubID: user.ID}, h.cfg.RefreshTokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "gagal membuat refresh session")
		return
	}

	// 3. Set Cookie Access Token
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookie,
		Value:    accessToken,
		Path:     "/",
		Domain:   ".wdeploy.my.id", // ← Wajib sama dengan Login
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})

	// 4. Set Refresh Token Cookie (paas_refresh)
	http.SetCookie(w, &http.Cookie{
		Name:     "paas_refresh",
		Value:    refreshToken,
		Path:     "/api/auth/refresh", // Hanya dikirim ke endpoint refresh
		Domain:   ".wdeploy.my.id",    // ← Wajib sama dengan Login
		MaxAge:   int(h.cfg.RefreshTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect ke frontend...
	h.redirectFront(w, r, "token", accessToken)

}

// Me: GET /api/auth/me — profil user yang sedang login.
// internal/handler/auth_handler.go
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.User(r) // Login + GitHubID
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "tidak terautentikasi")
		return
	}

	expiresAt := time.Now().Add(h.cfg.SessionTTL).UTC().Format(time.RFC3339)

	// Ambil data lengkap dari DB
	tok, err := h.tokens.Profile(r.Context(), claims.Login)
	if err != nil {
		// fallback: tetap kirim session claims
		writeJSON(w, http.StatusOK, map[string]any{
			"login":      claims.Login,
			"github_id":  claims.GitHubID,
			"expires_at": expiresAt,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"login":               tok.Login,
		"name":                tok.Name,
		"avatar_url":          tok.AvatarURL,
		"github_id":           tok.GitHubID,
		"github_login":        tok.Login, // alias untuk frontend
		"github_scopes":       tok.Scope,
		"github_connected_at": tok.CreatedAt.Format(time.RFC3339),
		"expires_at":          expiresAt, // <-- TAMBAHKAN INI
	})
}

// Logout: POST /api/auth/logout — hapus cookie session (token GitHub tetap tersimpan
// agar webhook & deploy otomatis tetap jalan; gunakan ?revoke=true untuk menghapusnya).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("revoke") == "true" {
		if claims := middleware.User(r); claims != nil {
			_ = h.tokens.Revoke(r.Context(), claims.Login)
		}
	}

	// 2. Hapus Access Token (paas_session)
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookie, // "paas_session"
		Value:    "",
		Path:     "/",
		Domain:   ".wdeploy.my.id", // Wajib sama dengan saat di-set
		MaxAge:   -1,               // Instruksi untuk menghapus cookie
		HttpOnly: true,
		Secure:   h.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})

	// 3. Hapus Refresh Token (paas_refresh)
	http.SetCookie(w, &http.Cookie{
		Name:     "paas_refresh",
		Value:    "",
		Path:     "/api/auth/refresh", // Wajib sama dengan Path saat di-set
		Domain:   ".wdeploy.my.id",    // Wajib sama dengan saat di-set
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})

	// 4. Kirim respons sukses
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (h *AuthHandler) redirectFront(w http.ResponseWriter, r *http.Request, key, val string) {
	front := h.cfg.FrontendURL + "/#" + key + "=" + url.QueryEscape(val)
	http.Redirect(w, r, front, http.StatusFound)
}
