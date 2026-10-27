package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"W-dev/internal/constants"
	"W-dev/internal/repository"
	"W-dev/internal/service"
)

// WebhookHandler menerima event GitHub dan memicu deploy otomatis.
type WebhookHandler struct {
	gh   *service.GitHubService
	apps *service.AppService
	repo *repository.AppRepository
}

func NewWebhookHandler(gh *service.GitHubService, apps *service.AppService, repo *repository.AppRepository) *WebhookHandler {
	return &WebhookHandler{gh: gh, apps: apps, repo: repo}
}

type pushEvent struct {
	Ref     string `json:"ref"`
	After   string `json:"after"`
	Deleted bool   `json:"deleted"`
	Repo    struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	HeadCommit *struct {
		Message string `json:"message"`
	} `json:"head_commit"`
}

// GitHub: POST /api/webhook/github
func (h *WebhookHandler) GitHub(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 5<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "body terlalu besar / tidak terbaca")
		return
	}
	// Signature WAJIB valid: endpoint ini publik dan memicu build.
	if !h.gh.VerifyWebhookSignature(body, r.Header.Get("X-Hub-Signature-256")) {
		writeError(w, http.StatusUnauthorized, "signature webhook tidak valid")
		return
	}

	switch event := r.Header.Get("X-GitHub-Event"); event {
	case "ping":
		writeJSON(w, http.StatusOK, map[string]string{"status": "pong"})
	case "push":
		h.handlePush(w, r, body)
	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "event": event})
	}
}

func (h *WebhookHandler) handlePush(w http.ResponseWriter, r *http.Request, body []byte) {
	var ev pushEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		writeError(w, http.StatusBadRequest, "payload push tidak valid")
		return
	}
	if ev.Deleted || !strings.HasPrefix(ev.Ref, "refs/heads/") || ev.Repo.FullName == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")

	apps, err := h.repo.FindByGitHubRepo(r.Context(), ev.Repo.FullName)
	if err != nil {
		log.Printf("[webhook] cari app: %v", err)
		writeError(w, http.StatusInternalServerError, "gagal mencari app")
		return
	}

	msg := ""
	if ev.HeadCommit != nil {
		msg = ev.HeadCommit.Message
		if i := strings.IndexByte(msg, '\n'); i >= 0 {
			msg = msg[:i]
		}
	}

	triggered := 0
	for i := range apps {
		app := &apps[i]
		// Hanya auto-deploy untuk GitHub/Git source ketika push terjadi pada branch yang dikonfigurasi (default: main)
		if (app.SourceType != string(constants.SourceGitHub) && app.SourceType != string(constants.SourceGit)) || app.Branch != branch {
			continue
		}
		if _, err := h.apps.Deploy(r.Context(), app, service.DeployOptions{
			TriggeredBy: "webhook", CommitSHA: ev.After, CommitMsg: msg,
		}); err != nil {
			log.Printf("[webhook] deploy %s gagal: %v", app.Slug, err)
			continue
		}
		triggered++
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "deployments_triggered": triggered})
}
