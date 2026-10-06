package handlers

import (
	"barber-booking/internal/mediastore"
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMediaStagingCleanupAndDevelopmentFallback(t *testing.T) {
	for _, mode := range []string{"development", "missing-production", "invalid-cloud", "cancelled-cloud"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("APP_ENV", "development")
			t.Setenv("RENDER", "")
			t.Setenv("SUPABASE_URL", "")
			t.Setenv("SUPABASE_SECRET_KEY", "")
			t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
			switch mode {
			case "missing-production":
				t.Setenv("APP_ENV", "production")
			case "invalid-cloud":
				t.Setenv("SUPABASE_URL", "https://storage.example")
			case "cancelled-cloud":
				t.Setenv("SUPABASE_URL", "https://storage.example")
				t.Setenv("SUPABASE_SECRET_KEY", "sb_secret_test-only")
			}
			path := filepath.Join(t.TempDir(), "abcd.png")
			if err := os.WriteFile(path, []byte("\x89PNG\r\n\x1a\n"), 0600); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", "/upload", nil)
			if mode == "cancelled-cloud" {
				ctx, cancel := context.WithCancel(r.Context())
				cancel()
				r = r.WithContext(ctx)
			}
			uri, err := persistMedia(r, "/uploads/site/", "abcd.png", path)
			_, statErr := os.Stat(path)
			if mode == "development" {
				if err != nil || uri != "/uploads/site/abcd.png" || statErr != nil {
					t.Fatalf("development persistence: %q %v %v", uri, err, statErr)
				}
			} else {
				if err == nil || uri != "" || !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("failed upload left staging file: %q %v %v", uri, err, statErr)
				}
				if mode == "cancelled-cloud" && !errors.Is(err, mediastore.ErrUpload) {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestMediaCredentialPreference(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://storage.example")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "bad\nlegacy-key")
	t.Setenv("SUPABASE_SECRET_KEY", "sb_secret_test-only")
	if _, err := configuredMediaStore(); err != nil {
		t.Fatalf("preferred secret key not used: %v", err)
	}
	t.Setenv("SUPABASE_SECRET_KEY", "")
	if _, err := configuredMediaStore(); !errors.Is(err, mediastore.ErrConfiguration) {
		t.Fatal("invalid legacy key accepted")
	}
}

func TestCloudCleanupOwnership(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://media.example")
	t.Setenv("SUPABASE_SECRET_KEY", "sb_secret_test-only")
	image := "https://media.example/storage/v1/object/public/degosalon-media/site/0123456789abcdef0123456789abcdef.png"
	video := "https://media.example/storage/v1/object/public/degosalon-media/hairstyle-videos/0123456789abcdef0123456789abcdef.mp4"
	if err := removeStoredImage(image); err != nil {
		t.Fatal(err)
	}
	if err := removeStoredVideo(video); err != nil {
		t.Fatal(err)
	}
	for _, foreign := range []string{"https://other.example/image.png", image + "?credential=secret", "https://media.example/storage/v1/object/public/other-bucket/site/0123456789abcdef0123456789abcdef.png"} {
		if err := removeStoredImage(foreign); err == nil {
			t.Fatal("foreign image scheduled")
		}
		if err := removeStoredVideo(foreign); err == nil {
			t.Fatal("foreign video scheduled")
		}
	}
}
