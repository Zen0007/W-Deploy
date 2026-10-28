package websocket

import (
	"W-dev/internal/service"
	"context"
	"log"
	"strings"
	"time"
)

const (
	// Interval sampling. 4 detik = kompromi antara realtime & load Docker API.
	containerStatsInterval   = 4 * time.Second
	containerStatsRoomPrefix = "stats/user/"
)

// ContainerStatsBroadcaster mendorong stats CPU/RAM/uptime per-container
// ke semua client yang sedang menonton room user masing-masing.
type ContainerStatsBroadcaster struct {
	hub        *Hub
	apps       *service.AppService
	containers *service.ContainerService
}

func NewContainerStatsBroadcaster(hub *Hub, apps *service.AppService, containers *service.ContainerService) *ContainerStatsBroadcaster {
	return &ContainerStatsBroadcaster{hub: hub, apps: apps, containers: containers}
}

// Run memblockir sampai ctx dibatalkan Panggil di goroutine terpisah
func (b *ContainerStatsBroadcaster) Start(ctx context.Context) {
	ticker := time.NewTicker(containerStatsInterval)
	defer ticker.Stop()
	log.Printf("[stats] broadcaster mulai, interval %s", containerStatsInterval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[stats] broadcaster berhenti")
			return
		case <-ticker.C:
			b.tick(ctx)

		}
	}
}

func (b *ContainerStatsBroadcaster) tick(ctx context.Context) {
	rooms := b.hub.RoomsWithPrefix(containerStatsRoomPrefix)
	for _, room := range rooms {
		login := strings.TrimPrefix(room, containerStatsRoomPrefix)
		if login == "" {
			continue
		}
		b.pushUserStats(ctx, login, room)
	}
}

func (b *ContainerStatsBroadcaster) pushUserStats(ctx context.Context, login string, room string) {
	apps, err := b.apps.List(ctx, login)
	if err != nil {
		return
	}

	payload := make(map[uint]map[string]any, len(apps))
	for _, a := range apps {
		if a.ContainerID == "" || a.Status != "running" {
			continue
		}

		statsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		stats, err := b.containers.GetStats(statsCtx, a.ContainerID)

		cancel()
		if err != nil {
			// container mungkin baru stop = skip, jangan crash loop.
			continue
		}

		payload[a.ID] = map[string]any{
			"cpu":            roundTo1(stats.CPUPercent),
			"memory_mb":      roundTo1(float64(stats.MemBytes) / (1024 * 1024)),
			"uptime_seconds": time.Since(stats.StartedAt).Seconds(),
		}
	}

	if len(payload) == 0 {
		return
	}

	// Broadcast tidak menyimpan history — client baru tetap akan dapat
	// snapshot dari handler saat connect.
	b.hub.Broadcast(room, map[string]any{
		"type":       "stats",
		"containers": payload,
		"ts":         time.Now().UTC().Format(time.RFC3339),
	})
}

// roundTo1
func roundTo1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}

// SnapshotUserStats adalah helper publik untuk handler saat WS connect —
// kirim nilai terkini sebelum tick pertama broadcast tiba.
func (b *ContainerStatsBroadcaster) SnapshotUserStats(ctx context.Context, login string) map[string]any {
	apps, err := b.apps.List(ctx, login)
	if err != nil {
		log.Printf("[stats] snapshot %s:%v", login, err)
		return nil
	}

	payload := make(map[uint]map[string]any, len(apps))
	for _, a := range apps {
		if a.ContainerID == "" || a.Status != "running" {
			continue
		}

		statsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		stats, err := b.containers.GetStats(statsCtx, a.ContainerID)

		cancel()
		if err != nil {
			continue
		}

		payload[a.ID] = map[string]any{
			"cpu":            roundTo1(stats.CPUPercent),
			"memory_mb":      roundTo1(float64(stats.MemBytes) / (1024 * 1024)),
			"uptime_seconds": time.Since(stats.StartedAt).Seconds(),
		}
	}

	return map[string]any{
		"type":       "stats",
		"containers": payload,
		"ts":         time.Now().UTC().Format(time.RFC3339),
	}
}
