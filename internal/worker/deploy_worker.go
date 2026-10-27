package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"W-dev/internal/constants"
	"W-dev/internal/projectconfig"
	"W-dev/internal/repository"
	"W-dev/internal/service"
)

const maxStoredLog = 1 << 20 // 1 MiB log disimpan di DB

// Deps mengelompokkan dependency worker.
type Deps struct {
	Queue      *Queue
	Apps       *repository.AppRepository
	Deploys    *repository.DeploymentRepository
	Envs       *repository.EnvVarRepository
	AppDBs     *repository.AppDatabaseRepository
	Git        *service.GitService
	Upload     *service.UploadService
	Build      *service.BuildService
	Containers *service.ContainerService
	Databases  *service.DatabaseService
	Tokens     *service.TokenService
	Traefik    *service.TraefikService
	Sink       service.LogSink
	Workspace  string
}

// DeployWorker memproses clone -> build -> deploy secara asinkron.
type DeployWorker struct {
	Deps
	locks sync.Map // appID -> *sync.Mutex (cegah deploy bersamaan untuk app yang sama)
}

func NewDeployWorker(d Deps) *DeployWorker { return &DeployWorker{Deps: d} }

// Start menjalankan n goroutine worker dan memblokir sampai antrean ditutup & semua job selesai.
func (w *DeployWorker) Start(ctx context.Context, n int) {
	if n <= 0 {
		n = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range w.Queue.Chan() {
				w.safeHandle(ctx, job)
			}
			log.Printf("[worker] #%d berhenti", id)
		}(i + 1)
	}
	wg.Wait()
}

func (w *DeployWorker) safeHandle(ctx context.Context, job service.DeployJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[worker] panic pada deployment %d: %v", job.DeploymentID, r)
			_ = w.Deploys.Finish(context.Background(), job.DeploymentID,
				string(constants.StatusFailed), "", fmt.Sprintf("panic: %v", r), "")
		}
	}()
	w.handle(ctx, job)
}

