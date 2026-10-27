package tests

import (
	"W-dev/internal/projectconfig"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestResolve_Default_NoSignals(t *testing.T) {
	dir := t.TempDir()
	rc, err := projectconfig.Resolve(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rc.ProjectType != projectconfig.TypeNode || rc.ConfigSource != projectconfig.SourceDefault {
		t.Errorf("want node/default, got %s/%s", rc.ProjectType, rc.ConfigSource)
	}
}

func TestResolve_FEHint(t *testing.T) {
	dir := t.TempDir()
	rc, err := projectconfig.Resolve(dir, "static")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rc.ProjectType != projectconfig.TypeStatic || rc.ConfigSource != projectconfig.SourceFE {
		t.Errorf("want static/fe, got %s/%s", rc.ProjectType, rc.ConfigSource)
	}
}

func TestResolve_FEHint_Invalid(t *testing.T) {
	dir := t.TempDir()
	_, err := projectconfig.Resolve(dir, "python")
	if err == nil {
		t.Fatal("want error for invalid fe hint, got nil")
	}
}

func TestResolve_WDeployJSON_WinsOverFE(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wdeploy.json", `{"type":"spa"}`)
	rc, err := projectconfig.Resolve(dir, "static")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rc.ProjectType != projectconfig.TypeSPA || rc.ConfigSource != projectconfig.SourceWDeployJSON {
		t.Errorf("want spa/wdeploy.json, got %s/%s", rc.ProjectType, rc.ConfigSource)
	}
}

func TestResolve_WDeployJSON_Corrupt(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wdeploy.json", `{not valid json`)
	_, err := projectconfig.Resolve(dir, "")
	if err == nil {
		t.Fatal("want error for corrupt wdeploy.json")
	}
}

func TestResolve_WDeployJSON_InvalidType(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wdeploy.json", `{"type":"ruby"}`)
	_, err := projectconfig.Resolve(dir, "")
	if err == nil {
		t.Fatal("want error for invalid type in wdeploy.json")
	}
}

func TestResolve_WDeployJSON_NoType_FallsToFE(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "wdeploy.json", `{"build":{"command":"npm run build"}}`)
	rc, err := projectconfig.Resolve(dir, "static")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rc.ProjectType != projectconfig.TypeStatic || rc.ConfigSource != projectconfig.SourceFE {
		t.Errorf("want static/fe, got %s/%s", rc.ProjectType, rc.ConfigSource)
	}
	if rc.Build == nil || rc.Build.Command != "npm run build" {
		t.Errorf("build config not preserved from wdeploy.json: %+v", rc.Build)
	}
}

func TestResolve_AutoDetect_SPA(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"scripts":{"build":"vite build","start":"vite preview"}}`)
	rc, err := projectconfig.Resolve(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rc.ProjectType != projectconfig.TypeSPA || rc.ConfigSource != projectconfig.SourceAuto {
		t.Errorf("want spa/auto, got %s/%s", rc.ProjectType, rc.ConfigSource)
	}
}

func TestResolve_AutoDetect_Node(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"scripts":{"start":"node server.js"}}`)
	rc, err := projectconfig.Resolve(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rc.ProjectType != projectconfig.TypeNode || rc.ConfigSource != projectconfig.SourceAuto {
		t.Errorf("want node/auto, got %s/%s", rc.ProjectType, rc.ConfigSource)
	}
}

func TestResolve_AutoDetect_Static(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<html></html>`)
	rc, err := projectconfig.Resolve(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rc.ProjectType != projectconfig.TypeStatic || rc.ConfigSource != projectconfig.SourceAuto {
		t.Errorf("want static/auto, got %s/%s", rc.ProjectType, rc.ConfigSource)
	}
}

func TestResolve_AutoDetect_AnyHTML(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "blog.html", `<html></html>`)
	rc, err := projectconfig.Resolve(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rc.ProjectType != projectconfig.TypeStatic || rc.ConfigSource != projectconfig.SourceAuto {
		t.Errorf("want static/auto, got %s/%s", rc.ProjectType, rc.ConfigSource)
	}
}
