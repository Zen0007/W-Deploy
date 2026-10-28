package service

import (
	"context"
	"encoding/json"
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

// ContainerStatsResult menyimpan snapshot stats satu container
type ContainerStatsResult struct {
	CPUPercent float64
	MemBytes   uint64
	StartedAt  time.Time
}

func NewContainerService(d *docker.Client, networkName string, t *TraefikService) *ContainerService {
	return &ContainerService{docker: d, network: networkName, traefik: t}
}

// RunSpec adalah parameter untuk menjalankan container app.
type RunSpec struct {
	AppID         uint
	Slug          string // Slug canonical aplikasi (untuk routing Traefik)
	ContainerName string // (Opsional) Override nama container di Docker
	Image         string
	Port          int
	Env           []string
}

// ContainerName menghasilkan nama container deterministik.
func ContainerName(slug string) string { return constants.ContainerPrefix + slug }

// Run mengganti container lama (jika ada) dengan yang baru dan mengembalikan ID-nya.
func (s *ContainerService) Run(ctx context.Context, spec RunSpec) (string, error) {
	if !util.IsValidSlug(spec.Slug) {
		return "", fmt.Errorf("%w: %q", ErrInvalidSlug, spec.Slug)
	}
	// Gunakan ContainerName override jika diberikan; jika tidak, gunakan default
	name := spec.ContainerName
	if name == "" {
		name = ContainerName(spec.Slug)
	}

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
	stagedSpec.ContainerName = tempName

	// asumsi Run() meng-create dengan containerName = "paas-app-" + spec.Slug
	// adjust jiak implementasi Run()
	id, err := c.Run(ctx, stagedSpec)
	if err != nil {
		return "", "", fmt.Errorf("run staged container : %w", err)
	}

	return id, tempName, nil
}

// WaitReadyContainer: host is already a full container name/alias.
func (c *ContainerService) WaitReadyContainer(
	ctx context.Context, host string, port int, timeout time.Duration,
) error {
	return waitTCP(ctx, host, port, timeout)
}

// WaitReady: takes a slug and resolves the canonical container name.
func (s *ContainerService) WaitReady(ctx context.Context, slug string, port int, timeout time.Duration) error {
	return waitTCP(ctx, ContainerName(slug), port, timeout)
}

func waitTCP(ctx context.Context, host string, port int, timeout time.Duration) error {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
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

// GetStats mengambil snapshot CPU/memory/uptime satu container dari Docker API.
// Non-blocking: butuh ~1 detik karena Docker stats butuh delta sampling.
func (s *ContainerService) GetStats(ctx context.Context, containerID string) (*ContainerStatsResult, error) {
	if containerID == "" {
		return nil, fmt.Errorf("container ID Kosong")
	}

	// 1. Inspect -> ambil StartedAt untuk uptime
	inspect, err := s.docker.Client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("inspect: %w", err)
	}
	if inspect.Container.State == nil || !inspect.Container.State.Running {
		return nil, fmt.Errorf("container tidak running")
	}

	// 2. stats snapshot (non-streaming)
	statsReps, err := s.docker.Client.ContainerStats(ctx, containerID, client.ContainerStatsOptions{Stream: false})
	if err != nil {
		return nil, fmt.Errorf("stats: %w", err)
	}
	defer statsReps.Body.Close()

	var v container.StatsResponse
	if err := json.NewDecoder(statsReps.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode stats: %w", err)
	}

	// 3. CPU % = (cpu_delta/ system_delta) x num_cores x 100
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)

	var cpuPct float64
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPct = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	// 4. memory - kurangi cache (samakan dengan 'docker stats)
	memUsage := v.MemoryStats.Usage
	if cache, ok := v.MemoryStats.Stats["cache"]; ok && cache < memUsage {
		memUsage -= cache
	}

	startedAt, _ := time.Parse(time.RFC3339Nano, inspect.Container.State.StartedAt)

	return &ContainerStatsResult{
		CPUPercent: cpuPct,
		MemBytes:   memUsage,
		StartedAt:  startedAt,
	}, nil
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
