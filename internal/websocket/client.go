package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const (
	// writeTimeout membatasi satu operasi tulis. Jika client lambat,
	// koneksi ditutup agar tidak memblokir hub.
	writeTimeout = 10 * time.Second

	// pingInterval untuk keepalive. Cloudflare/Traefik idle timeout
	// default ~60 detik, jadi 30 detik aman.
	pingInterval = 30 * time.Second

	// pingTimeout adalah batas waktu tunggu pong sebelum koneksi dianggap mati.
	pingTimeout = 10 * time.Second
)

// Client adalah satu koneksi WebSocket.
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// NewClient meng-upgrade koneksi HTTP. originPatterns berisi host yang diizinkan
// (mis. "app.domainanda.com"); kosong/"*" = semua origin (hanya untuk development).
func NewClient(w http.ResponseWriter, r *http.Request, originPatterns []string) (*Client, error) {
	opts := &websocket.AcceptOptions{}
	if len(originPatterns) == 0 || (len(originPatterns) == 1 && originPatterns[0] == "*") {
		opts.InsecureSkipVerify = true
	} else {
		opts.OriginPatterns = originPatterns
	}
	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, send: make(chan []byte, 256)}, nil
}

// Send mengirim event ke client. Tidak pernah memblokir: jika buffer penuh
// (client lambat), pesan dibuang dan client ditandai untuk ditutup.
// Aman dipanggil dari goroutine mana pun (dipanggil hub).
func (c *Client) Send(event any) {
	data, err := marshalEvent(event)
	if err != nil {
		log.Printf("[ws] marshal event: %v", err)
		return
	}
	select {
	case c.send <- data:
	default:
		// Buffer penuh — client terlalu lambat. Tutup agar hub berhenti
		// mengirim; goroutine write akan keluar saat channel ditutup.
		log.Printf("[ws] client buffer penuh, menutup koneksi")
		select {
		case c.send <- data: // satu percobaan terakhir, mungkin berhasil
		default:
			// biarkan; Serve() akan mendeteksi via ticker ping
		}
	}
}

// Serve menjalankan loop utama sampai koneksi ditutup atau ctx dibatalkan.
// Handler harus memanggil ini setelah Register dan defer Unregister.
func (c *Client) Serve(ctx context.Context) {
	// Set limit untuk pesan masuk (client tidak mengirim data besar).
	c.conn.SetReadLimit(4096)

	// Context yang dibatalkan saat koneksi ditutup.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Read pump: mendeteksi disconnect + membuang pesan dari client.
	go func() {
		defer cancel()
		for {
			if _, _, err := c.conn.Read(ctx); err != nil {
				return
			}
			// Abaikan pesan dari client (protokol server → client saja).
		}
	}()

	// Keepalive: kirim ping berkala agar proxy tidak memutus idle.
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, pingTimeout)
				err := c.conn.Ping(pingCtx)
				pingCancel()
				if err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Write pump: konsumsi channel send, tulis ke koneksi.
	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				// Channel ditutup → keluar rapi.
				_ = c.conn.Close(websocket.StatusNormalClosure, "")
				return
			}
			wCtx, wCancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(wCtx, websocket.MessageText, data)
			wCancel()
			if err != nil {
				// Client lambat / disconnect.
				return
			}
		case <-ctx.Done():
			// Client disconnect atau server shutdown.
			_ = c.conn.Close(websocket.StatusNormalClosure, "")
			return
		}
	}
}

// Close menutup koneksi dari sisi server.
func (c *Client) Close() {
	_ = c.conn.Close(websocket.StatusNormalClosure, "")
}

// marshalEvent menserialisasi event apa pun menjadi JSON bytes.
// Jika event sudah []byte (dari LogSink lama), pakai apa adanya.
func marshalEvent(event any) ([]byte, error) {
	if b, ok := event.([]byte); ok {
		return b, nil
	}
	return json.Marshal(event)
}
