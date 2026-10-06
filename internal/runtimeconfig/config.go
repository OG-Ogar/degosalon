// Package runtimeconfig defines deployment safety settings shared by startup,
// authentication, and persistence. Errors name settings without exposing values.
package runtimeconfig

import (
	"fmt"
	"net/netip"
	"os"
	"strings"
)

func production(getenv func(string) string) bool {
	return getenv("APP_ENV") == "production" || getenv("RENDER") == "true"
}
func Production() bool { return production(os.Getenv) }
func validateAdmin(getenv func(string) string) error {
	if !production(getenv) {
		return nil
	}
	if strings.TrimSpace(getenv("ADMIN_USERNAME")) == "" {
		return fmt.Errorf("ADMIN_USERNAME is required in production")
	}
	password := getenv("ADMIN_PASSWORD")
	if strings.TrimSpace(password) == "" || len(password) < 12 || len(password) > 72 {
		return fmt.Errorf("ADMIN_PASSWORD must contain 12 to 72 bytes in production")
	}
	return nil
}
func ValidateAdmin() error { return validateAdmin(os.Getenv) }
func validate(getenv func(string) string) error {
	proxies := strings.TrimSpace(getenv("TRUSTED_PROXY_CIDRS"))
	if len(proxies) > 4096 || (proxies != "" && len(strings.Split(proxies, ",")) > 32) {
		return fmt.Errorf("TRUSTED_PROXY_CIDRS supports at most 32 CIDR ranges")
	}
	if proxies != "" {
		for _, value := range strings.Split(proxies, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
			if err != nil || prefix.Bits() == 0 {
				return fmt.Errorf("TRUSTED_PROXY_CIDRS must contain specific valid CIDR ranges")
			}
		}
	}
	switch getenv("APP_ENV") {
	case "", "development", "test", "production":
	default:
		return fmt.Errorf("APP_ENV must be development, test, or production")
	}
	switch getenv("SECURE_COOKIES") {
	case "", "0", "1":
	default:
		return fmt.Errorf("SECURE_COOKIES must be 0 or 1")
	}
	switch getenv("SHOW_RESET_LINKS") {
	case "", "0", "1":
	default:
		return fmt.Errorf("SHOW_RESET_LINKS must be 0 or 1")
	}
	if !production(getenv) {
		return nil
	}
	if strings.TrimSpace(getenv("DATABASE_URL")) == "" {
		return fmt.Errorf("DATABASE_URL is required in production; development memory storage is disabled")
	}
	if err := validateAdmin(getenv); err != nil {
		return err
	}
	if getenv("SECURE_COOKIES") == "0" {
		return fmt.Errorf("SECURE_COOKIES cannot be disabled in production")
	}
	if getenv("SHOW_RESET_LINKS") == "1" {
		return fmt.Errorf("SHOW_RESET_LINKS cannot be enabled in production")
	}
	return nil
}
func Validate() error { return validate(os.Getenv) }
