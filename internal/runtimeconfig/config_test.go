package runtimeconfig

import (
	"strings"
	"testing"
)

func TestProductionConfiguration(t *testing.T) {
	baseline := map[string]string{"APP_ENV": "production", "DATABASE_URL": "private-database-value", "ADMIN_USERNAME": "operator", "ADMIN_PASSWORD": "private-password-value", "SUPABASE_URL": "https://storage.example", "SUPABASE_SECRET_KEY": "private-storage-value"}
	for _, test := range []struct{ name, key, value, want string }{{"valid", "", "", ""}, {"missing database", "DATABASE_URL", "", "DATABASE_URL"}, {"missing admin name", "ADMIN_USERNAME", "", "ADMIN_USERNAME"}, {"missing password", "ADMIN_PASSWORD", "", "ADMIN_PASSWORD"}, {"short password", "ADMIN_PASSWORD", "short", "ADMIN_PASSWORD"}, {"long password", "ADMIN_PASSWORD", strings.Repeat("x", 73), "ADMIN_PASSWORD"}, {"whitespace password", "ADMIN_PASSWORD", strings.Repeat(" ", 16), "ADMIN_PASSWORD"}, {"insecure cookies", "SECURE_COOKIES", "0", "SECURE_COOKIES"}, {"exposed reset links", "SHOW_RESET_LINKS", "1", "SHOW_RESET_LINKS"}, {"missing storage url", "SUPABASE_URL", "", "SUPABASE_URL"}, {"insecure storage url", "SUPABASE_URL", "http://storage.example", "SUPABASE_URL"}, {"missing storage key", "SUPABASE_SECRET_KEY", "", "SUPABASE_SECRET_KEY"}, {"unknown mode", "APP_ENV", "prod", "APP_ENV"}, {"invalid cookie flag", "SECURE_COOKIES", "true", "SECURE_COOKIES"}, {"invalid reset flag", "SHOW_RESET_LINKS", "true", "SHOW_RESET_LINKS"}} {
		t.Run(test.name, func(t *testing.T) {
			env := map[string]string{}
			for k, v := range baseline {
				env[k] = v
			}
			if test.key != "" {
				env[test.key] = test.value
			}
			err := validate(func(k string) string { return env[k] })
			if test.want == "" && err != nil {
				t.Fatal(err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("configuration: %v", err)
			}
			if err != nil && (strings.Contains(err.Error(), "private-database-value") || strings.Contains(err.Error(), "private-password-value") || strings.Contains(err.Error(), "private-storage-value")) {
				t.Fatal("configuration error exposed a secret")
			}
		})
	}
	for _, mode := range []string{"", "development", "test"} {
		env := map[string]string{"APP_ENV": mode, "SHOW_RESET_LINKS": "1", "SECURE_COOKIES": "0"}
		if err := validate(func(k string) string { return env[k] }); err != nil {
			t.Fatalf("development mode: %v", err)
		}
	}
	for _, mode := range []string{"", "development", "test"} {
		env := map[string]string{"APP_ENV": mode, "RENDER": "true"}
		if !production(func(k string) string { return env[k] }) {
			t.Fatal("Render production detection bypassed")
		}
		if err := validate(func(k string) string { return env[k] }); err == nil {
			t.Fatal("Render silently used memory storage")
		}
	}
}

func TestTrustedProxyConfiguration(t *testing.T) {
	for _, value := range []string{"10.0.0.0/8, 192.168.0.0/16", "2001:db8::/32", ""} {
		if err := validate(func(k string) string {
			if k == "TRUSTED_PROXY_CIDRS" {
				return value
			}
			return ""
		}); err != nil {
			t.Fatalf("valid proxies: %v", err)
		}
	}
	for _, value := range []string{"invalid", "0.0.0.0/0", "::/0", strings.Repeat("10.0.0.0/8,", 33)} {
		if err := validate(func(k string) string {
			if k == "TRUSTED_PROXY_CIDRS" {
				return value
			}
			return ""
		}); err == nil {
			t.Fatal("unsafe proxy configuration accepted")
		}
	}
}
