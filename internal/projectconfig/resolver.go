package projectconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const wdeployJSON = "wdeploy.json"

type wdeployFile struct {
	Type  string       `json:"project_type"`
	Build *BuildConfig `json:"build,omitempty"`
	Run   *RunConfig   `json:"run,omitempty"`
}

// Resolve menentukan ResolvedConfig berdasarkan urutan precedence:
//
//  1. wdeploy.json  → jika ada & type valid
//  2. feHint        → project_type dari request FE
//  3. auto-detect   → deteksi filesystem (package.json / index.html)
//  4. default       → "node"
//
// File wdeploy.json yang rusak atau berisi type tidak dikenal → return error.
// Ini mencegah fallback diam-diam ke kategori yang salah.

func Resolve(sourcePath, feHint string) (*ResolveConfig, error) {
	if sourcePath == "" {
		return nil, errors.New("sourcePath tidak boleh kosong")
	}

	wd, err := readWDeployJSON(sourcePath)
	if err != nil {
		return nil, err
	}

	// Resolve projectype
	var pt ProjectType
	var src ConfigSource

	switch {
	case wd != nil && wd.Type != "":
		if !IsValidProjectType(wd.Type) {
			return nil, fmt.Errorf(
				"wdeploy.json: type %q tidak dikenal (harus salah satu: static, spa, node, custom)",
				wd.Type)
		}
		pt = ProjectType(wd.Type)
		src = SourceWDeployJSON

	case feHint != "":
		if !IsValidProjectType(feHint) {
			return nil, fmt.Errorf(
				"project_type dari request tidak dikenal: %q (harus salah satu: static, spa, node, custom)",
				feHint)
		}
		pt = ProjectType(feHint)
		src = SourceFE
	default:
		if detected, ok := detectFromFS(sourcePath); ok {
			pt = detected
			src = SourceAuto
		} else {
			pt = TypeNode
			src = SourceDefault
		}
	}

	rc := &ResolveConfig{
		ProjectType:  pt,
		ConfigSource: src,
	}

	if wd != nil {
		rc.Build = wd.Build
		rc.Run = wd.Run
	}

	return rc, nil
}

// readWDeployJSON mengembalikan nil, nil jika file tidak ada.
// Mengembalikan error jika file ada tapi rusak.
func readWDeployJSON(sourcePath string) (*wdeployFile, error) {
	path := filepath.Join(sourcePath, wdeployJSON)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("baca %s :%w", wdeployJSON, err)
	}

	var wd wdeployFile
	if err := json.Unmarshal(data, &wd); err != nil {
		return nil, fmt.Errorf("%s tidak valid %w", wdeployJSON, err)
	}

	return &wd, nil
}

// detectFromFS mendeteksi tipe proyek dari isi filesystem.
// Mengembalikan ("", false) jika tidak ada sinyal yang jelas.
func detectFromFS(sourcePath string) (ProjectType, bool) {

	// 1.Node.js family
	pkgPath := filepath.Join(sourcePath, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}

		if json.Unmarshal(data, &pkg) == nil {
			if _, ok := pkg.Scripts["build"]; ok {
				return TypeSPA, true
			}
			if _, ok := pkg.Scripts["start"]; ok {
				return TypeNode, true
			}
		}
		// package.json ada tapi tanpa script umum → asumsikan node.
		return TypeNode, true
	}

	// 2. pure static- index.html
	if _, err := os.Stat(filepath.Join(sourcePath, "index.html")); err == nil {
		return TypeStatic, true
	}

	// 2b. Pure static .html
	if match, _ := filepath.Glob(filepath.Join(sourcePath, "*.html")); len(match) > 0 {
		return TypeStatic, true
	}

	return "", false
}
