package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxUploadBytes = 5 << 20
	maxVideoBytes  = 50 << 20
	maxFormMemory  = 1 << 20
)

type imageStore struct {
	dir     string
	urlPath string
}

type videoStore struct {
	dir     string
	urlPath string
}

var (
	hairstyleImages = imageStore{
		dir:     "uploads/hairstyles",
		urlPath: "/uploads/hairstyles/",
	}

	hairstyleVideos = videoStore{
		dir:     "uploads/hairstyle-videos",
		urlPath: "/uploads/hairstyle-videos/",
	}

	productImages = imageStore{
		dir:     "uploads/products",
		urlPath: "/uploads/products/",
	}

	barberImages = imageStore{
		dir:     "uploads/barbers",
		urlPath: "/uploads/barbers/",
	}

	customerImages = imageStore{
		dir:     "uploads/customers",
		urlPath: "/uploads/customers/",
	}

	landingImages = imageStore{
		dir:     "uploads/site",
		urlPath: "/uploads/site/",
	}

	customerLandingImages = imageStore{
		dir:     "uploads/site",
		urlPath: "/uploads/site/",
	}
)

var imageStores = []imageStore{
	hairstyleImages,
	productImages,
	barberImages,
	customerImages,
	landingImages,
	customerLandingImages,
}

var allowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

var allowedVideoExtensions = map[string]bool{
	".mp4":  true,
	".webm": true,
	".mov":  true,
}

var errNoUpload = errors.New("no file uploaded")

const (
	landingImageRecord         = "uploads/site/landing-image.txt"
	customerLandingImageRecord = "uploads/site/customer-landing-image.txt"
)

func limitRequestBody(w http.ResponseWriter, r *http.Request) {
	limitRequestBodyFor(w, r, 1)
}

func limitRequestBodyFor(w http.ResponseWriter, r *http.Request, photos int) {
	limitFormBody(
		w,
		r,
		maxUploadBytes*int64(photos)+maxFormMemory,
	)
}

func limitVideoRequestBody(w http.ResponseWriter, r *http.Request) {
	limitFormBody(
		w,
		r,
		maxVideoBytes+maxFormMemory,
	)
}

func (store imageStore) save(r *http.Request, field string) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", errNoUpload
		}

		return "", fmt.Errorf("read upload: %w", err)
	}

	defer file.Close()

	if header.Size > maxUploadBytes {
		return "", fmt.Errorf(
			"image is %d MiB; the limit is %d MiB",
			header.Size>>20,
			maxUploadBytes>>20,
		)
	}

	extension := strings.ToLower(filepath.Ext(header.Filename))

	if !allowedImageExtensions[extension] {
		return "", fmt.Errorf("unsupported image type %q", extension)
	}

	head := make([]byte, 512)

	read, _ := io.ReadFull(file, head)

	if kind := http.DetectContentType(head[:read]); !strings.HasPrefix(kind, "image/") {
		return "", fmt.Errorf("that file is %s, not an image", kind)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind upload: %w", err)
	}

	if err := os.MkdirAll(store.dir, 0o755); err != nil {
		return "", fmt.Errorf("create upload directory: %w", err)
	}

	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), extension)
	path := filepath.Join(store.dir, name)

	destination, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create image file: %w", err)
	}

	if _, err := io.Copy(
		destination,
		io.LimitReader(file, maxUploadBytes),
	); err != nil {
		destination.Close()
		os.Remove(path)

		return "", fmt.Errorf("save image: %w", err)
	}

	if err := destination.Close(); err != nil {
		os.Remove(path)

		return "", fmt.Errorf("close image file: %w", err)
	}

	// Shrink the stored file down to what a page can actually draw.
	//
	// A picture straight off a phone carries several megabytes of detail
	// that the browser downloads and then throws away, so an oversized
	// photo is rewritten here rather than served whole. Header dimensions are
	// checked before decoding; malformed or oversized images are rejected.
	if err := normalizeStoredImage(path); err != nil {
		os.Remove(path)

		return "", fmt.Errorf("resize image: %w", err)
	}

	if storageURL := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"); storageURL != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read stored image: %w", err)
		}
		kind := http.DetectContentType(data)
		objectPath := strings.TrimPrefix(store.urlPath, "/uploads/") + name
		if err := uploadSupabaseMedia(storageURL, objectPath, kind, data); err != nil {
			return "", err
		}
		_ = os.Remove(path)
		return storageURL + "/storage/v1/object/public/degosalon-media/" + escapeObjectPath(objectPath), nil
	}

	return store.urlPath + name, nil
}

