package service

import (
	"archive/zip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
	return p
}

func TestExtractZipStripsSingleRoot(t *testing.T) {
	z := makeZip(t, map[string]string{"proj/package.json": "{}", "proj/src/index.js": "x"})
	dest := filepath.Join(t.TempDir(), "out")
	if err := NewUploadService().ExtractZip(z, dest); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"package.json", "src/index.js"} {
		if _, err := os.Stat(filepath.Join(dest, p)); err != nil {
			t.Errorf("%s harus ada: %v", p, err)
		}
	}
}

func TestExtractZipRejectsZipSlip(t *testing.T) {
	z := makeZip(t, map[string]string{"../evil.txt": "x", "ok.txt": "y"})
	dest := filepath.Join(t.TempDir(), "out")
	if err := NewUploadService().ExtractZip(z, dest); err == nil {
		t.Fatal("zip-slip harus ditolak")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "evil.txt")); err == nil {
		t.Fatal("file keluar dari direktori tujuan")
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	s := NewGitHubService("id", "sec", "whsecret", "https://api.example.com/")
	body := []byte(`{"ref":"refs/heads/main"}`)
	m := hmac.New(sha256.New, []byte("whsecret"))
	m.Write(body)
	sig := "sha256=" + hex.EncodeToString(m.Sum(nil))

	if !s.VerifyWebhookSignature(body, sig) {
		t.Error("signature valid harus diterima")
	}
	if s.VerifyWebhookSignature([]byte("tampered"), sig) {
		t.Error("body berubah harus ditolak")
	}
	if s.VerifyWebhookSignature(body, "") || s.VerifyWebhookSignature(body, "sha256=zz") {
		t.Error("signature kosong/rusak harus ditolak")
	}
	if NewGitHubService("id", "sec", "", "x").VerifyWebhookSignature(body, sig) {
		t.Error("tanpa webhook secret semua request harus ditolak")
	}
}

func TestGitHubAuthURLAndWebhookURL(t *testing.T) {
	s := NewGitHubService("cid", "sec", "w", "https://api.example.com/")
	if got := s.WebhookURL(); got != "https://api.example.com/api/webhook/github" {
		t.Errorf("WebhookURL = %s", got)
	}
	u, err := url.Parse(s.AuthURL("https://api.example.com/cb", "st"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("client_id") != "cid" || q.Get("state") != "st" || !strings.Contains(q.Get("scope"), "repo") {
		t.Errorf("query salah: %v", q)
	}
}

func TestTraefikLabels(t *testing.T) {
	tr := NewTraefikService("proxy-net", "example.com", false, "cloudflare")
	l := tr.Labels(7, "myapp", 3000)
	if l["traefik.http.routers.paas-myapp.rule"] != "Host(`myapp.example.com`)" {
		t.Errorf("rule salah: %v", l)
	}
	if l["traefik.http.services.paas-myapp.loadbalancer.server.port"] != "3000" {
		t.Error("port salah")
	}
	if l["traefik.http.routers.paas-myapp.entrypoints"] != "web" {
		t.Error("tanpa TLS harus entrypoint web")
	}
	tls := NewTraefikService("proxy-net", "example.com", true, "cloudflare").Labels(7, "myapp", 3000)
	if tls["traefik.http.routers.paas-myapp.entrypoints"] != "websecure" ||
		tls["traefik.http.routers.paas-myapp.tls.certresolver"] != "cloudflare" {
		t.Error("label TLS salah")
	}
}

func TestValidateRepoURLAndBranch(t *testing.T) {
	for _, bad := range []string{"http://github.com/a/b", "https://user:pw@github.com/a/b", "git@github.com:a/b", "--upload-pack=x"} {
		if validateRepoURL(bad) == nil {
			t.Errorf("%q harus ditolak", bad)
		}
	}
	if validateRepoURL("https://github.com/a/b.git") != nil {
		t.Error("url valid ditolak")
	}
	if branchRe.MatchString("--force") || branchRe.MatchString("a b") {
		t.Error("branch berbahaya lolos")
	}
}

func TestValidateEnvKey(t *testing.T) {
	if validateEnvKey("DATABASE_URL") != nil || validateEnvKey("a1") != nil {
		t.Error("key valid ditolak")
	}
	for _, bad := range []string{"", "1A", "A-B", "A=B", "A B"} {
		if validateEnvKey(bad) == nil {
			t.Errorf("%q harus ditolak", bad)
		}
	}
}

func TestLogWriterKeepsFullLog(t *testing.T) {
	ch := make(chan string, 10)
	w := NewLogWriter(ch, 1<<20)
	_, _ = w.Write([]byte("line1\nli"))
	_, _ = w.Write([]byte("ne2\r\n"))
	if got := w.String(); got != "line1\nline2\n" {
		t.Errorf("String() = %q", got)
	}
	if len(ch) != 2 {
		t.Errorf("channel menerima %d baris, harap 2", len(ch))
	}
}
