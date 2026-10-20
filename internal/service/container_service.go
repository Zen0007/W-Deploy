package service

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"W-dev/internal/constants"
	"W-dev/pkg/docker"
	"W-dev/pkg/util"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// ContainerService menjalankan image hasil build sebagai container di proxy-net.
type ContainerService struct {
	docker  *docker.Client
	network string
	traefik *TraefikService
}

func NewContainerService(d *docker.Client, networkName string, t *TraefikService) *ContainerService {
	return &ContainerService{docker: d, network: networkName, traefik: t}
}

// RunSpec adalah parameter untuk menjalankan container app.
type RunSpec struct {
	AppID uint
	Slug  string
	Image string
	Port  int
	Env   []string
}

// ContainerName menghasilkan nama container deterministik.
func ContainerName(slug string) string { return constants.ContainerPrefix + slug }

// Run mengganti container lama (jika ada) dengan yang baru dan mengembalikan ID-nya.
func (s *ContainerService) Run(ctx context.Context, spec RunSpec) (string, error) {
	if !util.IsValidSlug(spec.Slug) {
		return "", fmt.Errorf("%w: %q", ErrInvalidSlug, spec.Slug)
	}
	name := ContainerName(spec.Slug)

	// Hapus container lama (jika ada). Error "not found" diabaikan.
	_ = s.Remove(ctx, name)

	resp, err := s.docker.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: name,
		Config: &container.Config{
			Image:  spec.Image,
			Env:    spec.Env,
			Labels: s.traefik.Labels(spec.AppID, spec.Slug, spec.Port),
			ExposedPorts: network.PortSet{
				network.MustParsePort(fmt.Sprintf("%d/tcp", spec.Port)): {},
			},
		},
		HostConfig: &container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
			NetworkMode:   container.NetworkMode(s.network),
			Resources: container.Resources{
				Memory:   1024 * 1024 * 1024, // 1 GB
				NanoCPUs: 1_000_000_000,      // 1 vCPU
			},
		},
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				s.network: {},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	if _, err := s.docker.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		_, _ = s.docker.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{Force: true})
		return "", fmt.Errorf("start container: %w", err)
	}

	log.Printf("[container] %s berjalan (%s)", name, util.ShortID(resp.ID))
	return resp.ID, nil
}

// Remove menghentikan lalu menghapus container berdasarkan nama/ID.
func (s *ContainerService) Remove(ctx context.Context, nameOrID string) error {
	if nameOrID == "" {
		return nil
	}
	timeout := 10
	_, _ = s.docker.ContainerStop(ctx, nameOrID, client.ContainerStopOptions{Timeout: &timeout})
	_, err := s.docker.ContainerRemove(ctx, nameOrID, client.ContainerRemoveOptions{Force: true})
	return err
}

// WaitReady mencoba koneksi TCP ke app (best-effort; hanya efektif jika backend berada di proxy-net).
func (s *ContainerService) WaitReady(ctx context.Context, slug string, port int, timeout time.Duration) error {
	addr := net.JoinHostPort(ContainerName(slug), fmt.Sprintf("%d", port))
	deadline := time.Now().Add(timeout)
	for {
		d := net.Dialer{Timeout: 2 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("app belum merespon di %s: %w", addr, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