func escapeObjectPath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func uploadSupabaseMedia(baseURL, objectPath, contentType string, data []byte) error {
	key := strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY"))
	if key == "" {
		return errors.New("SUPABASE_SERVICE_ROLE_KEY is required when SUPABASE_URL is configured")
	}
	u := baseURL + "/storage/v1/object/degosalon-media/" + escapeObjectPath(objectPath)
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create storage request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("apikey", key)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload image to Supabase Storage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("upload image to Supabase Storage: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (store videoStore) save(r *http.Request, field string) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", errNoUpload
		}

		return "", fmt.Errorf("read video upload: %w", err)
	}

	defer file.Close()

	if header.Size > maxVideoBytes {
		return "", fmt.Errorf(
			"video is %d MiB; the limit is %d MiB",
			header.Size>>20,
			maxVideoBytes>>20,
		)
	}

	extension := strings.ToLower(filepath.Ext(header.Filename))

	if !allowedVideoExtensions[extension] {
		return "", fmt.Errorf("unsupported video type %q", extension)
	}

	head := make([]byte, 512)

	read, _ := io.ReadFull(file, head)

	kind := http.DetectContentType(head[:read])

	if !strings.HasPrefix(kind, "video/") {
		return "", fmt.Errorf("that file is %s, not a video", kind)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind video upload: %w", err)
	}

	if err := os.MkdirAll(store.dir, 0o755); err != nil {
		return "", fmt.Errorf("create video directory: %w", err)
	}

	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), extension)
	path := filepath.Join(store.dir, name)

	destination, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create video file: %w", err)
	}

	if _, err := io.Copy(
		destination,
		io.LimitReader(file, maxVideoBytes),
	); err != nil {
		destination.Close()
		os.Remove(path)

		return "", fmt.Errorf("save video: %w", err)
	}

	if err := destination.Close(); err != nil {
		os.Remove(path)

		return "", fmt.Errorf("close video file: %w", err)
	}

	return store.urlPath + name, nil
}

func removeStoredImage(imageURL string) error {
	if imageURL == "" || strings.HasPrefix(imageURL, "data:image/") {
		return nil
	}

	for _, store := range imageStores {
		if !strings.HasPrefix(imageURL, store.urlPath) {
			continue
		}

		name := strings.TrimPrefix(imageURL, store.urlPath)

		if name == "" ||
			strings.Contains(name, "/") ||
			name == "." ||
			name == ".." {
			return fmt.Errorf("invalid stored image path %q", imageURL)
		}

		path := filepath.Join(store.dir, name)

		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}

			return err
		}

		return nil
	}

	return fmt.Errorf("unknown stored image URL %q", imageURL)
}

func removeStoredVideo(videoURL string) error {
	if videoURL == "" {
		return nil
	}

	if !strings.HasPrefix(videoURL, hairstyleVideos.urlPath) {
		return fmt.Errorf("unknown stored video URL %q", videoURL)
	}

	name := strings.TrimPrefix(videoURL, hairstyleVideos.urlPath)

	if name == "" ||
		strings.Contains(name, "/") ||
		name == "." ||
		name == ".." {
		return fmt.Errorf("invalid stored video path %q", videoURL)
	}

	path := filepath.Join(hairstyleVideos.dir, name)

	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	return nil
}

func removeImageIgnoringError(imageURL string) {
	if imageURL == "" {
		return
	}

	if err := removeStoredImage(imageURL); err != nil {
		log.Printf("remove stored image %q: %v", imageURL, err)
	}
}

func removeVideoIgnoringError(videoURL string) {
	if videoURL == "" {
		return
	}

	if err := removeStoredVideo(videoURL); err != nil {
		log.Printf("remove stored video %q: %v", videoURL, err)
	}
}
