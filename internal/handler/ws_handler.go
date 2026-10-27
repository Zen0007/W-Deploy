package handler

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"W-dev/internal/middleware"
	"W-dev/internal/service"
	"W-dev/internal/websocket"
)

// WSHandler meng-upgrade koneksi ke WebSocket untuk streaming log deploy per app.
type WSHandler struct {
	hub        *websocket.Hub
	apps       *service.AppService
	containers *service.ContainerService
	resource   *service.ResourceService
	origins    []string
}

func NewWSHandler(hub *websocket.Hub, apps *service.AppService, containers *service.ContainerService, resource *service.ResourceService, corsOrigins string) *WSHandler {
	var origins []string
	for o := range strings.SplitSeq(corsOrigins, ",") {
		o = strings.TrimSpace(o)
		// OriginPatterns membandingkan host (tanpa skema).
		o = strings.TrimPrefix(strings.TrimPrefix(o, "https://"), "http://")
		if o != "" {
			origins = append(origins, strings.TrimRight(o, "/"))
		}
	}
	return &WSHandler{
		hub:        hub,
		apps:       apps,
		containers: containers,
		resource:   resource,
		origins:    origins,
	}
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

	log.Printf("[ws] log client terhubung (app=%s, remote=%s)", app.Slug, r.RemoteAddr)

	// Stream log runtime dari Docker container jika container tersedia
	targetContainer := app.ContainerID
	if targetContainer == "" {
		targetContainer = service.ContainerName(app.Slug)
	}
	if h.containers != nil && targetContainer != "" {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		go func() {
			logsReader, err := h.containers.StreamLogs(ctx, targetContainer, "150", true)
			if err != nil {
				return
			}
			defer logsReader.Close()
			scanner := bufio.NewScanner(logsReader)
			for scanner.Scan() {
				line := scanner.Text()
				client.Send(map[string]any{
					"type": "log",
					"line": line,
					"app":  app.Slug,
					"ts":   time.Now().UTC().Format(time.RFC3339),
				})
			}
		}()
	}

	client.Serve(r.Context())
}

// serveResourceRoom menangani pola umum: upgrade → register → snapshot awal →
// serve. `snapshot` dipanggil sekali untuk mengirim nilai terkini ke client
// yang baru connect (Broadcast tidak menyimpan history).
func (h *WSHandler) serveResourceRoom(
	w http.ResponseWriter,
	r *http.Request,
	room string,
	snapshot func() map[string]any,
) {
	client, err := websocket.NewClient(w, r, h.origins)
	if err != nil {
		return // Accept sudah menulis respons error
	}

	h.hub.Register(room, client)
	defer h.hub.Unregister(room, client)

	// Kirim snapshot awal agar UI langsung terisi tanpa menunggu tick pertama.
	if snapshot != nil {
		if data := snapshot(); data != nil {
			client.Send(data)
		}
	}

	log.Printf("[ws] resource client terhubung (room=%s, remote=%s)", room, r.RemoteAddr)

	// r.Context() dibatalkan saat client disconnect → Serve berhenti.
	client.Serve(r.Context())
}

// Resource monitoring WebSocket: GET /api/ws/resource/cpu
func (h *WSHandler) ResourceCPU(w http.ResponseWriter, r *http.Request) {
	h.serveResourceRoom(w, r, "resource/cpu", h.resource.CollectCPU)
}

// Resource monitoring WebSocket: GET /api/ws/resource/ram
func (h *WSHandler) ResourceRAM(w http.ResponseWriter, r *http.Request) {
	h.serveResourceRoom(w, r, "resource/ram", h.resource.CollectRAM)
}

// Resource monitoring WebSocket: GET /api/ws/resource/disk
func (h *WSHandler) ResourceDisk(w http.ResponseWriter, r *http.Request) {
	h.serveResourceRoom(w, r, "resource/disk", h.resource.CollectDisk)

}
