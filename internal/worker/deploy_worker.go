package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"W-dev/internal/constants"
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
		r.logs.WriteString(line + "\n")
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
		if job.ZipPath == "" {
			return "", errors.New("source zip tidak tersedia: unggah ulang file .zip")
		}
		r.logf("==> Mengekstrak source dari zip...")
		err := w.Upload.ExtractZip(job.ZipPath, srcDir)
		_ = os.Remove(job.ZipPath)
		if err != nil {
			return "", err
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

	// 4. Build
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

	req := service.BuildRequest{
		AppSlug: app.Slug, SourcePath: srcDir, ImageName: imageTag,
		Env: buildEnv, LogChannel: logCh,
	}
	if app.Buildpack != "" {
		req.Buildpacks = []string{app.Buildpack}
	}
	_, buildErr := w.Build.Build(buildCtx, req)
	close(logCh)
	<-done
	if buildErr != nil {
		return "", buildErr
	}

	// 5. Run
	_ = w.Apps.UpdateStatus(ctx, app.ID, string(constants.StatusDeploying), "")
	r.status(string(constants.StatusDeploying))
	r.logf("==> Menjalankan container...")

	runCtx, cancelRun := context.WithTimeout(ctx, constants.DeployTimeout)
	defer cancelRun()

	oldImage := app.ImageTag
	containerID, err := w.Containers.Run(runCtx, service.RunSpec{
		AppID: app.ID, Slug: app.Slug, Image: imageTag, Port: app.Port, Env: runEnv,
	})
	if err != nil {
		return imageTag, err
	}
	if err := w.Apps.SetRuntime(ctx, app.ID, containerID, imageTag); err != nil {
		return imageTag, fmt.Errorf("simpan runtime: %w", err)
	}
	_ = w.Apps.UpdateStatus(ctx, app.ID, string(constants.StatusRunning), "")

	// 6. Health check best-effort (hanya bermakna jika backend ada di proxy-net)
	if err := w.Containers.WaitReady(runCtx, app.Slug, app.Port, 45*time.Second); err != nil {
		r.logf("peringatan: %v", err)
	} else {
		r.logf("app merespon pada port %d", app.Port)
	}

	// 7. Bersihkan image lama
	if oldImage != "" && oldImage != imageTag {
		if err := w.Build.RemoveImage(ctx, oldImage); err != nil {
			log.Printf("[worker] hapus image lama: %v", err)
		}
	}
	r.logf("==> Selesai: %s", w.Traefik.PublicURL(app.Slug))
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
