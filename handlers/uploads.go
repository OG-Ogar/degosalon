package handlers

import (
	"barber-booking/internal/mediastore"
	"barber-booking/internal/runtimeconfig"
	"barber-booking/storage"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	name, err := mediaName(extension)
	if err != nil {
		return "", err
	}
	path := filepath.Join(store.dir, name)

	destination, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
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

	return persistMedia(r, store.urlPath, name, path)
}

func mediaName(extension string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", errors.New("could not create media identifier")
	}
	return hex.EncodeToString(random[:]) + extension, nil
}

func configuredMediaStore() (*mediastore.Store, error) {
	base := strings.TrimSpace(os.Getenv("SUPABASE_URL"))
	if base == "" {
		if runtimeconfig.Production() {
			return nil, mediastore.ErrConfiguration
		}
		return nil, nil
	}
	return mediastore.FromEnvironment()
}

func persistMedia(r *http.Request, urlPath, name, path string) (string, error) {
	store, err := configuredMediaStore()
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if store == nil {
		return urlPath + name, nil
	}
	defer store.Close()
	// The staging file must be removed on success, rejection and cancellation.
	defer os.Remove(path)
	file, err := os.Open(path)
	if err != nil {
		return "", errors.New("could not read media upload")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", errors.New("could not read media upload")
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return "", errors.New("could not read media upload")
	}
	objectPath := strings.TrimPrefix(urlPath, "/uploads/") + name
	uri, err := store.PublicURL(objectPath)
	if err != nil {
		return "", err
	}
	if err := storage.TrackMediaContext(r.Context(), uri); err != nil {
		return "", mediastore.ErrUpload
	}
	if err := store.Upload(r.Context(), objectPath, http.DetectContentType(head[:n]), info.Size(), file); err != nil {
		return "", err
	}
	return uri, nil
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

	name, err := mediaName(extension)
	if err != nil {
		return "", err
	}
	path := filepath.Join(store.dir, name)

	destination, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
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

	return persistMedia(r, store.urlPath, name, path)
}

func removeStoredImage(imageURL string) error {
	if strings.HasPrefix(imageURL, "https://") {
		return scheduleCloudMedia(imageURL)
	}
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
	if strings.HasPrefix(videoURL, "https://") {
		return scheduleCloudMedia(videoURL)
	}
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
		log.Print("stored image cleanup could not be scheduled")
	}
}

func removeVideoIgnoringError(videoURL string) {
	if videoURL == "" {
		return
	}

	if err := removeStoredVideo(videoURL); err != nil {
		log.Print("stored video cleanup could not be scheduled")
	}
}

func scheduleCloudMedia(uri string) error {
	store, err := configuredMediaStore()
	if err != nil {
		return err
	}
	if store == nil {
		return mediastore.ErrConfiguration
	}
	defer store.Close()
	if !store.Owns(uri) {
		return mediastore.ErrConfiguration
	}
	return storage.TrackMediaContext(context.Background(), uri)
}
