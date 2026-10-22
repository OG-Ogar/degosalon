package observability

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompressGzipSupported(t *testing.T) {
	largeText := strings.Repeat("De Go Salon - Quality Grooming Ecosystem. ", 100)

	handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(largeText))
	}))

	req := httptest.NewRequest(http.MethodGet, "/explore", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res := rec.Result()
	if res.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected Content-Encoding gzip, got %q", res.Header.Get("Content-Encoding"))
	}
	if !strings.Contains(res.Header.Get("Vary"), "Accept-Encoding") {
		t.Fatalf("expected Vary header with Accept-Encoding, got %q", res.Header.Get("Vary"))
	}

	gzReader, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("failed to read decompressed body: %v", err)
	}

	if string(decompressed) != largeText {
		t.Fatalf("decompressed body mismatch")
	}
}

func TestCompressGzipNotRequested(t *testing.T) {
	body := "Plain uncompressed text response"

	handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res := rec.Result()
	if enc := res.Header.Get("Content-Encoding"); enc != "" {
		t.Fatalf("expected no Content-Encoding, got %q", enc)
	}

	data, _ := io.ReadAll(res.Body)
	if string(data) != body {
		t.Fatalf("body mismatch")
	}
}

func TestCompressSkipIncompressibleMedia(t *testing.T) {
	fakeImage := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF, 0xE0}, 200)

	handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(fakeImage)
	}))

	req := httptest.NewRequest(http.MethodGet, "/image.jpg", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res := rec.Result()
	if enc := res.Header.Get("Content-Encoding"); enc != "" {
		t.Fatalf("expected image/jpeg to not be gzipped, got %q", enc)
	}

	data, _ := io.ReadAll(res.Body)
	if !bytes.Equal(data, fakeImage) {
		t.Fatalf("image data altered")
	}
}

func TestCompressFlusher(t *testing.T) {
	handler := Compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("first chunk\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("second chunk\n"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res := rec.Result()
	if res.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip encoding, got %q", res.Header.Get("Content-Encoding"))
	}

	gzReader, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}
	if string(decompressed) != "first chunk\nsecond chunk\n" {
		t.Fatalf("unexpected content: %q", string(decompressed))
	}
}
