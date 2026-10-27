package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"W-dev/internal/constants"
	"W-dev/pkg/docker"
	"W-dev/pkg/util"

	"github.com/moby/moby/api/pkg/stdcopy"
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

// RunStaged menjalankan container baru dengan nama & alias sementara.
// Container lama TIDAK disentuh — masih melayani traffic.
//
// Traefik label tetap dipasang (agar saat new ready, Traefik sudah tahu).
// Traefik akan round-robin antara old & new selama keduanya up.
func (c *ContainerService) RunStaged(
	ctx context.Context, spec RunSpec, deploymentID uint,
) (containerID, tempName string, err error) {
	tempName = fmt.Sprintf("paas-app-%s-v%d", spec.Slug, deploymentID)

	// bangun RunSpec dengan name temp + tetapi pakai port/labels dari spec alsinya
	stagedSpec := spec
	stagedSpec.Slug = tempName

	// asumsi Run() meng-create dengan containerName = "paas-app-" + spec.Slug
	// adjust jiak implementasi Run()
	id, err := c.Run(ctx, stagedSpec)
	if err != nil {
		return "", "", fmt.Errorf("run staged container : %w", err)
	}

	return id, tempName, nil
}

// WaitReadyContainer menunggu container dengan host/alias tertentu bisa diakses.
// Dipakai untuk health check container baru sebelum di-promote.
func (c *ContainerService) WaitReadyContainer(
	ctx context.Context, host string, port int, timeout time.Duration,
) error {
	return c.WaitReady(ctx, host, port, timeout)
}

// Promote menghentikan & menghapus container lama, lalu me-rename container baru
// menjadi nama kanonik (paas-app-<slug>). Return error bila rename gagal.
func (c *ContainerService) Promote(
	ctx context.Context, oldContainerID, newContainerID, finalName string,
) error {
	// 1. Stop & remove old
	if oldContainerID != "" {
		if err := c.Stop(ctx, oldContainerID, 15*time.Second); err != nil {
			log.Printf("[container] promote: stop old (%s) warning: %v", oldContainerID[:12], err)
		}
		if err := c.Remove(ctx, oldContainerID); err != nil {
			log.Printf("[container] promote: remove old (%s) warning: %v", oldContainerID[:12], err)
		}
	}

	// 2. Rename new → finalName (docker rename preserves labels)
	if err := c.Rename(ctx, newContainerID, finalName); err != nil {
		return fmt.Errorf("promote: rename new container: %w", err)
	}
	log.Printf("[container] promoted %s → %s", newContainerID[:12], finalName)

	return nil
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

// StreamLogs men-stream log stdout & stderr dari container Docker runtime.
func (s *ContainerService) StreamLogs(ctx context.Context, nameOrID string, tail string, follow bool) (io.ReadCloser, error) {
	if nameOrID == "" {
		return nil, fmt.Errorf("nama atau ID container kosong")
	}
	opts := client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       tail,
	}
	res, err := s.docker.ContainerLogs(ctx, nameOrID, opts)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer res.Close()
		_, _ = stdcopy.StdCopy(pw, pw, res)
	}()
	return pr, nil
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

func (s *ContainerService) Stop(ctx context.Context, id string, timeout time.Duration) error {
	secs := int(timeout.Seconds())
	_, err := s.docker.Client.ContainerStop(ctx, id, client.ContainerStopOptions{
		Timeout: &secs,
	})
	return err
}

func (s *ContainerService) Rename(ctx context.Context, id, newName string) error {
	_, err := s.docker.ContainerRename(ctx, id, client.ContainerRenameOptions{
		NewName: newName,
	})
	return err
}
