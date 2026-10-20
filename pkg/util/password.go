package util

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
)

// Generate password string url save
func GeneratePassword(n int) (string, error) {
	if n <= 0 {
		return "", errors.New("panjang password harus >  0")
	}

	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// RawURLEncoding menghasilkan ceil(n*4/3) char, jadi >= n.
	s := base64.RawURLEncoding.EncodeToString(buf)
	return s[:n], nil
}
