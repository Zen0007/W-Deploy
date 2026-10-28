package handler

import (
	"errors"
	"log"
	"net/http"

	"W-dev/internal/middleware"
	"W-dev/internal/service"
)

// GitHubHandler menyediakan daftar repo milik user ke frontend.
type GitHubHandler struct {
	gh     *service.GitHubService
	tokens *service.TokenService
}

func NewGitHubHandler(gh *service.GitHubService, tokens *service.TokenService) *GitHubHandler {
	return &GitHubHandler{gh: gh, tokens: tokens}
}

// Repos: GET /api/github/repos?page=1&per_page=30&type=all&sort=updated
func (h *GitHubHandler) Repos(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r)
	if u == nil || u.Login == "" {
		writeError(w, http.StatusUnauthorized, "sesi tidak valid")
		return
	}
	login := u.Login

	tok, err := h.tokens.Get(r.Context(), login)
	if err != nil {
		log.Printf("[github] DECRYPT FAILED for %s: %v", login, err)
		writeError(w, http.StatusInternalServerError,
			"gagal decrypt token: "+err.Error())
		return
	}
	if tok == "" {
		log.Printf("[github] TOKEN EMPTY for %s", login)
		writeError(w, http.StatusUnauthorized, "token GitHub tidak ada — login ulang")
		return
	}

	log.Printf("[github] FETCH for %s (token_len=%d)", login, len(tok))

	repos, err := h.gh.FetchUserRepositories(r.Context(), tok, nil)
	if err == nil {
		writeJSON(w, http.StatusOK, repos) // <- the missing success path
		return
	}

	// Error non-401 → bukan masalah auth, teruskan pesan apa adanya
	var ghErr *service.GitHubAPIError
	if !errors.As(err, &ghErr) || ghErr.StatusCode != http.StatusUnauthorized {
		log.Printf("[github] fetch failed for %s: %v", login, err)
		writeError(w, http.StatusBadGateway, "GitHub API error: "+err.Error())
		return
	}

	// === GitHub 401 Bad credentials → coba refresh token GitHub ===
	log.Printf("[github] 401 from GitHub for %s, attempting refresh", login)

	// 401 from GitHub: try refreshing once
	refreshTok, rErr := h.tokens.GetRefresh(r.Context(), login)
	if rErr != nil || refreshTok == "" {
		writeGitHubReauth(w, "GitHub membutuhkan otorisasi ulang. Silakan hubungkan ulang GitHub.")
		return
	}

	newTok, rErr := h.gh.RefreshAccessToken(r.Context(), refreshTok)
	if rErr != nil {
		log.Printf("[github] refresh failed for %s: %v", login, rErr)
		writeGitHubReauth(w, "Refresh token GitHub tidak valid. Silakan hubungkan ulang GitHub.")
		return
	}

	if sErr := h.tokens.SaveRefreshed(r.Context(), login, newTok); sErr != nil {
		log.Printf("[github] save refreshed token failed for %s: %v", login, sErr)
		// Tetap lanjut; token baru masih valid di memori untuk request ini.
	}

	// Retry SEKALI dengan token baru
	repos, err = h.gh.FetchUserRepositories(r.Context(), newTok.AccessToken, nil)
	if err != nil {
		if errors.As(err, &ghErr) && ghErr.StatusCode == http.StatusUnauthorized {
			log.Printf("[github] retry still 401 for %s", login)
			writeGitHubReauth(w, "GitHub masih menolak token hasil refresh. Silakan hubungkan ulang GitHub.")
			return
		}
		log.Printf("[github] retry failed for %s: %v", login, err)
		writeError(w, http.StatusBadGateway, "GitHub API Error: "+err.Error())
		return
	}

	log.Printf("[github] OK after refresh: %d repos for %s", len(repos), login)
	writeJSON(w, http.StatusOK, repos)
}

// writeGitHubReauth menandai bahwa user perlu reconnect OAuth GitHub.
func writeGitHubReauth(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusUnauthorized, map[string]string{
		"error":   "GITHUB_REAUTH_REQUIRED",
		"message": msg,
	})
}
