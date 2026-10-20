package handler

import (
	"log"
	"net/http"
	"strconv"
	"strings"

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
	claims := middleware.User(r)
	token, err := h.tokens.Get(r.Context(), claims.Login)
	if err != nil {
		log.Printf("[github] baca token: %v", err)
		writeError(w, http.StatusInternalServerError, "gagal membaca token GitHub")
		return
	}
	if token == "" {
		writeError(w, http.StatusUnauthorized, "token GitHub tidak ada, login ulang dengan GitHub")
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))
	opts := &service.ListRepositoriesOptions{
		Type:      allow(q.Get("type"), "all", "owner", "public", "private", "member"),
		Sort:      allow(q.Get("sort"), "created", "updated", "pushed", "full_name"),
		Direction: allow(q.Get("direction"), "asc", "desc"),
		PerPage:   perPage,
		Page:      page,
	}

	repos, err := h.gh.FetchUserRepositories(r.Context(), token, opts)
	if err != nil {
		log.Printf("[github] repos: %v", err)
		if strings.Contains(err.Error(), "HTTP 401") {
			writeError(w, http.StatusUnauthorized, "token GitHub tidak valid/kedaluwarsa, login ulang")
			return
		}
		writeError(w, http.StatusBadGateway, "gagal mengambil repository dari GitHub")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": repos, "page": opts.Page, "per_page": opts.PerPage})
}

// allow mengembalikan v hanya jika termasuk whitelist (selain itu "").
func allow(v string, ok ...string) string {
	for _, o := range ok {
		if v == o {
			return v
		}
	}
	return ""
}
