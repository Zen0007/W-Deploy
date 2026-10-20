package service

import (
	"W-dev/pkg/docker"
	"W-dev/pkg/util"
	"context"
	"fmt"
	"io"
	"log"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/strslice"
	"github.com/moby/moby/client"
)

// DatabaseService menangani provisioning container database.
type DatabaseService struct {
	docker  *docker.Client
	network string // "proxy-net"
}

func NewDatabaseService(dockerCli *docker.Client, networkName string) *DatabaseService {
	return &DatabaseService{docker: dockerCli, network: networkName}
}

// Provision membuat container database baru untuk aplikasi.
func (s *DatabaseService) Provision(ctx context.Context, req DBProvisionRequest) (*DBProvisionResult, error) {
	// 1. Validasi input
	if !util.IsValidSlug(req.AppSlug) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidSlug, req.AppSlug)
	}
	spec, ok := dbSpecs[req.Engine]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrEngineNotSupported, req.Engine)
	}

	// 2. Generate password
	password, err := util.GeneratePassword(32)
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}

	// 3. Resolve identity SEKALI (user & dbName) untuk env + URI
	user, dbName, err := resolveIdentity(req.Engine, req)
	if err != nil {
		return nil, err
	}

	// 4. Nama resource
	version := req.Version
	if version == "" {
		version = spec.DefaultVer
	}
	image := fmt.Sprintf("%s:%s", spec.Image, version)
	containerName := fmt.Sprintf("paas-db-%s-%s", req.AppSlug, req.Engine)
	volumeName := fmt.Sprintf("paas-db-%s-%s-data", req.AppSlug, req.Engine)
	hostAlias := fmt.Sprintf("%s-db", req.AppSlug)

	// 5. Fail-fast jika container sudah ada (cegah overwrite / bug rollback)
	if _, err := s.docker.ContainerInspect(ctx, containerName, client.ContainerInspectOptions{}); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrAlreadyProvisioned, containerName)
	}

	// 6. Pastikan image tersedia
	if err := s.ensureImage(ctx, image); err != nil {
		return nil, err
	}

	// 7. Volume — track apakah benar-benar baru dibuat (rollback safety)
	volumeCreated, err := s.ensureVolume(ctx, volumeName)
	if err != nil {
		return nil, fmt.Errorf("buat volume: %w", err)
	}
	rollbackVolume := func() {
		if volumeCreated {
			_, _ = s.docker.VolumeRemove(ctx, volumeName, client.VolumeRemoveOptions{Force: true})
		}
	}

	// 8. Command (khusus Redis)
	var cmd strslice.StrSlice
	if req.Engine == DBRedis {
		cmd = strslice.StrSlice{
			"redis-server",
			"--requirepass", password,
			"--appendonly", "yes",
		}
	}

	// 9. Env
	envVars := spec.EnvFunc(user, dbName, password)

	// 10. Create container
	resp, err := s.docker.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: containerName,
		Config: &container.Config{
			Image: image,
			Env:   envVars,
			Cmd:   cmd,
			ExposedPorts: network.PortSet{
				network.MustParsePort(fmt.Sprintf("%d/tcp", spec.Port)): {},
			},
			Healthcheck: spec.HealthCheck, // ✅ di Config, bukan top-level
			Labels: map[string]string{
				"paas.managed":   "true",
				"paas.app.id":    fmt.Sprintf("%d", req.AppID),
				"paas.db.engine": string(req.Engine),
				"paas.db.role":   "database",
				"paas.db.volume": volumeName,
			},
		},
		HostConfig: &container.HostConfig{
			Binds:         []string{fmt.Sprintf("%s:%s", volumeName, spec.VolumePath)},
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
			NetworkMode:   container.NetworkMode(s.network),
			Resources: container.Resources{
				Memory:   512 * 1024 * 1024, // 512 MB
				NanoCPUs: 1_000_000_000,     // 1 vCPU
			},
		},
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				s.network: {
					Aliases: []string{hostAlias, string(req.Engine)},
				},
			},
		},
	})
	if err != nil {
		rollbackVolume()
		return nil, fmt.Errorf("create container: %w", err)
	}

	// 11. Start
	if _, err := s.docker.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		_, _ = s.docker.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{Force: true})
		rollbackVolume()
		return nil, fmt.Errorf("start container: %w", err)
	}

	log.Printf("[db] container %s (%s) berjalan", containerName, util.ShortID(resp.ID))

	// 12. Connection URI
	uri := buildConnectionURI(req.Engine, user, password, hostAlias, spec.Port, dbName)

	return &DBProvisionResult{
		ContainerID:   resp.ID,
		ContainerName: containerName,
		Engine:        req.Engine,
		Host:          hostAlias,
		Port:          spec.Port,
		Database:      dbName,
		Username:      user,
		Password:      password,
		ConnectionURI: uri,
		VolumeName:    volumeName,
	}, nil
}

// Deprovision menghapus container, volume, dan semua resource terkait.
func (s *DatabaseService) Deprovision(ctx context.Context, containerID, volumeName string) error {
	if containerID == "" {
		return fmt.Errorf("containerID kosong")
	}

	// 1. Stop — Timeout dalam DETIK (bukan nanodetik)
	stopTimeout := 10
	if _, err := s.docker.ContainerStop(ctx, containerID,
		client.ContainerStopOptions{Timeout: &stopTimeout}); err != nil {
		log.Printf("[db] warning: gagal stop %s: %v", util.ShortID(containerID), err)
	}

	// 2. Remove CONTAINER (bukan VolumeRemove!)
	if _, err := s.docker.ContainerRemove(ctx, containerID,
		client.ContainerRemoveOptions{Force: true, RemoveVolumes: false}); err != nil {
		return fmt.Errorf("gagal remove container: %w", err)
	}

	// 3. Remove volume
	if volumeName != "" {
		if _, err := s.docker.VolumeRemove(ctx, volumeName,
			client.VolumeRemoveOptions{Force: true}); err != nil {
			return fmt.Errorf("gagal remove volume %s: %w", volumeName, err)
		}
	}

	log.Printf("[db] container %s & volume %s dihapus",
		util.ShortID(containerID), volumeName)
	return nil
}

// ---- helpers ----

// ensureImage memastikan image tersedia lokal (pull jika belum).
func (s *DatabaseService) ensureImage(ctx context.Context, image string) error {
	if _, err := s.docker.ImageInspect(ctx, image); err == nil {
		return err
	}
	rc, err := s.docker.ImagePull(ctx, image, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", image, err)
	}
	defer rc.Close()
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("baca stream pull %s: %w", image, err)
	}
	return nil
}

// ensureVolume membuat volume jika belum ada.
// Return true HANYA jika volume baru dibuat (untuk rollback yang aman).
func (s *DatabaseService) ensureVolume(ctx context.Context, name string) (bool, error) {
	if _, err := s.docker.VolumeInspect(ctx, name, client.VolumeInspectOptions{}); err == nil {
		return false, nil
	}
	_, err := s.docker.VolumeCreate(ctx, client.VolumeCreateOptions{
		Name:   name,
		Labels: map[string]string{"paas.managed": "true"},
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
