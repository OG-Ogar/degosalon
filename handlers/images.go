package handlers

import (
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	// maxImageEdge bounds the longest side of a stored photo. The widest
	// any page draws one is a little under 900 pixels, so a picture
	// straight off a phone carries several megabytes of detail that the
	// browser downloads and then throws away.
	maxImageEdge = 1600

	// maxImageDimension and maxImagePixels bound what may be decoded.
	// A modern phone camera produces 12 to 50 megapixel pictures, so the
	// budget is generous: the picture is resized and re-encoded here
	// instead of being refused. The limits still stop a malformed header
	// from asking for an impossible allocation.
	maxImageDimension = 20000
	maxImagePixels    = 64_000_000

	// jpegQuality is the re-encode setting: the usual point where the
	// file gets much smaller without the loss being visible.
	jpegQuality = 82

	// keepAsIsBytes is the size below which a photo that already fits is
	// left exactly as it arrived, since re-encoding it could only lose
	// detail for no useful saving.
	keepAsIsBytes = 300 << 10
)

// normalizeStoredImage rewrites a saved upload at a sensible size.
//
// JPEGs are re-encoded whenever they are oversized or large, because a
// camera file is saved at a quality nothing here needs. PNGs are only
// touched when their dimensions are too big; re-encoding one otherwise
// gains almost nothing. GIF and WEBP retain their encoded content after header
// validation, preserving animation. Every supported format has bounded header
// dimensions before a full JPEG/PNG decode can allocate its pixel buffer.
func normalizeStoredImage(path string) error {
	extension := strings.ToLower(filepath.Ext(path))

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("measure image: %w", err)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer file.Close()

	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return fmt.Errorf("read image dimensions: %w", err)
	}

	if config.Width < 1 ||
		config.Height < 1 ||
		config.Width > maxImageDimension ||
		config.Height > maxImageDimension ||
		config.Width > maxImagePixels/config.Height {
		return fmt.Errorf(
			"image exceeds the limit of %d megapixels or %d pixels per side",
			maxImagePixels/1_000_000,
			maxImageDimension,
		)
	}

	// Verify that the filename extension matches the actual image format.
	// JPEG files may use either .jpg or .jpeg.
	switch format {
	case "jpeg":
		if extension != ".jpg" && extension != ".jpeg" {
			return fmt.Errorf(
				"image extension %q does not match detected format %q",
				extension,
				format,
			)
		}
		extension = ".jpg"

	case "png":
		if extension != ".png" {
			return fmt.Errorf(
				"image extension %q does not match detected format %q",
				extension,
				format,
			)
		}
		extension = ".png"

	case "gif":
		if extension != ".gif" {
			return fmt.Errorf(
				"image extension %q does not match detected format %q",
				extension,
				format,
			)
		}
		extension = ".gif"

	case "webp":
		if extension != ".webp" {
			return fmt.Errorf(
				"image extension %q does not match detected format %q",
				extension,
				format,
			)
		}
		extension = ".webp"

	default:
		return fmt.Errorf("unsupported image format")
	}

	// Preserve GIF and WEBP encoded content, including animation.
	if extension == ".gif" || extension == ".webp" {
		return nil
	}

	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("rewind image: %w", err)
	}

	source, _, err := image.Decode(file)
	_ = file.Close()

	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	bounds := source.Bounds()

	if bounds.Dx() != config.Width || bounds.Dy() != config.Height {
		return fmt.Errorf("image dimensions changed during decoding")
	}

	width, height := fitWithin(bounds.Dx(), bounds.Dy(), maxImageEdge)
	fits := width == bounds.Dx() && height == bounds.Dy()

	if fits && (extension == ".png" || info.Size() <= keepAsIsBytes) {
		return nil
	}

	if !fits {
		source = downscale(source, width, height)
	}

	return replaceImageFile(path, extension, source)
}

// fitWithin returns the largest size inside limit that keeps the aspect
// ratio. It never enlarges: scaling a small photo up invents detail and
// costs bytes to store the invention.
func fitWithin(width, height, limit int) (int, int) {
	if width <= limit && height <= limit {
		return width, height
	}

	if width >= height {
		return limit, max(height*limit/width, 1)
	}

	return max(width*limit/height, 1), limit
}

// downscale resizes a picture to the given size.
//
// The resampling filter looks at every source pixel that falls inside a
// destination pixel, so hair keeps its texture instead of turning into
// noise. A phone photo can carry tens of megapixels, so the work is done by
// the draw package rather than by walking the source pixel by pixel here.
func downscale(source image.Image, width, height int) image.Image {
	bounds := source.Bounds()
	target := image.NewRGBA(image.Rect(0, 0, width, height))

	draw.CatmullRom.Scale(target, target.Bounds(), source, bounds, draw.Src, nil)

	return target
}

// replaceImageFile encodes the picture beside the original and renames it
// into place, so a failure part way through leaves the upload as it was
// rather than truncated.
func replaceImageFile(path, extension string, picture image.Image) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), "resized-*")
	if err != nil {
		return fmt.Errorf("create temporary image: %w", err)
	}

	name := temporary.Name()

	if extension == ".png" {
		err = png.Encode(temporary, picture)
	} else {
		err = jpeg.Encode(temporary, picture, &jpeg.Options{Quality: jpegQuality})
	}

	if err == nil {
		// CreateTemp opens at 0600, which would make the file unreadable
		// to anything but this process once it is renamed into a
		// directory the server hands out.
		err = temporary.Chmod(0o644)
	}

	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		os.Remove(name)

		return fmt.Errorf("write resized image: %w", err)
	}

	if err := os.Rename(name, path); err != nil {
		os.Remove(name)

		return fmt.Errorf("replace image: %w", err)
	}

	return nil
}
