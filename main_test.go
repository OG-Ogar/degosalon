package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRevalidateAsksBrowsersToCheckWithTheServer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		w.WriteHeader(http.StatusNotModified)
	})

	recorder := httptest.NewRecorder()
	revalidate(next).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/static/app.js", nil))

	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
	if recorder.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotModified)
	}
}

func TestRevalidateReturnsNotModifiedOnMatchingETag(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	rec1 := httptest.NewRecorder()
	revalidate(next).ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/static/app.css", nil))
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header on static response")
	}

	called = false
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	req2.Header.Set("If-None-Match", etag)
	revalidate(next).ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", rec2.Code, http.StatusNotModified)
	}
	if called {
		t.Fatal("next handler should not be invoked when ETag matches")
	}
}

func TestProductionStartupRejectsMemoryBeforeServing(t *testing.T) {
	if os.Getenv("DEGOSALON_STARTUP_TEST") == "1" {
		main()
		return
	}
	t.Setenv("DEGOSALON_STARTUP_TEST", "1")
	t.Setenv("APP_ENV", "production")
	t.Setenv("RENDER", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "")
	cmd := exec.Command(os.Args[0], "-test.run=^TestProductionStartupRejectsMemoryBeforeServing$")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "DATABASE_URL is required in production") || strings.Contains(string(output), "server listening") {
		t.Fatalf("unsafe production startup: %v %s", err, output)
	}
}
