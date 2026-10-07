// Package imagevariants creates bounded, cropped JPEG previews. Original media
// remain the source of truth, including full-size viewing and archival.
package imagevariants

import (
	"bytes"
	"errors"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
)

var ErrImage = errors.New("image cannot be previewed")

func ValidWidth(width int) bool { return width == 320 || width == 640 || width == 960 || width == 1600 }

// JPEG emits a fixed 4:3 thumbnail; its width matches the srcset descriptor.
// Bound headers before decoding to keep compressed-image allocations predictable.
func JPEG(source []byte, width int) ([]byte, error) {
	if !ValidWidth(width) || len(source) > 5<<20 {
		return nil, ErrImage
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(source))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 10000 || cfg.Height > 10000 || int64(cfg.Width)*int64(cfg.Height) > 16000000 {
		return nil, ErrImage
	}
	photo, _, err := image.Decode(bytes.NewReader(source))
	if err != nil {
		return nil, ErrImage
	}
	bounds := photo.Bounds()
	crop := bounds
	if bounds.Dx()*3 > bounds.Dy()*4 {
		cropWidth := bounds.Dy() * 4 / 3
		crop.Min.X += (bounds.Dx() - cropWidth) / 2
		crop.Max.X = crop.Min.X + cropWidth
	} else {
		cropHeight := bounds.Dx() * 3 / 4
		if cropHeight < 1 {
			cropHeight = 1
		}
		crop.Min.Y += (bounds.Dy() - cropHeight) / 2
		crop.Max.Y = crop.Min.Y + cropHeight
	}
	if crop.Dx() < 1 {
		return nil, ErrImage
	}
	preview := image.NewRGBA(image.Rect(0, 0, width, width*3/4))
	// White background preserves transparent logos without black JPEG artifacts.
	draw.Draw(preview, preview.Bounds(), image.NewUniform(image.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(preview, preview.Bounds(), photo, crop, draw.Over, nil)
	var output bytes.Buffer
	if jpeg.Encode(&output, preview, &jpeg.Options{Quality: 78}) != nil {
		return nil, ErrImage
	}
	return output.Bytes(), nil
}
