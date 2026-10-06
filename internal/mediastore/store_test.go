package mediastore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testStore(t *testing.T, handler http.HandlerFunc, key string) *Store {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	s, err := New(server.URL, key)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the production redirect policy while trusting this test certificate.
	s.client.Transport = server.Client().Transport
	t.Cleanup(s.Close)
	return s
}

func TestUploadCredentialsAndStreaming(t *testing.T) {
	for _, key := range []string{"sb_secret_test-only", "legacy.jwt.test"} {
		t.Run(key, func(t *testing.T) {
			payload := bytes.Repeat([]byte{42}, 7<<20)
			s := testStore(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/storage/v1/object/degosalon-media/hairstyle-videos/abcd.mp4" {
					t.Error("unexpected request")
				}
				if r.Header.Get("apikey") != key || r.Header.Get("x-upsert") != "false" || r.Header.Get("Content-Type") != "video/mp4" || r.ContentLength != int64(len(payload)) {
					t.Error("incorrect upload headers")
				}
				want := "Bearer " + key
				if strings.HasPrefix(key, "sb_secret_") {
					want = ""
				}
				if r.Header.Get("Authorization") != want {
					t.Error("incorrect credential scheme")
				}
				got, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(got, payload) {
					t.Error("upload body differs")
				}
				w.WriteHeader(http.StatusCreated)
			}, key)
			if err := s.Upload(context.Background(), "hairstyle-videos/abcd.mp4", "video/mp4", int64(len(payload)), bytes.NewReader(payload)); err != nil {
				t.Fatal(err)
			}
			uri, err := s.PublicURL("hairstyle-videos/abcd.mp4")
			if err != nil || !strings.Contains(uri, "/object/public/degosalon-media/hairstyle-videos/abcd.mp4") {
				t.Fatalf("public URL: %q %v", uri, err)
			}
		})
	}
}

func TestUploadRefusesRedirectAndHidesProviderErrors(t *testing.T) {
	var forwarded atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Store(true) }))
	defer target.Close()
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError} {
		s := testStore(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", target.URL)
			w.WriteHeader(status)
			_, _ = io.WriteString(w, "private-provider-details sb_secret_test-only")
		}, "sb_secret_test-only")
		err := s.Upload(context.Background(), "site/abcd.png", "image/png", 1, strings.NewReader("x"))
		if !errors.Is(err, ErrUpload) || strings.Contains(err.Error(), "private-provider") || strings.Contains(err.Error(), "sb_secret") {
			t.Fatalf("unsafe error: %v", err)
		}
	}
	if forwarded.Load() {
		t.Fatal("redirect forwarded server credential")
	}
}

func TestUploadHonorsCancellationAndDeadline(t *testing.T) {
	for _, preCancelled := range []bool{true, false} {
		release := make(chan struct{})
		s := testStore(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}, "sb_secret_test-only")
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		if preCancelled {
			cancel()
		}
		start := time.Now()
		err := s.Upload(ctx, "site/abcd.png", "image/png", 1, strings.NewReader("x"))
		cancel()
		close(release)
		if !errors.Is(err, ErrUpload) || time.Since(start) > time.Second {
			t.Fatalf("cancellation not bounded: %v", err)
		}
	}
}

func TestConfigurationAndObjectPathValidation(t *testing.T) {
	for _, base := range []string{"", "http://storage.example", "https://user:password@storage.example", "https://storage.example/path", "https://storage.example?key=secret", "https://storage.example?", "https://storage.example/#fragment", "https://storage.example/%2f"} {
		if _, err := New(base, "key"); !errors.Is(err, ErrConfiguration) {
			t.Fatalf("accepted invalid origin %q", base)
		}
	}
	for _, key := range []string{"", " key", "key\r\nother", strings.Repeat("a", 4097)} {
		if _, err := New("https://storage.example", key); !errors.Is(err, ErrConfiguration) {
			t.Fatal("accepted invalid credential")
		}
	}
	s, err := New("https://storage.example/", "key")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"site/../secret", "site/..", "other/file.png", "site/file?key", "site/file#fragment", "site/file%2fsecret", "site/file\\name.png", "site/", "/site/file.png"} {
		if _, err := s.PublicURL(path); !errors.Is(err, ErrConfiguration) {
			t.Fatalf("accepted path %q", path)
		}
	}
	if s.client.Timeout != Timeout || s.client.CheckRedirect == nil {
		t.Fatal("unbounded HTTP client")
	}
}

