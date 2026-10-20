package docker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moby/moby/client"
)

// Client wrapper moby SDK with helper
type Client struct {
	*client.Client
}

// NewClient Init Connect to docker Daemon via /var/run/docker.sock
func NewClient() (*Client, error) {
	// 1. Fungsi yang benar adalah client.NewClient(), bukan client.New()
	// 2. client.WithAPIVersionNegotiation() TETAP DIBUTUHKAN. Jika tidak dipanggil,
	//    SDK akan menggunakan versi API default (misal 1.43), yang bisa memicu
	//    error jika Docker Daemon di server menggunakan versi yang lebih lama/baru.
	cli, err := client.New(
		client.WithHost("unix:///var/run/docker.sock"),
	)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat docker client: %w", err)
	}

	// Ping untuk memastikan daemon hidup dan socket bisa diakses
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 3. Method cli.Ping() HANYA menerima context.Context.
	//    client.PingOptions{} tidak valid pada signature fungsi Ping di Docker SDK terbaru.
	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		cli.Close() // Praktik yang sangat baik: mencegah connection/goroutine leak
		return nil, fmt.Errorf("docker daemon tidak merespon: %w", err)
	}

	log.Println("[docker] berhasil terhubung ke Docker daemon")
	return &Client{cli}, nil
}

func (c *Client) Close() error {
	return c.Client.Close()
}
