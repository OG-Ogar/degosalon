package handlers

import (
	"barber-booking/internal/imagevariants"
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPreviewCacheOwnershipAndWidths(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("APP_ENV", "test")
	t.Setenv("RENDER", "false")
	var photo bytes.Buffer
	_ = png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 80, 60)))
	var reads atomic.Int32
	h := newPreviewHandler(func(context.Context, string) ([]byte, error) { reads.Add(1); return photo.Bytes(), nil })
	source := "/uploads/products/0123456789abcdef0123456789abcdef.png"
	for _, method := range []string{"GET", "GET", "HEAD"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/media/preview?"+url.Values{"src": {source}, "width": {"320"}}.Encode(), nil))
		if w.Code != 200 || w.Header().Get("Content-Type") != "image/jpeg" || w.Header().Get("Cache-Control") == "" {
			t.Fatalf("response: %d %v", w.Code, w.Header())
		}
		if method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD returned a body")
		}
	}
	if reads.Load() != 1 {
		t.Fatalf("cache read %d times", reads.Load())
	}
	for _, source := range []string{"https://evil.example/photo.jpg", "/uploads/../secret.jpg", "/uploads/products/photo.jpg?x=y", "/uploads/hairstyle-videos/video.mp4", "/etc/passwd"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/media/preview?"+url.Values{"src": {source}, "width": {"320"}}.Encode(), nil))
		if w.Code != 400 {
			t.Fatalf("accepted %q", source)
		}
	}
	if reads.Load() != 1 {
		t.Fatal("invalid source reached reader")
	}
	if imageSrcset("/uploads/products/animation.gif") != "" || !strings.Contains(imageSrcset(source), "1600w") || !imagevariants.ValidWidth(320) {
		t.Fatal("unexpected candidates")
	}
}
