package handlers

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// gradient builds a picture with a different colour in every direction,
// so a resize has something to average and cannot pass by producing one
// flat block.
func gradient(width, height int) image.Image {
	picture := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			picture.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / width),
				G: uint8(y * 255 / height),
				B: 90,
				A: 255,
			})
		}
	}

	return picture
}

// writePNG saves a picture into the test's own directory and returns the
// path, so nothing has to be cleaned up by hand.
func writePNG(t *testing.T, name string, picture image.Image) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}

	if err := png.Encode(file, picture); err != nil {
		file.Close()
		t.Fatalf("encode %s: %v", name, err)
	}

	if err := file.Close(); err != nil {
		t.Fatalf("close %s: %v", name, err)
	}

	return path
}

// storedSize reports the pixel size of an image on disk.
func storedSize(t *testing.T, path string) (int, int) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}

	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}

	return config.Width, config.Height
}

// TestFitWithinNeverEnlarges is the property that keeps a small photo
// from being blown up into a bigger file with no more detail in it.
func TestFitWithinNeverEnlarges(t *testing.T) {
	cases := []struct {
		width, height int
		wantW, wantH  int
	}{
		{800, 600, 800, 600},     // already inside the limit
		{1600, 1600, 1600, 1600}, // exactly on it
		{4000, 3000, 1600, 1200}, // landscape
		{3000, 4000, 1200, 1600}, // portrait
		{5000, 5, 1600, 1},       // a strip, rounded up to one pixel
	}

	for _, c := range cases {
		width, height := fitWithin(c.width, c.height, maxImageEdge)

		if width != c.wantW || height != c.wantH {
			t.Errorf("fitWithin(%d, %d) = (%d, %d), want (%d, %d)",
				c.width, c.height, width, height, c.wantW, c.wantH)
		}
	}
}

// TestOversizedImageIsShrunk covers the upload this was written for: a
// picture off a phone, several times wider than any page shows it.
func TestOversizedImageIsShrunk(t *testing.T) {
	path := writePNG(t, "big.png", gradient(2400, 1800))

	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	if err := normalizeStoredImage(path); err != nil {
		t.Fatalf("normalizeStoredImage: %v", err)
	}

	width, height := storedSize(t, path)

	if width != 1600 || height != 1200 {
		t.Fatalf("stored at %dx%d, want 1600x1200", width, height)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}

	if after.Size() >= before.Size() {
		t.Fatalf("file grew from %d to %d bytes", before.Size(), after.Size())
	}
}

// TestSmallImageIsLeftAlone checks the other half of the rule. Rewriting
// a photo that already fits would cost quality for nothing.
func TestSmallImageIsLeftAlone(t *testing.T) {
	path := writePNG(t, "small.png", gradient(400, 300))

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	if err := normalizeStoredImage(path); err != nil {
		t.Fatalf("normalizeStoredImage: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}

	if !bytes.Equal(before, after) {
		t.Fatal("the file was rewritten even though it already fits")
	}
}

// TestAnimatedGifSurvivesUpload is why GIFs are skipped: re-encoding one
// through image.Decode would keep the first frame and throw the rest of
// the animation away.
func TestAnimatedGifSurvivesUpload(t *testing.T) {
	frames := &gif.GIF{
		Image: []*image.Paletted{
			image.NewPaletted(image.Rect(0, 0, 2400, 1800), palette()),
			image.NewPaletted(image.Rect(0, 0, 2400, 1800), palette()),
		},
		Delay: []int{10, 10},
	}

	path := filepath.Join(t.TempDir(), "moving.gif")

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gif: %v", err)
	}

	if err := gif.EncodeAll(file, frames); err != nil {
		file.Close()
		t.Fatalf("encode gif: %v", err)
	}

	if err := file.Close(); err != nil {
		t.Fatalf("close gif: %v", err)
	}

	if err := normalizeStoredImage(path); err != nil {
		t.Fatalf("normalizeStoredImage: %v", err)
	}

	reopened, err := os.Open(path)
	if err != nil {
		t.Fatalf("open gif: %v", err)
	}

	defer reopened.Close()

	decoded, err := gif.DecodeAll(reopened)
	if err != nil {
		t.Fatalf("decode gif: %v", err)
	}

	if len(decoded.Image) != 2 {
		t.Fatalf("the stored gif has %d frames, want 2", len(decoded.Image))
	}
}

// palette is the two colours the test animation is drawn from.
func palette() color.Palette {
	return color.Palette{color.White, color.Black}
}
