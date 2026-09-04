package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"barber-booking/security"
)

// The application has exactly one administrator. Their credentials come
// from the environment instead of being written into the source, and the
// password is stored only as a hash.
var (
	adminUsername     = "admin"
	adminPasswordHash string
)

// InitAdmin prepares the administrator account.
//
// ADMIN_USERNAME and ADMIN_PASSWORD override the defaults. When no
// password is configured a random one is generated and returned so that
// main can print it once at startup; that keeps a fresh checkout usable
// without shipping a well-known password like the previous hardcoded
// admin/password123.
func InitAdmin() (generated string, err error) {
	if name := strings.TrimSpace(os.Getenv("ADMIN_USERNAME")); name != "" {
		adminUsername = name
	}

	password := os.Getenv("ADMIN_PASSWORD")

	if password == "" {
		buf := make([]byte, 12)

		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("generate admin password: %w", err)
		}

		password = base64.RawURLEncoding.EncodeToString(buf)
		generated = password
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return "", err
	}

	adminPasswordHash = hash

	return generated, nil
}

// AdminUsername reports the configured administrator name, for the
// startup banner.
func AdminUsername() string {
	return adminUsername
}

// validAdminCredentials checks a login attempt. Both comparisons run
// before the result is combined so that a wrong username costs the same
// as a wrong password.
func validAdminCredentials(username, password string) bool {
	nameMatches := subtle.ConstantTimeCompare(
		[]byte(username),
		[]byte(adminUsername),
	) == 1

	passwordMatches := security.VerifyPassword(adminPasswordHash, password)

	return nameMatches && passwordMatches
}
