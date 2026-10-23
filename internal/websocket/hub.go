package websocket

import "sync"

const historyPerRoom = 300

// Hub mengelola koneksi client per room (room = slug app) dan menyimpan
// riwayat singkat agar client yang baru terhubung bisa melihat log yang sudah lewat.
type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[*Client]struct{}
	history map[string][]any
}

func NewHub() *Hub {
	return &Hub{
		rooms:   make(map[string]map[*Client]struct{}),
		history: make(map[string][]any),
	}
}

// Register menambahkan client ke room dan mengirimkan riwayat terakhir.
func (h *Hub) Register(room string, c *Client) {
	h.mu.Lock()
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]struct{})
	}
	h.rooms[room][c] = struct{}{}
	past := append([]any(nil), h.history[room]...)
	h.mu.Unlock()

	for _, ev := range past {
		c.Send(ev)
	}
}

// Unregister melepas client dari room.
func (h *Hub) Unregister(room string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.rooms[room]; ok {
		delete(clients, c)
		if len(clients) == 0 {
			delete(h.rooms, room)
		}
	}
}

// Publish mengirim event ke semua client di room (implementasi service.LogSink).
// Tidak pernah memblokir: client yang lambat akan kehilangan pesan.
func (h *Hub) Publish(room string, event any) {
	h.mu.Lock()
	hist := append(h.history[room], event)
	if len(hist) > historyPerRoom {
		hist = append([]any(nil), hist[len(hist)-historyPerRoom:]...)
	}
	h.history[room] = hist
	clients := make([]*Client, 0, len(h.rooms[room]))
	for c := range h.rooms[room] {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		c.Send(event)
	}
}

// ResetHistory menghapus riwayat room (dipanggil di awal deploy baru).
func (h *Hub) ResetHistory(room string) {
	h.mu.Lock()
	delete(h.history, room)
	h.mu.Unlock()
}

// Clients mengembalikan jumlah client terhubung di room.
func (h *Hub) Clients(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[room])
}

// Broadcast mengirim event ke semua client di room TANPA menyimpan history.
// Cocok untuk data realtime yang bersifat point-in-time (CPU, RAM, Disk,
// progress bar, heartbeat) — client yang baru connect tidak perlu
// menerima nilai lama yang sudah tidak relevan.
//
// Tidak pernah memblokir: client yang lambat akan kehilangan pesan (lihat Client.Send).

func (h *Hub) Broadcast(room string, event any) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.rooms[room]))
	for c := range h.rooms[room] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	// Fast-path tidak ada client,tidak perlu loop.

	if len(clients) == 0 {
		return
	}
	for _, c := range clients {
		c.Send(event)
	}
}
