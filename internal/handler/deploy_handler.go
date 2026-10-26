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
	"W-dev/pkg/util"
)

const maxZipUpload = 200 << 20 // 200 MiB
const serverSourceRoot = "/srv/apps"

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
//   - multipart/form-data     -> uploaded folder files + name, slug, port, buildpack, database, env (JSON)
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
		if req.Source == "local" {
			if req.Path == "" {
				writeError(w, http.StatusBadRequest, "path source lokal wajib diisi")
				return
			}
			req.SourceType = string(constants.SourceZip)
			if req.Name == "" {
				req.Name = filepath.Base(filepath.Clean(req.Path))
			}
			if req.Slug == "" {
				req.Slug = util.Slugify(req.Name)
			}
		} else if req.Source == "github" && req.SourceType == "" {
			req.SourceType = string(constants.SourceGitHub)
		} else if req.Source != "" && req.Source != "github" {
			writeError(w, http.StatusBadRequest, "source harus github atau local")
			return
		}
	}

	if req.SourceType == string(constants.SourceZip) && req.Source != "local" &&
		!strings.HasPrefix(ct, "multipart/form-data") {
		writeError(w, http.StatusBadRequest, "source zip harus dikirim sebagai multipart/form-data")
		return
	}
	if req.Database != "" && !validEngine(req.Database) {
		writeError(w, http.StatusBadRequest, "database harus salah satu: postgres, mysql, mongodb, redis")
		return
	}

	if req.Source == "local" {
		var err error
		zipPath, err = h.saveServerFolderAsZip(req.Path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	app, warnings, err := h.apps.Prepare(r.Context(), service.AppInput{
		Owner: claims.Login, Name: req.Name, Slug: req.Slug,
		SourceType: constants.SourceType(req.SourceType), RepoFull: req.RepoFull,
		Branch: req.Branch, Port: req.Port, Buildpack: req.Buildpack,
		AutoDeploy: req.AutoDeploy, Env: req.Env, Database: req.Database,
	})
	if err != nil {
		if zipPath != "" {
			_ = os.Remove(zipPath)
		}
		writeServiceError(w, err)
		return
	}

	if app.SourceType == string(constants.SourceZip) && zipPath == "" {
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

func (h *DeployHandler) saveServerFolderAsZip(sourcePath string) (string, error) {
	return archiveServerFolder(serverSourceRoot, sourcePath, h.workspace)
}

func archiveServerFolder(sourceRoot, sourcePath, workspace string) (string, error) {
	if !filepath.IsAbs(sourcePath) {
		return "", fmt.Errorf("path source harus berupa absolute path di bawah %s", sourceRoot)
	}

	root, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return "", fmt.Errorf("folder source server tidak tersedia: %s", sourceRoot)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(sourcePath))
	if err != nil {
		return "", fmt.Errorf("folder source tidak ditemukan")
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == "." || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path source harus berada di dalam %s", sourceRoot)
	}

	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("path source harus berupa folder")
	}

	dir := filepath.Join(workspace, "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("gagal membuat folder upload: %v", err)
	}
	out, err := os.CreateTemp(dir, "local-*.zip")
	if err != nil {
		return "", fmt.Errorf("gagal membuat file zip source")
	}
	dst := out.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(dst)
		}
	}()

	zipWriter := zip.NewWriter(out)
	var total int64
	fileCount := 0
	walkErr := filepath.WalkDir(resolved, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == resolved {
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("source tidak boleh berisi symbolic link: %s", entry.Name())
		}
		if fileInfo.IsDir() {
			return nil
		}
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("tipe file source tidak didukung: %s", entry.Name())
		}
		if fileInfo.Size() > maxZipUpload-total {
			return fmt.Errorf("folder source terlalu besar (maks 200 MB)")
		}

		relativePath, err := filepath.Rel(resolved, path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relativePath)
		header.Method = zip.Deflate
		entryWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		openedInfo, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return err
		}
		if !openedInfo.Mode().IsRegular() || !os.SameFile(fileInfo, openedInfo) {
			_ = file.Close()
			return fmt.Errorf("file source berubah saat dibaca: %s", entry.Name())
		}
		copied, copyErr := io.Copy(entryWriter, io.LimitReader(file, maxZipUpload-total+1))
		closeErr := file.Close()
		total += copied
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if total > maxZipUpload {
			return fmt.Errorf("folder source terlalu besar (maks 200 MB)")
		}
		fileCount++
		return nil
	})
	if walkErr != nil {
		_ = zipWriter.Close()
		_ = out.Close()
		return "", fmt.Errorf("gagal membaca folder source: %v", walkErr)
	}
	if fileCount == 0 {
		_ = zipWriter.Close()
		_ = out.Close()
		return "", fmt.Errorf("folder source tidak berisi file")
	}
	if err := zipWriter.Close(); err != nil {
		_ = out.Close()
		return "", fmt.Errorf("gagal menyelesaikan file zip: %v", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("gagal menulis file zip: %v", err)
	}
	ok = true
	return dst, nil
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
