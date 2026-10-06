package handlers

import (
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

const (
	// maxImageEdge bounds the longest side of a stored photo. The widest
	// any page draws one is a little under 900 pixels, so a picture
	// straight off a phone carries several megabytes of detail that the
	// browser downloads and then throws away.
	maxImageEdge      = 1600
	maxImageDimension = 10000
	maxImagePixels    = 16_000_000

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
	if config.Width < 1 || config.Height < 1 || config.Width > maxImageDimension || config.Height > maxImageDimension || config.Width > maxImagePixels/config.Height {
		return fmt.Errorf("image exceeds the limit of 16 megapixels or 10000 pixels per side")
	}
	expected := map[string]string{".jpg": "jpeg", ".jpeg": "jpeg", ".png": "png", ".gif": "gif", ".webp": "webp"}[extension]
	if expected == "" || format != expected {
		return fmt.Errorf("image content does not match its filename type")
	}
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

// downscale averages every source pixel that falls inside a destination
// pixel. Picking one pixel per destination instead is shorter, but at
// these ratios it turns hair into noise.
func downscale(source image.Image, width, height int) image.Image {
	bounds := source.Bounds()
	target := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		top := bounds.Min.Y + y*bounds.Dy()/height
		bottom := bounds.Min.Y + (y+1)*bounds.Dy()/height

		if bottom <= top {
			bottom = top + 1
		}

		for x := 0; x < width; x++ {
			left := bounds.Min.X + x*bounds.Dx()/width
			right := bounds.Min.X + (x+1)*bounds.Dx()/width

			if right <= left {
				right = left + 1
			}

			target.SetRGBA(x, y, averageColor(source, left, top, right, bottom))
		}
	}

	return target
}

// averageColor is the mean of one box of source pixels. RGBA reports
// alpha-premultiplied values, which is what makes averaging them safe: a
// transparent pixel contributes nothing rather than contributing whatever
// colour happens to sit behind its transparency.
func averageColor(source image.Image, left, top, right, bottom int) color.RGBA {
	var red, green, blue, alpha uint64

	count := uint64((right - left) * (bottom - top))

	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			r, g, b, a := source.At(x, y).RGBA()

			red += uint64(r)
			green += uint64(g)
			blue += uint64(b)
			alpha += uint64(a)
		}
	}

	// RGBA values run to 65535, so each average is shifted back down into
	// the single byte per channel that color.RGBA holds.
	return color.RGBA{
		R: uint8((red / count) >> 8),
		G: uint8((green / count) >> 8),
		B: uint8((blue / count) >> 8),
		A: uint8((alpha / count) >> 8),
	}
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
