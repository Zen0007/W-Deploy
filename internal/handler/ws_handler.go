package handler

import (
	"context"
	"net/http"
	"strings"

	"W-dev/internal/middleware"
	"W-dev/internal/service"
	"W-dev/internal/websocket"
)

// WSHandler meng-upgrade koneksi ke WebSocket untuk streaming log deploy per app.
type WSHandler struct {
	hub     *websocket.Hub
	apps    *service.AppService
	origins []string
}

func NewWSHandler(hub *websocket.Hub, apps *service.AppService, corsOrigins string) *WSHandler {
	var origins []string
	for _, o := range strings.Split(corsOrigins, ",") {
		o = strings.TrimSpace(o)
		// OriginPatterns membandingkan host (tanpa skema).
		o = strings.TrimPrefix(strings.TrimPrefix(o, "https://"), "http://")
		if o != "" {
			origins = append(origins, strings.TrimRight(o, "/"))
		}
	}
	return &WSHandler{hub: hub, apps: apps, origins: origins}
}

// Serve: GET /api/ws/apps/{id}?token=<session>
func (h *WSHandler) Serve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "id tidak valid")
		return
	}
	app, err := h.apps.Get(r.Context(), id, middleware.User(r).Login)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	client, err := websocket.NewClient(w, r, h.origins)
	if err != nil {
		return // Accept sudah menulis respons error
	}
	h.hub.Register(app.Slug, client)
	defer h.hub.Unregister(app.Slug, client)

	client.Serve(context.Background())
}
