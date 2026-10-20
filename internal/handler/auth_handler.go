package handler

import (
	"crypto/subtle"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"W-dev/internal/config"
	"W-dev/internal/middleware"
	"W-dev/internal/model"
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
		Name: stateCookie, Value: state, Path: "/api/auth",
		MaxAge: 600, HttpOnly: true, Secure: h.cfg.IsProduction(), SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.gh.AuthURL(h.cfg.GitHubRedirectURL, state), http.StatusFound)
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
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		writeError(w, http.StatusBadRequest, "state OAuth tidak valid")
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

	session, err := util.SignSession(h.cfg.JWTSecret,
		util.SessionClaims{Login: user.Login, GitHubID: user.ID}, h.cfg.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "gagal membuat session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: middleware.SessionCookie, Value: session, Path: "/",
		Expires: time.Now().Add(h.cfg.SessionTTL), HttpOnly: true,
		Secure: h.cfg.IsProduction(), SameSite: http.SameSiteLaxMode,
	})
	// Token juga dikirim lewat fragment (tidak masuk log server) untuk frontend lintas-origin.
	h.redirectFront(w, r, "token", session)
}

// Me: GET /api/auth/me — profil user yang sedang login.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.User(r)
	p, err := h.tokens.Profile(r.Context(), claims.Login)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user tidak ditemukan, login ulang")
		return
	}
	writeJSON(w, http.StatusOK, model.UserView{Login: p.Login, Name: p.Name, AvatarURL: p.AvatarURL})
}

// Logout: POST /api/auth/logout — hapus cookie session (token GitHub tetap tersimpan
// agar webhook & deploy otomatis tetap jalan; gunakan ?revoke=true untuk menghapusnya).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("revoke") == "true" {
		if claims := middleware.User(r); claims != nil {
			_ = h.tokens.Revoke(r.Context(), claims.Login)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: middleware.SessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (h *AuthHandler) redirectFront(w http.ResponseWriter, r *http.Request, key, val string) {
	base := strings.TrimRight(h.cfg.FrontendURL, "/")
	http.Redirect(w, r, base+"/auth/callback#"+key+"="+url.QueryEscape(val), http.StatusFound)
}
