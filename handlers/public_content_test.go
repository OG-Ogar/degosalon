package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"barber-booking/models"
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
	for _, path := range []string{"/help", "/privacy", "/terms"} {
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

func TestApprovedSupportContacts(t *testing.T) {
	t.Setenv("SITE_URL", "https://salon.example.com")
	req := httptest.NewRequest(http.MethodGet, "http://untrusted.example/support?private=secret", nil)
	req.Header.Set("X-Forwarded-Host", "attacker.example")
	rec := httptest.NewRecorder()
	PublicContentHandler(rec, req)
	body := rec.Body.String()
	if rec.Code != 200 || rec.Header().Get("X-Robots-Tag") != "" {
		t.Fatalf("support: %d %v", rec.Code, rec.Header())
	}
	for _, expected := range []string{`href="tel:+2348128162561">08128162561`, `href="tel:+2349164226427">09164226427`, `href="https://salon.example.com/support"`} {
		if !strings.Contains(body, expected) {
			t.Fatal("missing", expected)
		}
	}
	for _, forbidden := range []string{"Placeholder — awaiting approved content.", "No support contact", "untrusted.example", "attacker.example", "private=secret", "noindex"} {
		if strings.Contains(body, forbidden) {
			t.Fatal("support contains", forbidden)
		}
	}
}

func TestPublicMetadataSocialResolution(t *testing.T) {
	t.Setenv("SITE_URL", "https://salon.example.com")
	t.Setenv("FB_APP_ID", "123456789")

	// 1. Relative image resolution
	meta := publicMetadata("/explore/1", "Shop 1", "Desc 1", "/uploads/site/hero.jpg", "Hero image", "profile", "summary_large_image")
	if meta.Canonical != "https://salon.example.com/explore/1" {
		t.Fatalf("canonical mismatch: %s", meta.Canonical)
	}
	if meta.Image != "https://salon.example.com/uploads/site/hero.jpg" {
		t.Fatalf("expected resolved image URL, got: %s", meta.Image)
	}
	if meta.ImageWidth != 1200 || meta.ImageHeight != 630 {
		t.Fatalf("expected 1200x630 dimensions, got: %dx%d", meta.ImageWidth, meta.ImageHeight)
	}
	if meta.ImageAlt != "Hero image" || meta.Type != "profile" || meta.TwitterCard != "summary_large_image" {
		t.Fatalf("metadata fields mismatch: %+v", meta)
	}
	if meta.FbAppID != "123456789" {
		t.Fatalf("expected FbAppID, got: %s", meta.FbAppID)
	}

	// 2. Unset origin should not invent absolute URL for relative image
	t.Setenv("SITE_URL", "")
	t.Setenv("RENDER_EXTERNAL_URL", "")
	metaEmptyOrigin := publicMetadata("/explore/1", "Shop 1", "Desc 1", "/uploads/site/hero.jpg")
	if metaEmptyOrigin.Image != "" {
		t.Fatalf("expected empty image URL without origin, got: %s", metaEmptyOrigin.Image)
	}

	// 3. Absolute HTTPS image is preserved
	t.Setenv("SITE_URL", "https://salon.example.com")
	metaAbs := publicMetadata("/explore/1", "Shop 1", "Desc 1", "https://cdn.example.com/cut.jpg")
	if metaAbs.Image != "https://cdn.example.com/cut.jpg" {
		t.Fatalf("expected preserved absolute image, got: %s", metaAbs.Image)
	}

	// 4. Insecure or malicious image paths are refused
	for _, bad := range []string{"http://insecure.example/img.jpg", "//protocol-relative.example/img.jpg", "/path\\escape.jpg", "/path#frag.jpg"} {
		if got := resolveMetadataURL("https://salon.example.com", bad); got != "" {
			t.Fatalf("unsafe image path accepted: %s -> %s", bad, got)
		}
	}
}

func TestWebsiteAndSalonSchema(t *testing.T) {
	t.Setenv("SITE_URL", "https://salon.example.com")

	webSchema := string(websiteSchema("/"))
	if !strings.Contains(webSchema, `"@type":"WebSite"`) || !strings.Contains(webSchema, `"SearchAction"`) || !strings.Contains(webSchema, `"https://salon.example.com"`) {
		t.Fatalf("invalid website schema: %s", webSchema)
	}

	barber := models.Barber{
		ID:          10,
		ShopName:    "VIP Grooming Lounge",
		ShopAddress: "42 Allen Avenue",
		LocalArea:   "Ikeja",
		State:       "Lagos",
		ProfileImage: "/uploads/shop10.jpg",
	}
	salon := string(salonSchema(barber, 4.8, 25))
	for _, expected := range []string{`"@type":"HairSalon"`, `"VIP Grooming Lounge"`, `"PostalAddress"`, `"Ikeja"`, `"Lagos"`, `"NG"`, `"AggregateRating"`, `"4.8"`, `"reviewCount":25`, `https://salon.example.com/uploads/shop10.jpg`} {
		if !strings.Contains(salon, expected) {
			t.Fatalf("salon schema missing %s: %s", expected, salon)
		}
	}
}
