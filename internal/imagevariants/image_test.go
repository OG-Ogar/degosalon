package imagevariants

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestPreviewDimensionsAndByteBudgets(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2400, 1800))
	for y := 0; y < 1800; y++ {
		for x := 0; x < 2400; x++ {
			source.SetRGBA(x, y, color.RGBA{uint8(x / 10), uint8(y / 8), uint8((x + y) / 12), 255})
		}
	}
	var original bytes.Buffer
	if err := jpeg.Encode(&original, source, &jpeg.Options{Quality: 96}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ width, budget int }{{320, 30 << 10}, {640, 120 << 10}, {960, 240 << 10}, {1600, 500 << 10}} {
		encoded, err := JPEG(original.Bytes(), c.width)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != c.width || cfg.Height != c.width*3/4 {
			t.Fatalf("dimensions: %+v", cfg)
		}
		if len(encoded) > c.budget || len(encoded) >= original.Len() {
			t.Fatalf("width %d: %d bytes, budget %d, original %d", c.width, len(encoded), c.budget, original.Len())
		}
		t.Logf("%dw: %d bytes; original: %d bytes", c.width, len(encoded), original.Len())
	}
}
func TestPreviewRejectsUnboundedOrMalformedInput(t *testing.T) {
	for _, input := range [][]byte{nil, []byte("not an image"), make([]byte, (5<<20)+1)} {
		if _, err := JPEG(input, 320); err == nil {
			t.Fatal("accepted invalid image")
		}
	}
	var input bytes.Buffer
	_ = png.Encode(&input, image.NewRGBA(image.Rect(0, 0, 10001, 1)))
	if _, err := JPEG(input.Bytes(), 320); err == nil {
		t.Fatal("accepted oversized header")
	}
	if _, err := JPEG(input.Bytes(), 123); err == nil {
		t.Fatal("accepted unknown width")
	}
}
