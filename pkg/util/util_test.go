package util

import (
	"strings"
	"testing"
	"time"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := Encrypt("secret", "gho_token123")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(enc, "gho_token123") {
		t.Fatal("ciphertext membocorkan plaintext")
	}
	got, err := Decrypt("secret", enc)
	if err != nil || got != "gho_token123" {
		t.Fatalf("decrypt = %q, %v", got, err)
	}
	if _, err := Decrypt("wrong", enc); err == nil {
		t.Fatal("secret salah harus gagal")
	}
}

func TestSessionSignVerify(t *testing.T) {
	tok, err := SignSession("s", SessionClaims{Login: "octocat", GitHubID: 1}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	c, err := VerifySession("s", tok)
	if err != nil || c.Login != "octocat" {
		t.Fatalf("verify = %+v, %v", c, err)
	}
	if _, err := VerifySession("other", tok); err == nil {
		t.Fatal("secret berbeda harus ditolak")
	}
	parts := strings.Split(tok, ".")
	if _, err := VerifySession("s", parts[0]+"x."+parts[1]); err == nil {
		t.Fatal("payload diubah harus ditolak")
	}
	expired, _ := SignSession("s", SessionClaims{Login: "a"}, -time.Minute)
	if _, err := VerifySession("s", expired); err == nil {
		t.Fatal("token kedaluwarsa harus ditolak")
	}
}
