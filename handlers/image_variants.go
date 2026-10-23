package handlers

import (
	"barber-booking/internal/imagevariants"
	"barber-booking/internal/mediastore"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type previewEntry struct {
	data       []byte
	expires    time.Time
	lastAccess time.Time
}
type previewHandler struct {
	mu      sync.Mutex
	entries map[string]previewEntry
	bytes   int
	slots   chan struct{}
	read    func(context.Context, string) ([]byte, error)
}

func newPreviewHandler(read func(context.Context, string) ([]byte, error)) *previewHandler {
	slots := runtime.GOMAXPROCS(0)
	if slots < 2 {
		slots = 2
	} else if slots > 8 {
		slots = 8
	}
	return &previewHandler{entries: map[string]previewEntry{}, slots: make(chan struct{}, slots), read: read}
}

var previews = newPreviewHandler(readPreviewSource)

func ImageVariantHandler(w http.ResponseWriter, r *http.Request) { previews.ServeHTTP(w, r) }

func previewable(uri string) bool {
	ext := strings.ToLower(filepath.Ext(uri))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return false
	}
	if _, ok := mediastore.LegacyObjectPath(uri); ok {
		return true
	}
	// Ownership validation uses configuration only, not a network request.
	store, err := configuredMediaStore()
	if err != nil || store == nil {
		return false
	}
	return store.Owns(uri)
}
func imageSrcset(uri string) string {
	if !previewable(uri) {
		return ""
	}
	var candidates []string
	for _, width := range []int{320, 640, 960, 1600} {
		candidates = append(candidates, "/media/preview?"+url.Values{"src": {uri}, "width": {strconv.Itoa(width)}}.Encode()+" "+strconv.Itoa(width)+"w")
	}
	return strings.Join(candidates, ", ")
}
func (h *previewHandler) evict(neededBytes int) {
	now := time.Now()
	// Step 1: Evict expired entries first.
	for k, e := range h.entries {
		if now.After(e.expires) {
			h.bytes -= len(e.data)
			delete(h.entries, k)
		}
	}
	// Step 2: If still over capacity, evict least recently accessed entries.
	for (h.bytes+neededBytes > 16<<20 || len(h.entries) >= 128) && len(h.entries) > 0 {
		var oldestKey string
		var oldestTime time.Time
		first := true
		for k, e := range h.entries {
			if first || e.lastAccess.Before(oldestTime) {
				oldestKey = k
				oldestTime = e.lastAccess
				first = false
			}
		}
		if oldestKey != "" {
			h.bytes -= len(h.entries[oldestKey].data)
			delete(h.entries, oldestKey)
		} else {
			break
		}
	}
}

func (h *previewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Context().Err() != nil {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", 405)
		return
	}
	uri := r.URL.Query().Get("src")
	width, err := strconv.Atoi(r.URL.Query().Get("width"))
	if err != nil || len(uri) > 2048 || !imagevariants.ValidWidth(width) || !previewable(uri) {
		http.Error(w, "Invalid preview", 400)
		return
	}
	key := uri + "|" + strconv.Itoa(width)
	h.mu.Lock()
	entry, found := h.entries[key]
	if found && time.Now().Before(entry.expires) {
		entry.lastAccess = time.Now()
		h.entries[key] = entry
	}
	h.mu.Unlock()
	if !found || time.Now().After(entry.expires) {
		queue, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		select {
		case h.slots <- struct{}{}:
			defer func() { <-h.slots }()
		case <-queue.Done():
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Preview busy; retry shortly", 503)
			return
		}
		source, readErr := h.read(r.Context(), uri)
		if readErr != nil {
			http.Error(w, "Image unavailable", 404)
			return
		}
		if r.Context().Err() != nil {
			return
		}
		data, encodeErr := imagevariants.JPEG(source, width)
		if encodeErr != nil {
			http.Error(w, "Image unavailable", 422)
			return
		}
		now := time.Now()
		entry = previewEntry{data: data, expires: now.Add(5 * time.Minute), lastAccess: now}
		h.mu.Lock()
		if old, ok := h.entries[key]; ok {
			h.bytes -= len(old.data)
			delete(h.entries, key)
		}
		h.evict(len(data))
		h.entries[key] = entry
		h.bytes += len(data)
		h.mu.Unlock()
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeContent(w, r, "preview.jpg", time.Time{}, bytes.NewReader(entry.data))
}
func readPreviewSource(ctx context.Context, uri string) ([]byte, error) {
	if object, ok := mediastore.LegacyObjectPath(uri); ok {
		path := filepath.Join("uploads", filepath.FromSlash(object))
		root, err := filepath.Abs("uploads")
		if err != nil {
			return nil, err
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, err
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil || !strings.HasPrefix(resolved, root+string(os.PathSeparator)) {
			return nil, errors.New("invalid image path")
		}
		file, err := os.Open(resolved)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 5<<20 {
			return nil, errors.New("invalid image size")
		}
		return io.ReadAll(io.LimitReader(file, (5<<20)+1))
	}
	store, err := configuredMediaStore()
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, mediastore.ErrConfiguration
	}
	return store.ReadPublicImage(ctx, uri)
}
