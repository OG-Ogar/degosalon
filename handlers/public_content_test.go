package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicMetadataTrustedOrigin(t *testing.T) {
	t.Setenv("SITE_URL", "")
	t.Setenv("RENDER_EXTERNAL_URL", "")
	if got := publicMetadata("/explore", "Title", "Description"); got.Canonical != "" {
		t.Fatal("invented origin", got)
	}
	t.Setenv("RENDER_EXTERNAL_URL", "https://salon.onrender.com")
	if got := publicMetadata("/explore", "Title", "Description"); got.Canonical != "https://salon.onrender.com/explore" {
		t.Fatal(got)
	}
	t.Setenv("SITE_URL", "https://salon.example.com/")
	if got := publicMetadata("/explore/123", "Title", "Description"); got.Canonical != "https://salon.example.com/explore/123" {
		t.Fatal(got)
	}
	for _, origin := range []string{"http://salon.example.com", "https://user:password@salon.example.com", "https://salon.example.com/path", "https://salon.example.com?", "https://salon.example.com#", "https://salon.example.com?secret=x", "https://salon.example.com/#secret", "//attacker.example", "https://"} {
		t.Setenv("SITE_URL", origin)
		if got := publicMetadata("/explore", "Title", "Description"); got.Canonical != "" {
			t.Fatalf("unsafe origin accepted %s", origin)
		}
	}
	t.Setenv("SITE_URL", "https://salon.example.com")
	for _, path := range []string{"//attacker.example", "/explore?state=Lagos", "/explore#private", "/\\attacker.example"} {
		if got := publicMetadata(path, "Title", "Description"); got.Canonical != "" {
			t.Fatalf("unsafe path accepted %s", path)
		}
	}
}
func TestPublicPlaceholderPages(t *testing.T) {
	t.Setenv("SITE_URL", "https://salon.example.com")
	for _, path := range []string{"/help", "/support", "/privacy", "/terms"} {
		req := httptest.NewRequest(http.MethodGet, "http://untrusted.example"+path, nil)
		req.Header.Set("X-Forwarded-Host", "attacker.example")
		rec := httptest.NewRecorder()
		PublicContentHandler(rec, req)
		body := rec.Body.String()
		if rec.Code != 200 || rec.Header().Get("X-Robots-Tag") != "noindex, follow" {
			t.Fatalf("%s %d", path, rec.Code)
		}
		for _, expected := range []string{"Placeholder — awaiting approved content.", `href="https://salon.example.com` + path + `"`, `property="og:title"`, `name="twitter:card"`, `aria-label="Help and policies"`} {
			if !strings.Contains(body, expected) {
				t.Fatalf("%s missing %s", path, expected)
			}
		}
		if strings.Contains(body, "untrusted.example") || strings.Contains(body, "attacker.example") {
			t.Fatal("request host leaked into metadata")
		}
		if strings.Count(body, `name="description"`) != 1 {
			t.Fatal("duplicate description")
		}
		rec = httptest.NewRecorder()
		PublicContentHandler(rec, httptest.NewRequest(http.MethodPost, path, nil))
		if rec.Code != 405 || rec.Header().Get("Allow") != "GET" {
			t.Fatal("method contract")
		}
	}
	rec := httptest.NewRecorder()
	PublicContentHandler(rec, httptest.NewRequest(http.MethodGet, "/support/unknown", nil))
	if rec.Code != 404 {
		t.Fatal("unknown content route")
	}
}
