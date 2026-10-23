package service

import (
	"W-dev/internal/websocket"
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// ResourceService memantau resource host (CPU, RAM, Disk) dan mengirim
// event ke WebSocket hub setiap interval.
type ResourceService struct {
	hub      *websocket.Hub
	hostProc string // contoh: /host/proc
	hostRoot string // contoh: /host/root
	interval time.Duration
}

func NewResourceService(hub *websocket.Hub) *ResourceService {
	return &ResourceService{
		hub:      hub,
		hostProc: detectHostProc(),
		hostRoot: detectHostRoot(),
		interval: 2 * time.Second,
	}
}

// detectHostProc mencari lokasi /proc host.
// Prioritas: env HOST_PROC → /host/p	roc (jika ada) → kosong (pakai /proc container).
func detectHostProc() string {
	if v := os.Getenv("HOST_PROC"); v != "" {
		return v
	}
	if _, err := os.Stat("/host/proc/stat"); err == nil {
		return "/host/proc"
	}
	return ""
}

// detectHostRoot mencari lokasi root host.
// Prioritas: env HOST_ROOT → /host/root (jika ada) → kosong (pakai / container).
func detectHostRoot() string {
	if v := os.Getenv("HOST_ROOT"); v != "" {
		return v
	}
	if _, err := os.Stat("/host/root"); err == nil {
		return "/host/root"
	}
	return ""
}

func (s *ResourceService) ctx() context.Context {
	ctx := context.Background()

	if s.hostProc == "" {
		return ctx
	}

	env := common.EnvMap{
		common.HostProcEnvKey: s.hostProc,
	}

	return context.WithValue(ctx, common.EnvKey, env)
}

// Start monitoring resource (CPU, RAM, Disk) dan mengirim event ke WebSocket hub.
func (s *ResourceService) Start(ctx context.Context) {

	warmCtx, cancel := context.WithTimeout(s.ctx(), 5*time.Second)
	defer cancel()

	if _, err := cpu.PercentWithContext(warmCtx, time.Second, false); err != nil {
		log.Printf("gopsutil: gagal membaca CPU: %v", err)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.collectAndBroadcast()
		case <-ctx.Done():
			return
		}
	}
}
func (s *ResourceService) collectAndBroadcast() {
	s.hub.Broadcast("resource/cpu", s.CollectCPU())
	s.hub.Broadcast("resource/ram", s.CollectRAM())
	s.hub.Broadcast("resource/disk", s.CollectDisk())
}

// ========================== CPU ==========================
func (s *ResourceService) CollectCPU() map[string]any {
	ctx := s.ctx()

	// Interval 0 = non-blocking, mengembalikan nilai rata-rata sejak boot.

	percent, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil || len(percent) == 0 {
		return map[string]any{
			"usage": 0.0,
			"cores": 0,
			"error": errStr(err),
		}
	}

	cores, _err := cpu.CountsWithContext(ctx, true)
	if _err != nil {
		log.Printf("gopsutil: gagal membaca jumlah core CPU: %v", _err)
	}

	return map[string]any{
		"usage": percent[0],
		"cores": cores,
	}
}

// ========================== RAM ==========================
func (s *ResourceService) CollectRAM() map[string]any {
	ctx := s.ctx()
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return map[string]any{"error": err}
	}

	swap, _ := mem.SwapMemoryWithContext(ctx)

	return map[string]any{
		"total":        vm.Total / 1024 / 1024,     // MB
		"used":         vm.Used / 1024 / 1024,      // MB
		"free":         vm.Free / 1024 / 1024,      // MB
		"available":    vm.Available / 1024 / 1024, // MB
		"percent":      vm.UsedPercent,             // 0-100
		"swap_total":   swap.Total / 1024 / 1024,
		"swap_used":    swap.Used / 1024 / 1024,
		"swap_percent": swap.UsedPercent,
	}

}

// ================= Disk =================

// physicalFS adalah daftar filesystem yang dianggap "nyata" (bukan virtual).
var physicalFS = map[string]bool{
	"ext2": true, "ext3": true, "ext4": true,
	"xfs": true, "btrfs": true, "zfs": true,
	"f2fs": true, "reiserfs": true, "jfs": true,
	"vfat": true, "ntfs": true, "exfat": true,
}

func (s *ResourceService) CollectDisk() map[string]any {
	ctx := s.ctx()

	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return map[string]any{"error": err}
	}

	var (
		total, used, free uint64
		seenDevice        = make(map[string]bool)
		details           []map[string]any
	)

	for _, p := range partitions {
		// filter filesystem virtual (tmpfs,overlay,proc,cgroup,dll)
		if !physicalFS[p.Fstype] {
			continue
		}

		// Deduplikasi device: beberapa mount point bisa merujuk ke device yang sama.
		if p.Device != "" && seenDevice[p.Device] {
			continue
		}

		seenDevice[p.Device] = true

		// tentukan path yang akan di statfs
		usagePath := p.Mountpoint
		if s.hostRoot != "" {
			// map mountpoint hsot ke dalam path yang di mount di container

			candidate := filepath.Join(s.hostRoot, p.Mountpoint)

			if _, err := os.Stat(candidate); err != nil {
				usagePath = p.Mountpoint
			}
		}
		u, err := disk.UsageWithContext(ctx, usagePath)
		if err != nil {
			continue
		}

		total += u.Total
		used += u.Used
		free += u.Free

		details = append(details, map[string]any{
			"device":     p.Device,
			"mountpoint": p.Mountpoint,
			"fstype":     p.Fstype,
			"total":      u.Total / 1024 / 1024, // MB
			"used":       u.Used / 1024 / 1024,  // MB
			"free":       u.Free / 1024 / 1024,  // MB
			"percent":    u.UsedPercent,         // 0-100
		})
	}

	if total == 0 {
		if u, err := disk.UsageWithContext(ctx, "/"); err != nil && u != nil {
			total, used, free = u.Total, u.Used, u.Free
			details = append(details, map[string]any{
				"device":     "container-root",
				"mountpoint": "/",
				"fstype":     "overlay",
				"total":      u.Total / 1024 / 1024,
				"used":       u.Used / 1024 / 1024,
				"free":       u.Free / 1024 / 1024,
				"percent":    u.UsedPercent,
			})
		}
	}
	percent := 0.0
	if total > 0 {
		percent = float64(used) / float64(total) * 100
	}
	return map[string]any{
		"total":      total / 1024 / 1024, // MB
		"used":       used / 1024 / 1024,  // MB
		"free":       free / 1024 / 1024,  // MB
		"percent":    percent,
		"partitions": details, // detail per partisi untuk ditampilkan di GUI
	}
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
