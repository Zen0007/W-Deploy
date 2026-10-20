package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"W-dev/internal/constants"
	"W-dev/internal/middleware"
	"W-dev/internal/model"
	"W-dev/internal/service"
)

const maxZipUpload = 200 << 20 // 200 MiB

// DeployHandler menerima permintaan deploy (repo GitHub atau upload ZIP).
type DeployHandler struct {
	apps      *service.AppService
	workspace string
}

func NewDeployHandler(apps *service.AppService, workspace string) *DeployHandler {
	return &DeployHandler{apps: apps, workspace: workspace}
}

// Deploy: POST /api/deploy
//   - application/json        -> source github (lihat model.DeployRequest)
//   - multipart/form-data     -> source zip: field name, slug, port, buildpack, database, env (JSON) + file "file"
func (h *DeployHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	claims := middleware.User(r)

	var (
		req     model.DeployRequest
		zipPath string
	)

	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "multipart/form-data"):
		r.Body = http.MaxBytesReader(w, r.Body, maxZipUpload+1<<20)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "form tidak valid atau file terlalu besar (maks 200 MB)")
			return
		}
		req.Name = r.FormValue("name")
		req.Slug = r.FormValue("slug")
		req.Buildpack = r.FormValue("buildpack")
		req.Database = r.FormValue("database")
		req.SourceType = string(constants.SourceZip)
		req.Port, _ = strconv.Atoi(r.FormValue("port"))
		if raw := r.FormValue("env"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &req.Env); err != nil {
				writeError(w, http.StatusBadRequest, "field env harus JSON object")
				return
			}
		}
	default:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "body JSON tidak valid")
			return
		}
	}

	if req.SourceType == string(constants.SourceZip) && !strings.HasPrefix(ct, "multipart/form-data") {
		writeError(w, http.StatusBadRequest, "source zip harus dikirim sebagai multipart/form-data")
		return
	}
	if req.Database != "" && !validEngine(req.Database) {
		writeError(w, http.StatusBadRequest, "database harus salah satu: postgres, mysql, mongodb, redis")
		return
	}

	app, warnings, err := h.apps.Prepare(r.Context(), service.AppInput{
		Owner: claims.Login, Name: req.Name, Slug: req.Slug,
		SourceType: constants.SourceType(req.SourceType), RepoFull: req.RepoFull,
		Branch: req.Branch, Port: req.Port, Buildpack: req.Buildpack,
		AutoDeploy: req.AutoDeploy, Env: req.Env, Database: req.Database,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if app.SourceType == string(constants.SourceZip) {
		path, err := h.saveZip(r, app.Slug)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		zipPath = path
	}

	dep, err := h.apps.Deploy(r.Context(), app, service.DeployOptions{
		TriggeredBy: "user", ZipPath: zipPath, DBEngine: req.Database,
	})
	if err != nil {
		if zipPath != "" {
			_ = os.Remove(zipPath)
		}
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, model.DeployResponse{
		App:          h.apps.View(app),
		DeploymentID: dep.ID,
		WebSocket:    fmt.Sprintf("/api/ws/apps/%d", app.ID),
		Warnings:     warnings,
	})
}

// saveZip menyimpan file upload ke workspace dan memvalidasi magic number ZIP.
func (h *DeployHandler) saveZip(r *http.Request, slug string) (string, error) {
	file, _, err := r.FormFile("file")
	if err != nil {
		return "", errors.New("file .zip wajib diunggah pada field \"file\"")
	}
	defer file.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(file, magic); err != nil || string(magic[:2]) != "PK" {
		return "", errors.New("file bukan zip yang valid")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	dir := filepath.Join(h.workspace, "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errors.New("gagal menyiapkan direktori upload")
	}
	dst := filepath.Join(dir, fmt.Sprintf("%s-%d.zip", slug, time.Now().UnixNano()))
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return "", errors.New("gagal menyimpan upload")
	}
	defer out.Close()

	if _, err := io.Copy(out, io.LimitReader(file, maxZipUpload+1)); err != nil {
		_ = os.Remove(dst)
		return "", errors.New("gagal menyimpan upload")
	}
	if fi, err := out.Stat(); err == nil && fi.Size() > maxZipUpload {
		_ = os.Remove(dst)
		return "", errors.New("ukuran zip melebihi 200 MB")
	}
	return dst, nil
}

func validEngine(e string) bool {
	switch service.DBEngine(e) {
	case service.DBPostgres, service.DBMySQL, service.DBMongo, service.DBRedis:
		return true
	}
	return false
}
