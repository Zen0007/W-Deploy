package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
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

// Send mengantrekan event (di-encode JSON). Drop jika buffer client penuh.
func (c *Client) Send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- b:
	default:
	}
}

// Serve memompa pesan keluar sampai koneksi putus atau ctx selesai.
func (c *Client) Serve(ctx context.Context) {
	defer c.conn.CloseNow()

	// CloseRead menangani frame kontrol (close/ping) dan membatalkan ctx saat client pergi.
	ctx = c.conn.CloseRead(ctx)
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.send:
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.conn.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.conn.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
