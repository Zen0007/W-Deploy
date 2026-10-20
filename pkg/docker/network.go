package docker

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/moby/moby/client"
)

// EnsureNetwork memastikan bridge network `name` sudah ada.
// Jika belum ada, network akan dibuat otomatis.
// Idempotent — aman dipanggil berkali-kali.
func (c *Client) EnsureNetwork(ctx context.Context, name string) (string, error) {
	// 1. Cek apakah network sudah ada
	exists, err := c.NetworkExists(ctx, name)
	if err != nil {
		return "", fmt.Errorf("gagal list network: %w", err)
	}
	if exists {
		return "", nil
	}

	// 2. buat network baru jika belum ada
	resp, err := c.NetworkCreate(ctx, name, client.NetworkCreateOptions{
		Driver:     "bridge",
		Attachable: true,
		Labels: map[string]string{
			"paas.managed": "true",
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Printf("[docker] network %s sudah di buat process lain", name)
			return name, nil
		}
		return "", fmt.Errorf("gagal membuat network %s: %w", name, err)
	}

	log.Printf("[docker] network %s dibuat (id=%s)", name, resp.ID[:min(12, len(resp.ID))])
	return resp.ID, nil
}

func (c *Client) NetworkExists(ctx context.Context, name string) (bool, error) {
	nets, err := c.NetworkList(ctx, client.NetworkListOptions{
		Filters: make(client.Filters).Add("name", name),
	})
	if err != nil {
		return false, err
	}
	for _, v := range nets.Items {
		if v.Name == name {
			log.Printf("%s => %s \n (id=%s)", v.Name, name, v.ID[:12])
			return true, nil
		}
	}
	return false, nil
}
