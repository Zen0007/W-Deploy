package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"W-dev/internal/constants"
	"W-dev/internal/middleware"
	"W-dev/internal/model"
	"W-dev/internal/repository"
	"W-dev/internal/service"
)

// AppHandler: CRUD aplikasi, riwayat deploy, redeploy, dan env var.
type AppHandler struct {
	apps *service.AppService
}

func NewAppHandler(apps *service.AppService) *AppHandler { return &AppHandler{apps: apps} }

// owned memuat app + cek kepemilikan; menulis error HTTP dan mengembalikan nil jika gagal.
func (h *AppHandler) owned(w http.ResponseWriter, r *http.Request) *repository.App {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "id tidak valid")
		return nil
	}
	app, err := h.apps.Get(r.Context(), id, middleware.User(r).Login)
	if err != nil {
		writeServiceError(w, err)
		return nil
	}
	return app
}

// List: GET /api/apps
func (h *AppHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.apps.List(r.Context(), middleware.User(r).Login)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	views := make([]model.AppView, 0, len(list))
	for i := range list {
		views = append(views, h.apps.View(&list[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": views})
}

// Get: GET /api/apps/{id}
func (h *AppHandler) Get(w http.ResponseWriter, r *http.Request) {
	if app := h.owned(w, r); app != nil {
		writeJSON(w, http.StatusOK, h.apps.View(app))
	}
}

// Delete: DELETE /api/apps/{id}
func (h *AppHandler) Delete(w http.ResponseWriter, r *http.Request) {
	app := h.owned(w, r)
	if app == nil {
		return
	}
	if err := h.apps.Delete(r.Context(), app); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Deployments: GET /api/apps/{id}/deployments?limit=20
func (h *AppHandler) Deployments(w http.ResponseWriter, r *http.Request) {
	app := h.owned(w, r)
	if app == nil {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	list, err := h.apps.Deployments(r.Context(), app.ID, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deployments": list})
}

// Redeploy: POST /api/apps/{id}/redeploy (hanya source github; zip harus diunggah ulang via /api/deploy)
func (h *AppHandler) Redeploy(w http.ResponseWriter, r *http.Request) {
	app := h.owned(w, r)
	if app == nil {
		return
	}
	if app.SourceType != string(constants.SourceGitHub) {
		writeError(w, http.StatusBadRequest, "redeploy hanya untuk source GitHub; unggah ulang zip melalui POST /api/deploy")
		return
	}
	dep, err := h.apps.Deploy(r.Context(), app, service.DeployOptions{TriggeredBy: "user"})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"deployment_id": dep.ID})
}

// ListEnv: GET /api/apps/{id}/env
func (h *AppHandler) ListEnv(w http.ResponseWriter, r *http.Request) {
	app := h.owned(w, r)
	if app == nil {
		return
	}
	list, err := h.apps.ListEnv(r.Context(), app.ID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"env": list})
}

// SetEnv: PUT /api/apps/{id}/env  body: {"env": {"KEY":"value"}}
func (h *AppHandler) SetEnv(w http.ResponseWriter, r *http.Request) {
	app := h.owned(w, r)
	if app == nil {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req model.SetEnvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Env) == 0 {
		writeError(w, http.StatusBadRequest, "body harus {\"env\": {...}}")
		return
	}
	if err := h.apps.SetEnv(r.Context(), app.ID, req.Env); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved", "note": "berlaku pada deploy berikutnya"})
}

// DeleteEnv: DELETE /api/apps/{id}/env/{key}
func (h *AppHandler) DeleteEnv(w http.ResponseWriter, r *http.Request) {
	app := h.owned(w, r)
	if app == nil {
		return
	}
	if err := h.apps.DeleteEnv(r.Context(), app.ID, r.PathValue("key")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
