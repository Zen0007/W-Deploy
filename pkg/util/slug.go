package util

import (
	"regexp"
	"strings"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// IsValidSlug memvalidasi string yang akan dipakai sebagai nama container / DNS label.
func IsValidSlug(s string) bool {
	return slugRe.MatchString(s)
}

// ShortID memotong container ID panjang (12 char) untuk logging.
func ShortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// Slugify mengubah string menjadi slug aman untuk subdomain.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == '-' || r == '_' || r == ' ':
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// SanitizeContainerName membatasi nama container sesuai regex Docker.
func SanitizeContainerName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
