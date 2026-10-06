package mediastore

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func goodBucket() bucketConfig {
	limit := MaxFileBytes
	return bucketConfig{ID: Bucket, Name: Bucket, Public: true, Type: "STANDARD", Limit: &limit, MIME: supportedMIME}
}
func TestExistingBucketCompatibility(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*bucketConfig)
		valid  bool
	}{
		{"valid", func(*bucketConfig) {}, true},
		{"private", func(b *bucketConfig) { b.Public = false }, false},
		{"wrong identity", func(b *bucketConfig) { b.ID = "other" }, false},
		{"wrong name", func(b *bucketConfig) { b.Name = "other" }, false},
		{"nonstandard", func(b *bucketConfig) { b.Type = "ANALYTICS" }, false},
		{"small", func(b *bucketConfig) { limit := MaxFileBytes - 1; b.Limit = &limit }, false},
		{"images only", func(b *bucketConfig) { b.MIME = []string{"image/*"} }, false},
		{"wildcards", func(b *bucketConfig) { b.MIME = []string{"image/*", "video/*"} }, true},
		{"unrestricted", func(b *bucketConfig) { b.MIME = nil; b.Limit = nil }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			bucket := goodBucket()
			test.change(&bucket)
			s := testStore(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/storage/v1/bucket/degosalon-media" {
					t.Error("existing bucket was modified")
				}
				_ = json.NewEncoder(w).Encode(bucket)
			}, "sb_secret_test-only")
			err := s.EnsureBucket(context.Background(), true)
			if test.valid && err != nil || !test.valid && !errors.Is(err, ErrBucket) {
				t.Fatalf("compatibility result: %v", err)
			}
		})
	}
}

func TestBucketCreationAndReadback(t *testing.T) {
	for _, mode := range []string{"success", "legacy missing", "concurrent creator", "lost acknowledgment", "lost transport acknowledgment", "missing after creation", "private after creation", "check only"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			exists := false
			created := 0
			gets := 0
			s := testStore(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if r.Header.Get("apikey") != "sb_secret_test-only" || r.Header.Get("Authorization") != "" {
					t.Error("incorrect secret credential")
				}
				switch r.Method {
				case "GET":
					gets++
					if !exists {
						if mode == "legacy missing" {
							w.WriteHeader(http.StatusBadRequest)
							_, _ = io.WriteString(w, `{"statusCode":"404","code":"NoSuchBucket"}`)
						} else {
							w.WriteHeader(http.StatusNotFound)
						}
						return
					}
					bucket := goodBucket()
					if mode == "private after creation" {
						bucket.Public = false
					}
					_ = json.NewEncoder(w).Encode(bucket)
				case "POST":
					created++
					if r.URL.Path != "/storage/v1/bucket" {
						t.Error("incorrect create path")
					}
					var config bucketConfig
					if json.NewDecoder(r.Body).Decode(&config) != nil || !compatibleBucket(config) || config.Limit == nil || *config.Limit != MaxFileBytes || len(config.MIME) != len(supportedMIME) {
						t.Error("incorrect bucket specification")
					}
					exists = mode != "missing after creation"
					switch mode {
					case "concurrent creator":
						w.WriteHeader(http.StatusConflict)
					case "lost acknowledgment":
						w.WriteHeader(http.StatusInternalServerError)
					case "lost transport acknowledgment":
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						conn.Close()
						return
					default:
						w.WriteHeader(http.StatusOK)
					}
					_, _ = io.WriteString(w, "private-provider-secret")
				default:
					t.Error("unexpected mutation")
				}
			}, "sb_secret_test-only")
			err := s.EnsureBucket(context.Background(), mode != "check only")
			expected := mode != "missing after creation" && mode != "private after creation" && mode != "check only"
			if expected && err != nil || !expected && !errors.Is(err, ErrBucket) {
				t.Fatalf("readback result: %v", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if mode == "check only" {
				if created != 0 || gets != 1 {
					t.Fatal("check-only mode mutated bucket")
				}
			} else if created != 1 || gets != 2 {
				t.Fatal("create lacked verification")
			}
		})
	}
}

func TestBucketFailureIsSafeAndBounded(t *testing.T) {
	for _, mode := range []string{"unauthorized", "malformed", "oversized", "redirect", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			s := testStore(t, func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "unauthorized":
					w.WriteHeader(http.StatusUnauthorized)
				case "redirect":
					w.Header().Set("Location", "https://other.example")
					w.WriteHeader(http.StatusTemporaryRedirect)
				case "oversized":
					_, _ = io.WriteString(w, strings.Repeat("x", 17<<10))
					return
				}
				_, _ = io.WriteString(w, "private-provider-secret")
			}, "legacy.jwt.test")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			err := s.EnsureBucket(ctx, true)
			if !errors.Is(err, ErrBucket) || strings.Contains(err.Error(), "private-provider-secret") {
				t.Fatalf("unsafe failure: %v", err)
			}
		})
	}
}
