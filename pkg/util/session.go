package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// SessionClaims adalah isi session token (mirip JWT HS256, tanpa dependency eksternal).
type SessionClaims struct {
	Login    string `json:"login"`
	GitHubID int64  `json:"gid"`
	Exp      int64  `json:"exp"`
}

var b64 = base64.RawURLEncoding

// SignSession membuat token "payload.signature".
func SignSession(secret string, c SessionClaims, ttl time.Duration) (string, error) {
	c.Exp = time.Now().Add(ttl).Unix()
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	p := b64.EncodeToString(payload)
	return p + "." + sign(secret, p), nil
}

// VerifySession memvalidasi signature & expiry, lalu mengembalikan claims.
func VerifySession(secret, token string) (*SessionClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("format token tidak valid")
	}
	if !hmac.Equal([]byte(sign(secret, parts[0])), []byte(parts[1])) {
		return nil, errors.New("signature token tidak valid")
	}
	raw, err := b64.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("payload token rusak")
	}
	var c SessionClaims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, errors.New("payload token rusak")
	}
	if time.Now().Unix() > c.Exp {
		return nil, errors.New("token kedaluwarsa")
	}
	return &c, nil
}

func sign(secret, data string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(data))
	return b64.EncodeToString(m.Sum(nil))
}
