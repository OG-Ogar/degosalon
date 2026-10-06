package handlers

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"barber-booking/models"
	"barber-booking/session"
)

func TestMutationFormBoundsBeforeCSRF(t *testing.T) {
	s := session.Session{CSRFToken: "valid-token"}
	for _, chunked := range []bool{false, true} {
		r := authFormRequest("/customer/book", url.Values{"csrf_token": {s.CSRFToken}, "padding": {strings.Repeat("x", maxMutationFormBytes)}})
		if chunked {
			r.ContentLength = -1
		}
		w := httptest.NewRecorder()
		if requireCSRF(w, r, s) || w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("large mutation form: %d", w.Code)
		}
	}
	r := authFormRequest("/customer/book", url.Values{"csrf_token": {s.CSRFToken}})
	if !requireCSRF(httptest.NewRecorder(), r, s) {
		t.Fatal("ordinary valid form rejected")
	}
	r = authFormRequest("/customer/profile/edit", url.Values{"csrf_token": {s.CSRFToken}, "padding": {strings.Repeat("x", maxMutationFormBytes)}})
	w := httptest.NewRecorder()
	limitRequestBodyFor(w, r, 1)
	if requireCSRF(w, r, s) || w.Code != http.StatusRequestEntityTooLarge {
		t.Fatal("upload route enlarged non-file form allowance")
	}
}

func TestCustomerMultipartBoundPrecedesCSRFParsing(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("csrf_token", "valid-token")
	_ = writer.WriteField("padding", strings.Repeat("x", maxUploadBytes+maxFormMemory))
	_ = writer.Close()
	for _, chunked := range []bool{false, true} {
		r := httptest.NewRequest(http.MethodPost, "/customer/profile/edit", bytes.NewReader(body.Bytes()))
		r.Header.Set("Content-Type", writer.FormDataContentType())
		if chunked {
			r.ContentLength = -1
		}
		w := httptest.NewRecorder()
		EditCustomerProfileHandler(w, r, models.Customer{ID: 1}, session.Session{CSRFToken: "valid-token"})
		cleanupMultipart(r)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("multipart limit applied too late: %d", w.Code)
		}
	}
}

func TestMalformedMultipartDoesNotPassValidCSRF(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/customer/book", strings.NewReader("--bad\r\ninvalid"))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
	w := httptest.NewRecorder()
	if requireCSRF(w, r, session.Session{CSRFToken: "valid-token"}) || w.Code != http.StatusBadRequest {
		t.Fatalf("invalid multipart: %d", w.Code)
	}
}

func TestMultipartFilesRemainReadableUntilCleanup(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("csrf_token", "valid-token")
	part, _ := writer.CreateFormFile("image", "large.png")
	_, _ = part.Write(bytes.Repeat([]byte{'x'}, maxFormMemory+1))
	_ = writer.Close()
	r := httptest.NewRequest(http.MethodPost, "/barber/profile/edit", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	limitRequestBodyFor(httptest.NewRecorder(), r, 2)
	if !requireCSRF(httptest.NewRecorder(), r, session.Session{CSRFToken: "valid-token"}) {
		t.Fatal("upload route inherited ordinary form limit")
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	entries, err := os.ReadDir(os.Getenv("TMPDIR"))
	if err != nil || len(entries) == 0 {
		t.Fatal("fixture did not spill multipart file to disk")
	}
	cleanupMultipart(r)
	entries, err = os.ReadDir(os.Getenv("TMPDIR"))
	if err != nil || len(entries) != 0 {
		t.Fatal("multipart temporary file retained")
	}
}

func TestHighlyCompressedOversizedImageRejected(t *testing.T) {
	// A real compressed 16.7 MP grayscale image is small on disk but would
	// allocate its full pixel buffer if header checks followed full decoding.
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewGray(image.Rect(0, 0, 4096, 4096))); err != nil {
		t.Fatal(err)
	}
	if buffer.Len() > 100<<10 {
		t.Fatal("fixture is not highly compressed")
	}
	path := filepath.Join(t.TempDir(), "large.png")
	if err := os.WriteFile(path, buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeStoredImage(path); err == nil || !strings.Contains(err.Error(), "16 megapixels") {
		t.Fatalf("pixel budget ignored: %v", err)
	}
}

func TestHeaderDimensionsAndContentTypeAreValidated(t *testing.T) {
	var original bytes.Buffer
	if err := png.Encode(&original, image.NewGray(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	for _, dimensions := range [][2]uint32{{10001, 1}, {1, 10001}, {2147483647, 2147483647}} {
		data := append([]byte(nil), original.Bytes()...)
		binary.BigEndian.PutUint32(data[16:20], dimensions[0])
		binary.BigEndian.PutUint32(data[20:24], dimensions[1])
		binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
		path := filepath.Join(t.TempDir(), "header.png")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := normalizeStoredImage(path); err == nil {
			t.Fatal("oversized header accepted")
		}
	}
	path := filepath.Join(t.TempDir(), "wrong.jpg")
	if err := os.WriteFile(path, original.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeStoredImage(path); err == nil {
		t.Fatal("extension/content mismatch accepted")
	}
	path = filepath.Join(t.TempDir(), "corrupt.png")
	if err := os.WriteFile(path, original.Bytes()[:33], 0600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeStoredImage(path); err == nil {
		t.Fatal("undecodable PNG retained")
	}
	// WebP extended headers declare a canvas before any image/frame payload.
	webp := make([]byte, 30)
	copy(webp, "RIFF")
	binary.LittleEndian.PutUint32(webp[4:8], 22)
	copy(webp[8:], "WEBPVP8X")
	binary.LittleEndian.PutUint32(webp[16:20], 10)
	webp[24] = 0x10
	webp[25] = 0x27 // width minus one = 10000
	path = filepath.Join(t.TempDir(), "huge.webp")
	if err := os.WriteFile(path, webp, 0600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeStoredImage(path); err == nil {
		t.Fatal("WebP dimensions unchecked")
	}
}
