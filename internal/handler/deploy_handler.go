package handler

import (
	"archive/zip"
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

		// Frontend mengirim source="upload"
		req.SourceType = string(constants.SourceZip) // Tetap gunakan tipe Zip agar service layer tidak perlu diubah

		// Ambil nama folder dari file pertama sebagai default name & slug
		files := r.MultipartForm.File["files"]
		defaultName := "uploaded-app"
		if len(files) > 0 {
			// fileHeader.Filename biasanya berformat "nama-folder/path/ke/file.go"
			parts := strings.Split(files[0].Filename, "/")
			if len(parts) > 0 && parts[0] != "" {
				defaultName = parts[0]
			}
		}

		req.Name = r.FormValue("name")
		if req.Name == "" {
			req.Name = defaultName
		}

		req.Slug = r.FormValue("slug")
		if req.Slug == "" {
			// Jika frontend tidak mengirim slug, buat sederhana dari nama folder
			req.Slug = strings.ToLower(strings.ReplaceAll(defaultName, " ", "-"))
		}

		req.Buildpack = r.FormValue("buildpack")
		req.Database = r.FormValue("database")
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
		// Panggil fungsi baru kita
		path, err := h.saveFolderAsZip(r, app.Slug)
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

// saveFolderAsZip menyimpan folder sebagai file zip di workspace dan memvalidasi magic number ZIP.
func (h *DeployHandler) saveFolderAsZip(r *http.Request, slug string) (string, error) {
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		return "", fmt.Errorf("file zip tidak ditemukan")
	}

	dir := filepath.Join(h.workspace, "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("gagal membuat folder upload: %v", err)
	}

	dst := filepath.Join(dir, fmt.Sprintf("%s-%d.zip", slug, time.Now().UnixNano()))
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return "", errors.New("gagal membuat file zip")
	}

	defer out.Close()

	zipWriter := zip.NewWriter(out)
	defer zipWriter.Close()

	for _, fileHeader := range files {
		// open file form
		file, err := fileHeader.Open()
		if err != nil {
			return "", fmt.Errorf("gagal membuka file %s: %v", fileHeader.Filename, err)
		}
		w, err := zipWriter.Create(fileHeader.Filename)
		if err != nil {
			file.Close()
			return "", fmt.Errorf("gagal membuat entry zip untuk %s: %v", fileHeader.Filename, err)
		}
		if _, err := io.Copy(w, file); err != nil {
			file.Close()
			return "", fmt.Errorf("gagal menulis file %s ke zip: %v", fileHeader.Filename, err)
		}
		file.Close()
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
