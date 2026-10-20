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
		name     string
		change   func(*bucketConfig)
		valid    bool
		repaired bool
	}{
		{"valid", func(*bucketConfig) {}, true, false},
		{"private", func(b *bucketConfig) { b.Public = false }, false, false},
		{"wrong identity", func(b *bucketConfig) { b.ID = "other" }, false, false},
		{"wrong name", func(b *bucketConfig) { b.Name = "other" }, false, false},
		{"nonstandard", func(b *bucketConfig) { b.Type = "ANALYTICS" }, false, false},
		{"small", func(b *bucketConfig) { limit := minBucketBytes - 1; b.Limit = &limit }, false, false},
		// A provider-wide cap (50 MB on Supabase's free plan) must not stop
		// startup. Uploads above the real bucket limit fail at upload time.
		{"provider-capped", func(b *bucketConfig) { limit := int64(50 << 20); b.Limit = &limit }, true, false},
		// An older bucket allowlist predates voice notes and chat video; the
		// missing types are appended, and nothing is removed.
		{"images only", func(b *bucketConfig) { b.MIME = []string{"image/*"} }, true, true},
		{"legacy wildcards", func(b *bucketConfig) { b.MIME = []string{"image/*", "video/*"} }, true, true},
		{"unrestricted", func(b *bucketConfig) { b.MIME = nil; b.Limit = nil }, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			bucket := goodBucket()
			test.change(&bucket)
			patched := false
			s := testStore(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/storage/v1/bucket/degosalon-media" {
					t.Error("incorrect bucket path")
				}
				switch r.Method {
				case "GET":
					_ = json.NewEncoder(w).Encode(bucket)
				case "PUT":
					patched = true
					var update struct {
						MIME []string `json:"allowed_mime_types"`
					}
					if json.NewDecoder(r.Body).Decode(&update) != nil {
						t.Error("incorrect patch body")
					}
					for _, kind := range bucket.MIME {
						found := false
						for _, kept := range update.MIME {
							if kept == kind {
								found = true
							}
						}
						if !found {
							t.Error("patch removed an allowed type")
						}
					}
					bucket.MIME = update.MIME
					w.WriteHeader(http.StatusOK)
				default:
					t.Error("unexpected request")
				}
			}, "sb_secret_test-only")
			err := s.EnsureBucket(context.Background(), true)
			if test.valid && err != nil || !test.valid && !errors.Is(err, ErrBucket) {
				t.Fatalf("compatibility result: %v", err)
			}
			if patched != test.repaired {
				t.Fatalf("patched = %v, want %v", patched, test.repaired)
			}
		})
	}
}

func TestBucketLimitRaiseFallsBackToProviderCap(t *testing.T) {
	providerLimit := int64(50 << 20)
	bucket := goodBucket()
	bucket.Limit = &providerLimit
	bucket.MIME = []string{"image/*"} // force the additive repair

	updates := 0
	s := testStore(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			_ = json.NewEncoder(w).Encode(bucket)
		case "PUT":
			updates++
			var update struct {
				MIME  []string `json:"allowed_mime_types"`
				Limit *int64   `json:"file_size_limit"`
			}
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				t.Error("invalid update")
				return
			}
			if update.Limit != nil && *update.Limit > providerLimit {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			bucket.MIME = update.MIME
			if update.Limit != nil {
				bucket.Limit = update.Limit
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Error("unexpected request")
		}
	}, "sb_secret_test-only")

	if err := s.EnsureBucket(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if updates != 2 {
		t.Fatalf("bucket updates = %d, want 2 (raise then fallback)", updates)
	}
	if bucket.Limit == nil || *bucket.Limit != providerLimit {
		t.Fatalf("bucket limit = %v, want %d", bucket.Limit, providerLimit)
	}
	for _, kind := range supportedMIME {
		found := false
		for _, allowed := range bucket.MIME {
			if allowed == kind {
				found = true
			}
		}
		if !found {
			t.Fatalf("MIME type %q was not added", kind)
		}
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
