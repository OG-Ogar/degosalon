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
	// uploadDir is where hairstyle images live on disk, relative to the
	// working directory the server was started from.
	uploadDir = "uploads/hairstyles"

	// uploadURLPath is the public prefix those files are served under.
	uploadURLPath = "/uploads/hairstyles/"

	// maxUploadBytes caps a single image. Without a cap, one request
	// could buffer an unbounded amount of data.
	maxUploadBytes = 5 << 20

	// maxFormMemory is how much of a multipart form is held in memory
	// before the rest spills to a temporary file.
	maxFormMemory = 1 << 20
)

// allowedImageExtensions is the set of extensions accepted for uploads.
var allowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

// errNoUpload reports that the form carried no file in that field,
// which is normal when editing a record and keeping the current image.
var errNoUpload = errors.New("no file uploaded")

// limitRequestBody caps the size of a request before any form parsing
// happens. Call it first in handlers that accept uploads.
func limitRequestBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+maxFormMemory)
}

// saveUploadedImage stores the image in the named form field and returns
// the public path to it.
//
// The extension is checked against a whitelist and the first bytes are
// sniffed, so a script renamed to .png is rejected rather than saved
// into a directory the server hands straight back to browsers.
func saveUploadedImage(r *http.Request, field string) (string, error) {
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

	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", fmt.Errorf("create upload directory: %w", err)
	}

	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), extension)
	path := filepath.Join(uploadDir, name)

	destination, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create image file: %w", err)
	}

	if _, err := io.Copy(destination, io.LimitReader(file, maxUploadBytes)); err != nil {
		destination.Close()
		os.Remove(path)

		return "", fmt.Errorf("save image: %w", err)
	}

	if err := destination.Close(); err != nil {
		os.Remove(path)

		return "", fmt.Errorf("close image file: %w", err)
	}

	return uploadURLPath + name, nil
}

// removeUploadedImage deletes a stored image from disk.
//
// It refuses any path that is not a plain file name directly under the
// upload directory. The image path is attacker-influenced (a barber
// submits it when editing a hairstyle), so passing it to os.Remove
// unchecked would let a crafted value delete unrelated files.
func removeUploadedImage(webPath string) error {
	if webPath == "" {
		return nil
	}

	if !strings.HasPrefix(webPath, uploadURLPath) {
		return fmt.Errorf("refusing to delete %q: outside %s", webPath, uploadURLPath)
	}

	name := strings.TrimPrefix(webPath, uploadURLPath)

	if name == "" || name != filepath.Base(name) {
		return fmt.Errorf("refusing to delete %q: not a plain file name", webPath)
	}

	if err := os.Remove(filepath.Join(uploadDir, name)); err != nil &&
		!os.IsNotExist(err) {

		return fmt.Errorf("delete %s: %w", name, err)
	}

	return nil
}

// removeImageIgnoringError deletes a stored image when the caller is
// already reporting a different failure, so the file left behind by an
// abandoned write does not become a leak. The delete itself is logged
// rather than returned, because the original error is the useful one.
func removeImageIgnoringError(webPath string) {
	if err := removeUploadedImage(webPath); err != nil {
		log.Printf("clean up unused image: %v", err)
	}
}
