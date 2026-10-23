package handler

import (
	"log"
	"net/http"
	"slices"

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
	login := middleware.User(r).Login
	if login == "" {
		writeError(w, http.StatusUnauthorized, "sesi tidak valid")
		return
	}

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
	if err != nil {
		log.Printf("[github] FETCH FAILED for %s: %v", login, err)
		writeError(w, http.StatusBadGateway, "GitHub: "+err.Error())
		return
	}

	log.Printf("[github] OK %d repos for %s", len(repos), login)
	writeJSON(w, http.StatusOK, repos)
}

// allow mengembalikan v hanya jika termasuk whitelist (selain itu "").
func allow(v string, ok ...string) string {
	if slices.Contains(ok, v) {
		return v
	}
	return ""
}