func (w *DeployWorker) lockFor(appID uint) *sync.Mutex {
	m, _ := w.locks.LoadOrStore(appID, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// run menyimpan state satu deployment agar pesan event konsisten.
type run struct {
	w    *DeployWorker
	app  *repository.App
	dep  uint
	room string
	logs strings.Builder
}

func (r *run) emit(ev map[string]any) {
	ev["app"] = r.app.Slug
	ev["deployment_id"] = r.dep
	ev["ts"] = time.Now().UTC().Format(time.RFC3339)
	r.w.Sink.Publish(r.room, ev)
}

func (r *run) logf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if r.logs.Len() < maxStoredLog {
		r.logs.WriteString(line)
		r.logs.WriteString("\n")
	}
	r.emit(map[string]any{"type": "log", "line": line})
}

func (r *run) status(s string) {
	r.emit(map[string]any{"type": "status", "status": s})
}

func (w *DeployWorker) handle(ctx context.Context, job service.DeployJob) {
	bg := context.Background()

	app, err := w.Apps.FindByID(bg, job.AppID)
	if err != nil || app == nil {
		_ = w.Deploys.Finish(bg, job.DeploymentID, string(constants.StatusFailed), "", "app tidak ditemukan", "")
		return
	}

	mu := w.lockFor(app.ID)
	mu.Lock()
	defer mu.Unlock()

	r := &run{w: w, app: app, dep: job.DeploymentID, room: app.Slug}
	w.Sink.ResetHistory(r.room)
	start := time.Now()

	_ = w.Deploys.MarkStarted(bg, job.DeploymentID)
	_ = w.Apps.UpdateStatus(bg, app.ID, string(constants.StatusBuilding), "")
	r.status("source")
	r.logf("===> Pipeline dimulai untuk %s", app.Slug)

	imageTag, err := w.pipeline(ctx, r, app, job)
	if err != nil {
		w.fail(bg, r, app, imageTag, err)
		return
	}

	_ = w.Deploys.Finish(bg, job.DeploymentID, string(constants.StatusRunning), truncate(r.logs.String()), "", imageTag)
	r.status(string(constants.StatusRunning))
	r.emit(map[string]any{
		"type":     "done",
		"url":      w.Traefik.PublicURL(app.Slug),
		"image":    imageTag,
		"duration": time.Since(start).Round(time.Second).String(),
	})
}

// pipeline: source -> (database) -> build -> run. Mengembalikan tag image (bila sudah ada) dan error.
func (w *DeployWorker) pipeline(ctx context.Context, r *run, app *repository.App, job service.DeployJob) (string, error) {
	srcDir := filepath.Join(w.Workspace, constants.ReposDirName, app.Slug)

	// 1. Source
	switch constants.SourceType(app.SourceType) {
	case constants.SourceZip:
		if job.ZipPath != "" {
			r.logf("==> Mengekstrak source dari zip...")
			err := w.Upload.ExtractZip(job.ZipPath, srcDir)
			_ = os.Remove(job.ZipPath)
			if err != nil {
				return "", err
			}
		} else {
			// Kasus redeploy: source code lama di srcDir dipakai ulang jika tersedia
			if fi, err := os.Stat(srcDir); err != nil || !fi.IsDir() {
				return "", errors.New("source zip tidak tersedia di workspace: silakan unggah ulang file/folder")
			}
			r.logf("==> Menggunakan source code tersimpan untuk redeploy...")
		}
	default:
		r.logf("==> Clone %s (branch %s)...", app.RepoURL, app.Branch)
		token, err := w.Tokens.Get(ctx, app.OwnerLogin)
		if err != nil {
			r.logf("peringatan: token GitHub tidak bisa dibaca (%v); mencoba clone anonim", err)
		}
		res, err := w.Git.Clone(ctx, service.CloneRequest{
			RepoURL: app.RepoURL, Branch: app.Branch, Dest: srcDir, Token: token,
		})
		if err != nil {
			return "", err
		}
		_ = w.Deploys.SetCommit(ctx, r.dep, res.CommitSHA, res.CommitMsg)
		r.logf("commit %s: %s", short(res.CommitSHA), res.CommitMsg)
	}
	// ─── Phase 2: resolve config ─────────────────────────────────────────
	resolved, err := projectconfig.Resolve(srcDir, job.ProjectTypeHint)
	if err != nil {
		return "", fmt.Errorf("resolve project config: %w", err)
	}
	r.logf("==> Config: type=%s (source=%s)", resolved.ProjectType, resolved.ConfigSource)

	// Simpan snapshot ke deployment (best-effort; jangan gagalkan deploy jika ini error).
	if cfgBytes, jerr := json.Marshal(resolved); jerr == nil {
		_ = w.Deploys.SetResolvedConfig(ctx, r.dep,
			string(resolved.ProjectType),
			string(resolved.ConfigSource),
			string(cfgBytes),
		)
	}

	// Emit ke FE.
	r.emit(map[string]any{
		"type":          "config_resolved",
		"project_type":  string(resolved.ProjectType),
		"config_source": string(resolved.ConfigSource),
	})

	// ─────────────────────────────────────────────────────────────────────
	// Otomatis setup static web server jika project adalah web frontend tanpa server
	// prepareStaticProject(srcDir, app.Port, r.logf)

	// 2. Database (opsional)
	if job.DBEngine != "" {
		if err := w.ensureDatabase(ctx, r, app, job.DBEngine); err != nil {
			return "", err
		}
	}

	// 3. Env
	envVars, err := w.Envs.ListByApp(ctx, app.ID)
	if err != nil {
		return "", fmt.Errorf("baca env: %w", err)
	}
	buildEnv := map[string]string{}
	runEnv := make([]string, 0, len(envVars)+1)
	for _, e := range envVars {
		if e.Key == "PORT" {
			continue
		}
		runEnv = append(runEnv, e.Key+"="+e.Value)
		if strings.HasPrefix(e.Key, "BP_") { // variabel build-time Paketo
			buildEnv[e.Key] = e.Value
		}
	}
	runEnv = append(runEnv, fmt.Sprintf("PORT=%d", app.Port))

	// ── 4. Build dengan strategy ────────────────────────────────
	r.status(string(constants.StatusBuilding))
	imageTag := fmt.Sprintf("paas/%s:%d", app.Slug, r.dep)

	r.logf("==> Build image %s dengan buildpacks...", imageTag)

	buildCtx, cancel := context.WithTimeout(ctx, constants.BuildTimeout)
	defer cancel()

	logCh := make(chan string, 256)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for line := range logCh {
			r.logf("%s", line)
		}
	}()

	buildResult, buildErr := w.Build.BuildByStrategy(buildCtx, service.BuildInput{
		SourcePath: srcDir,
		AppSlug:    app.Slug,
		ImageName:  imageTag,
		Port:       app.Port,
		BuildEnv:   buildEnv,
		LogChannel: logCh,
		Config:     resolved, // dari Phase 2
	})
	close(logCh)
	<-done
	if buildErr != nil || buildResult == nil || !buildResult.Success {
		if buildResult != nil && buildResult.Error != "" {
			return "", fmt.Errorf("build (%s): %s", buildResult.Strategy, buildResult.Error)
		}
		return "", buildErr
	}
	r.logf("==> Build selesai (strategy=%s, port=%d, %s)",
		buildResult.Strategy, buildResult.RuntimePort, buildResult.Duration.Round(time.Second))

	// ── 5. Run staged container (old tetap hidup) ───────────────
	_ = w.Apps.UpdateStatus(ctx, app.ID, string(constants.StatusDeploying), "")
	r.status(string(constants.StatusDeploying))
	r.logf("==> Menjalankan container baru (staged)…")

	// Simpan container ID lama untuk rollback & promote
	oldContainerID := app.ContainerID
	oldImage := app.ImageTag

	runCtx, cancelRun := context.WithTimeout(ctx, constants.DeployTimeout)
	defer cancelRun()

	newContainerID, tempName, err := w.Containers.RunStaged(runCtx, service.RunSpec{
		AppID: app.ID, Slug: app.Slug, Image: imageTag,
		Port: buildResult.RuntimePort, Env: runEnv,
	}, r.dep)
	if err != nil {
		return imageTag, err
	}

	// ── 6. Health check container baru ──────────────────────────
	r.logf("==> Health check container %s…", tempName)
	if err := w.Containers.WaitReadyContainer(runCtx, tempName, buildResult.RuntimePort, 45*time.Second); err != nil {
		// ROLLBACK: container baru gagal → bersihkan, old tetap hidup
		r.logf("health check gagal: %v — rollback (container lama tetap aktif)", err)
		_ = w.Containers.Remove(runCtx, newContainerID)
		return imageTag, fmt.Errorf("health check gagal: %w", err)
	}
	r.logf("container baru sehat")

	// ── 7. Promote: swap old → new ──────────────────────────────
	finalName := "paas-app-" + app.Slug
	r.logf("==> Promote: swap %s → %s", short(oldContainerID), finalName)
	if err := w.Containers.Promote(runCtx, oldContainerID, newContainerID, finalName); err != nil {
		// Promote gagal di tengah jalan — laporkan tapi jangan matikan new
		// (old sudah di-remove, new masih ada dengan nama temp)
		return imageTag, fmt.Errorf("promote container: %w", err)
	}

	// Update DB setelah promote sukses
	if err := w.Apps.SetRuntime(ctx, app.ID, newContainerID, imageTag); err != nil {
		return imageTag, fmt.Errorf("simpan runtime: %w", err)
	}
	_ = w.Apps.UpdateStatus(ctx, app.ID, string(constants.StatusRunning), "")

	// ── 8. Cleanup image lama ───────────────────────────────────
	if oldImage != "" && oldImage != imageTag {
		if err := w.Build.RemoveImage(ctx, oldImage); err != nil {
			log.Printf("[worker] hapus image lama %v", err)
		}
	}

	r.logf("==> selesai : %s", w.Traefik.PublicURL(app.Slug))
	return imageTag, nil
}

