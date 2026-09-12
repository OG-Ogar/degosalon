package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxUploadBytes = 5 << 20
	maxFormMemory  = 1 << 20
)

type imageStore struct {
	dir     string
	urlPath string
}

var (
	hairstyleImages = imageStore{
		dir:     "uploads/hairstyles",
		urlPath: "/uploads/hairstyles/",
	}

	barberImages = imageStore{
		dir:     "uploads/barbers",
		urlPath: "/uploads/barbers/",
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
	barberImages,
	landingImages,
	customerLandingImages,
}

var allowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png": true,
	".gif": true,
	".webp": true,
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
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxUploadBytes*int64(photos)+maxFormMemory,
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

	if err := normalizeStoredImage(path); err != nil {
		log.Printf("resize %s: %v", name, err)
	}

	return store.urlPath + name, nil
}

func removeStoredImage(imageURL string) error {
	if imageURL == "" {
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

func removeImageIgnoringError(imageURL string) {
	if imageURL == "" {
		return
	}

	if err := removeStoredImage(imageURL); err != nil {
		log.Printf("remove stored image %q: %v", imageURL, err)
	}
}

func saveLandingImage(r *http.Request) (string, error) {
	image, err := landingImages.save(r, "landing_image")

	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(landingImages.dir, 0o755); err != nil {
		removeImageIgnoringError(image)

		return "", fmt.Errorf("create landing image directory: %w", err)
	}

	if err := os.WriteFile(
		landingImageRecord,
		[]byte(image),
		0o644,
	); err != nil {
		removeImageIgnoringError(image)

		return "", fmt.Errorf("remember landing image: %w", err)
	}

	return image, nil
}

func currentLandingImage() string {
	data, err := os.ReadFile(landingImageRecord)

	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("read landing image record: %v", err)
		}

		return ""
	}

	image := strings.TrimSpace(string(data))

	if image == "" {
		return ""
	}

	return image
}

func saveCustomerLandingImage(r *http.Request) (string, error) {
	image, err := customerLandingImages.save(r, "customer_landing_image")

	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(customerLandingImages.dir, 0o755); err != nil {
		removeImageIgnoringError(image)

		return "", fmt.Errorf(
			"create customer landing image directory: %w",
			err,
		)
	}

	oldImage := currentCustomerLandingImage()

	if err := os.WriteFile(
		customerLandingImageRecord,
		[]byte(image),
		0o644,
	); err != nil {
		removeImageIgnoringError(image)

		return "", fmt.Errorf(
			"remember customer landing image: %w",
			err,
		)
	}

	if oldImage != "" && oldImage != image {
		removeImageIgnoringError(oldImage)
	}

	return image, nil
}

func currentCustomerLandingImage() string {
	data, err := os.ReadFile(customerLandingImageRecord)

	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf(
				"read customer landing image record: %v",
				err,
			)
		}

		return ""
	}

	image := strings.TrimSpace(string(data))

	if image == "" {
		return ""
	}

	return image
}