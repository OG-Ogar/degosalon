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