func (w *DeployWorker) ensureDatabase(ctx context.Context, r *run, app *repository.App, engine string) error {
	existing, err := w.AppDBs.ListByApp(ctx, app.ID)
	if err != nil {
		return err
	}
	for _, d := range existing {
		if d.Engine == engine {
			r.logf("database %s sudah tersedia (%s)", engine, d.Host)
			return nil
		}
	}

	r.logf("==> Provision database %s...", engine)
	res, err := w.Databases.Provision(ctx, service.DBProvisionRequest{
		AppID: app.ID, AppSlug: app.Slug, Engine: service.DBEngine(engine),
	})
	if err != nil {
		return fmt.Errorf("provision database: %w", err)
	}
	rec := &repository.AppDatabase{
		AppID: app.ID, Engine: engine, ContainerID: res.ContainerID, ContainerName: res.ContainerName,
		VolumeName: res.VolumeName, Host: res.Host, Port: res.Port, Database: res.Database, Username: res.Username,
	}
	if err := w.AppDBs.Create(ctx, rec); err != nil {
		_ = w.Databases.Deprovision(ctx, res.ContainerID, res.VolumeName)
		return fmt.Errorf("simpan database: %w", err)
	}
	// URI hanya muncul sekali — simpan sebagai env var agar app bisa membacanya.
	if err := w.Envs.Set(ctx, app.ID, "DATABASE_URL", res.ConnectionURI); err != nil {
		return fmt.Errorf("simpan DATABASE_URL: %w", err)
	}
	r.logf("DATABASE_URL telah diset untuk app")
	return nil
}

func (w *DeployWorker) fail(ctx context.Context, r *run, app *repository.App, imageTag string, err error) {
	msg := err.Error()
	r.logf("GAGAL: %s", msg)
	_ = w.Deploys.Finish(ctx, r.dep, string(constants.StatusFailed), truncate(r.logs.String()), msg, imageTag)

	// Jika sudah ada container lama yang tetap berjalan (build gagal), jangan tandai app failed.
	fresh, _ := w.Apps.FindByID(ctx, app.ID)
	if fresh != nil && fresh.ContainerID != "" && fresh.Status != string(constants.StatusDeploying) {
		_ = w.Apps.UpdateStatus(ctx, app.ID, string(constants.StatusRunning), msg)
	} else {
		_ = w.Apps.UpdateStatus(ctx, app.ID, string(constants.StatusFailed), msg)
	}
	r.status(string(constants.StatusFailed))
	r.emit(map[string]any{"type": "error", "error": msg})
}

func truncate(s string) string {
	if len(s) <= maxStoredLog {
		return s
	}
	return s[len(s)-maxStoredLog:]
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
