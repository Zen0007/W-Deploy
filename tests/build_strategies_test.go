package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"W-dev/internal/projectconfig"
	"W-dev/internal/service"
)

func TestStaticStrategy_RequiresHTML(t *testing.T) {
	dir := t.TempDir() // kosong
	strat := &service.StaticStrategy{}
	_, err := strat.Build(context.Background(), service.BuildInput{
		SourcePath: dir, AppSlug: "x", ImageName: "paas/x:1", Port: 8080,
		Config: &projectconfig.ResolveConfig{ProjectType: projectconfig.TypeStatic},
	})
	if err == nil {
		t.Fatal("expected error when no html files present")
	}
}

func TestSPAStrategy_RequiresPackageJSON(t *testing.T) {
	dir := t.TempDir()
	strat := &service.SPAStrategy{}
	_, err := strat.Build(context.Background(), service.BuildInput{
		SourcePath: dir, AppSlug: "x", ImageName: "paas/x:1", Port: 8080,
		Config: &projectconfig.ResolveConfig{ProjectType: projectconfig.TypeSPA},
	})
	if err == nil {
		t.Fatal("expected error when package.json missing")
	}
}

func TestSPAStrategy_NoBuildScript(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "package.json"),
		[]byte(`{"scripts":{"start":"node app.js"}}`), 0o644)
	strat := &service.SPAStrategy{}
	_, err := strat.Build(context.Background(), service.BuildInput{
		SourcePath: dir, AppSlug: "x", ImageName: "paas/x:1", Port: 8080,
		Config: &projectconfig.ResolveConfig{ProjectType: projectconfig.TypeSPA},
	})
	if err == nil {
		t.Fatal("expected error when build script missing")
	}
}

func TestSPAStrategy_WDeployOverrides(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "package.json"),
		[]byte(`{"scripts":{"build":"echo should-be-ignored"}}`), 0o644)

	cfg := &projectconfig.ResolveConfig{
		ProjectType: projectconfig.TypeSPA,
		Build: &projectconfig.BuildConfig{
			Command: "echo custom-build",
			Output:  "build",
		},
	}

	parsed, err := service.ParseFrontendProject(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if parsed.BuildCmd != "echo custom-build" {
		t.Errorf("want build cmd from wdeploy.json, got %q", parsed.BuildCmd)
	}
	if parsed.OutputDir != "build" {
		t.Errorf("want output dir from wdeploy.json, got %q", parsed.OutputDir)
	}
}

// func TestCustomStrategy_DetectsDockerfile(t *testing.T) {
// 	dir := t.TempDir()
// 	// Tidak ada Dockerfile — fallback ke node akan error karena tidak ada package.json
// 	strat := service.NewCustomStrategy(&service.NodeStrategy{build: &service.BuildService{}})
// 	_, err := strat.Build(context.Background(), service.BuildInput{
// 		SourcePath: dir, AppSlug: "x", ImageName: "paas/x:1", Port: 8080,
// 		Config: &projectconfig.ResolveConfig{ProjectType: projectconfig.TypeCustom},
// 	})
// 	// akan gagal di node strategy karena package.json tidak ada — expected
// 	if err == nil {
// 		t.Fatal("expected fallback to fail when no package.json")
// 	}
// }

func TestEscapeEnvValue(t *testing.T) {
	cases := map[string]string{
		`simple`:      `"simple"`,
		`with"quote`:  `"with\"quote"`,
		`with\backsl`: `"with\\backsl"`,
	}
	for in, want := range cases {
		if got := service.EscapeEnvValue(in); got != want {
			t.Errorf("escapeEnvValue(%q) = %q, want %q", in, got, want)
		}
	}
}