func TestOwnedObjectDeletion(t *testing.T) {
	for _, key := range []string{"sb_secret_test-only", "legacy.jwt.test"} {
		t.Run(key, func(t *testing.T) {
			var requests atomic.Int32
			s := testStore(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != "DELETE" || r.URL.Path != "/storage/v1/object/degosalon-media" || r.Header.Get("apikey") != key {
					t.Error("incorrect delete request")
				}
				expected := "Bearer " + key
				if strings.HasPrefix(key, "sb_secret_") {
					expected = ""
				}
				if r.Header.Get("Authorization") != expected {
					t.Error("incorrect delete credential")
				}
				data, err := io.ReadAll(r.Body)
				if err != nil || string(data) != `{"prefixes":["site/0123456789abcdef0123456789abcdef.png"]}` {
					t.Errorf("unexpected delete body: %s", data)
				}
				_, _ = io.WriteString(w, "[]")
			}, key)
			uri := s.base + "/storage/v1/object/public/degosalon-media/site/0123456789abcdef0123456789abcdef.png"
			for range 2 {
				if err := s.Delete(context.Background(), uri); err != nil {
					t.Fatal(err)
				}
			}
			for _, invalid := range []string{
				"https://another.example/storage/v1/object/public/degosalon-media/site/0123456789abcdef0123456789abcdef.png",
				uri + "?secret=key", uri + "#fragment", strings.Replace(uri, "/site/", "/site/../", 1), strings.Replace(uri, "degosalon-media", "private-bucket", 1), strings.Replace(uri, "/site/", "/unowned/", 1), strings.Replace(uri, ".png", ".mp4", 1), strings.Replace(uri, "0123456789abcdef0123456789abcdef", "arbitrary-file", 1), strings.Replace(uri, "/site/", "/site%2f", 1),
			} {
				if s.Owns(invalid) {
					t.Fatalf("claimed foreign URL %q", invalid)
				}
				if err := s.Delete(context.Background(), invalid); !errors.Is(err, ErrConfiguration) {
					t.Fatal("foreign object deletion allowed")
				}
			}
			if requests.Load() != 2 {
				t.Fatal("invalid URL reached provider")
			}
		})
	}
}

func TestDeleteRetainsSafeErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError, http.StatusTemporaryRedirect} {
		s := testStore(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "https://another.example/")
			w.WriteHeader(status)
			_, _ = io.WriteString(w, "private provider token sb_secret_test-only")
		}, "sb_secret_test-only")
		uri := s.base + "/storage/v1/object/public/degosalon-media/hairstyle-videos/0123456789abcdef0123456789abcdef.mp4"
		if err := s.Delete(context.Background(), uri); !errors.Is(err, ErrDelete) {
			t.Fatalf("provider failure hidden or leaked: %v", err)
		}
	}
}

func TestLegacyObjectPath(t *testing.T) {
	for _, uri := range []string{"/uploads/site/legacy.png", "/uploads/barbers/1234567890123456789.jpeg", "/uploads/hairstyle-videos/legacy.mp4"} {
		if path, ok := LegacyObjectPath(uri); !ok || path == "" {
			t.Fatalf("legacy path rejected: %q", uri)
		}
	}
	for _, uri := range []string{"/uploads/site/../secret.png", "/uploads/site/x.png?token=secret", "/uploads/site/x.png#fragment", "/uploads/site%2fsecret.png", "/uploads/site/.png", "/uploads/site/x.mp4", "/uploads/hairstyle-videos/x.png", "/uploads/unknown/x.png", "https://external.example/x.png", "/uploads/site/x\\y.png"} {
		if _, ok := LegacyObjectPath(uri); ok {
			t.Fatalf("unsafe legacy path accepted: %q", uri)
		}
	}
}
