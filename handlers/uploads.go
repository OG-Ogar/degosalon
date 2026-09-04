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
	// maxUploadBytes caps a single image. Without a cap, one request
	// could buffer an unbounded amount of data.
	maxUploadBytes = 5 << 20

	// maxFormMemory is how much of a multipart form is held in memory
	// before the rest spills to a temporary file.
	maxFormMemory = 1 << 20
)

// imageStore is a directory of uploaded images together with the URL
// prefix it is served under. The two kinds of photo are kept apart so
// that deleting one can never reach a file belonging to the other.
type imageStore struct {
	dir     string
	urlPath string
}

var (
	// hairstyleImages holds the photo attached to one hairstyle.
	hairstyleImages = imageStore{
		dir:     "uploads/hairstyles",
		urlPath: "/uploads/hairstyles/",
	}

	// barberImages holds both photos on a barber's profile: the shop and
	// the barber themselves.
	barberImages = imageStore{
		dir:     "uploads/barbers",
		urlPath: "/uploads/barbers/",
	}
)

// imageStores is every store there is, used to match a stored path back
// to the directory that owns it.
var imageStores = []imageStore{hairstyleImages, barberImages}

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
	limitRequestBodyFor(w, r, 1)
}

// limitRequestBodyFor is the same cap for a form carrying more than one
// file input. A single body has to hold every photo the form offers, or
// filling all of them in at once is rejected as too large.
func limitRequestBodyFor(w http.ResponseWriter, r *http.Request, photos int) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxUploadBytes*int64(photos)+maxFormMemory,
	)
}

// save stores the image in the named form field and returns the public
// path to it.
//
// The extension is checked against a whitelist and the first bytes are
// sniffed, so a script renamed to .png is rejected rather than saved
// into a directory the server hands straight back to browsers.
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

	if _, err := io.Copy(destination, io.LimitReader(file, maxUploadBytes)); err != nil {
		destination.Close()
		os.Remove(path)

		return "", fmt.Errorf("save image: %w", err)
	}

	if err := destination.Close(); err != nil {
		os.Remove(path)

		return "", fmt.Errorf("close image file: %w", err)
	}

	// A photo that could not be shrunk is still a usable photo, so this
	// is logged rather than failing an upload that has already succeeded.
	// replaceImageFile writes beside the original, so what stays on disk
	// either way is a complete image.
	if err := normalizeStoredImage(path); err != nil {
		log.Printf("resize %s: %v", name, err)
	}

	return store.urlPath + name, nil
}

// removeStoredImage deletes an uploaded image from whichever store owns
// it.
//
// It refuses any path that is not a plain file name directly under one of
// the upload directories. The path is attacker-influenced (a barber
// submits it when editing a hairstyle), so passing it to os.Remove
// unchecked would let a crafted value delete unrelated files.
func removeStoredImage(webPath string) error {
	if webPath == "" {
		return nil
	}

	for _, store := range imageStores {
		if !strings.HasPrefix(webPath, store.urlPath) {
			continue
		}

		name := strings.TrimPrefix(webPath, store.urlPath)

		if name == "" || name != filepath.Base(name) {
			return fmt.Errorf("refusing to delete %q: not a plain file name", webPath)
		}

		if err := os.Remove(filepath.Join(store.dir, name)); err != nil &&
			!os.IsNotExist(err) {

			return fmt.Errorf("delete %s: %w", name, err)
		}

		return nil
	}

	return fmt.Errorf("refusing to delete %q: outside the upload directories", webPath)
}

// removeImageIgnoringError deletes a stored image when the caller is
// already reporting a different failure, so the file left behind by an
// abandoned write does not become a leak. The delete itself is logged
// rather than returned, because the original error is the useful one.
func removeImageIgnoringError(webPath string) {
	if err := removeStoredImage(webPath); err != nil {
		log.Printf("clean up unused image: %v", err)
	}
}
