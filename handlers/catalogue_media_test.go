package handlers

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"barber-booking/storage"
)

// clipBytes builds a small ISO media file, the shape a phone records when
// it films a service.
func clipBytes(t *testing.T) []byte {
	t.Helper()

	ftyp := []byte{
		0, 0, 0, 0x18,
		'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
		0, 0, 2, 0,
		'i', 's', 'o', 'm',
		'i', 's', 'o', '2',
	}

	mdat := append([]byte{0, 0, 0, 0x48}, make([]byte, 64)...)

	return append(ftyp, mdat...)
}

// paddedPhoto returns a picture that decodes normally but is larger than
// the old five megabyte cap. The padding is an ancillary chunk, which a
// reader skips, so the test pays for the bytes without paying for pixels.
func paddedPhoto(t *testing.T, padding int) []byte {
	t.Helper()

	base := photoBytes(t, 64, 64)

	// The trailer is a length, a type and a checksum. The padding goes
	// in front of the length, so the file still ends with a whole chunk.
	end := bytes.LastIndex(base, []byte("IEND"))

	if end < 4 {
		t.Fatal("the encoded picture has no IEND chunk")
	}

	chunk := make([]byte, 0, padding+12)
	chunk = binary.BigEndian.AppendUint32(chunk, uint32(padding))
	chunk = append(chunk, 'p', 'a', 'D', 'd')
	chunk = append(chunk, make([]byte, padding)...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))

	padded := make([]byte, 0, len(base)+len(chunk))
	padded = append(padded, base[:end-4]...)
	padded = append(padded, chunk...)
	padded = append(padded, base[end-4:]...)

	return padded
}

// storedClipPath turns a public clip path back into the file on disk.
func storedClipPath(t *testing.T, webPath string) string {
	t.Helper()

	for _, store := range videoStores {
		if strings.HasPrefix(webPath, store.urlPath) {
			return store.dir + "/" + strings.TrimPrefix(webPath, store.urlPath)
		}
	}

	t.Fatalf("%q is not under any clip directory", webPath)

	return ""
}

// TestBarberUploadsProductPhotoAndClip covers the request a barber makes
// when they list a product: a photo of any size, and a clip customers can
// watch on the product page.
func TestBarberUploadsProductPhotoAndClip(t *testing.T) {
	server := newServer(t)

	// Uploads are written relative to the working directory, which is
	// this package while the tests run.
	t.Cleanup(func() { os.RemoveAll("uploads") })

	barber := newBrowser(t)

	// Resizing is pixel work, and the race detector makes pixel work slow
	// enough to trip the default client timeout.
	barber.Timeout = time.Minute

	id := registerBarber(t, barber, server.URL, "bob@example.com")

	form := get(t, barber, server.URL+"/barber/products/new")

	status, body := uploadForm(t, barber, server.URL+"/barber/products/new",
		map[string]string{
			"csrf_token":  csrfToken(t, "product form", form),
			"name":        "Beard butter",
			"category":    "Care",
			"description": "A rich balm for beards.",
			"price":       "2500",
			"quantity":    "12",
			"available":   "on",
		},
		photoFile{"image", "product.png", paddedPhoto(t, 6<<20)},
		photoFile{"video", "clip.mp4", clipBytes(t)})

	if status != http.StatusOK {
		t.Fatalf("listing a product with a photo and a clip: status %d\n%s", status, body)
	}

	products, err := storage.ListProductsByBarber(context.Background(), id)
	if err != nil {
		t.Fatalf("read the products: %v", err)
	}

	if len(products) != 1 {
		t.Fatalf("the product was not stored: %+v", products)
	}

	product := products[0]

	if product.Image == "" {
		t.Fatal("the product photo was not stored")
	}

	if product.Video == "" {
		t.Fatal("the product clip was not stored")
	}

	if !strings.HasPrefix(product.Video, productVideos.urlPath) {
		t.Fatalf("the product clip was stored outside its own directory: %q", product.Video)
	}

	if _, err := os.Stat(storedClipPath(t, product.Video)); err != nil {
		t.Fatalf("the product clip is not on disk: %v", err)
	}

	// The customer sees both the photo and the clip on the product card.
	customer := newBrowser(t)
	registerCustomer(t, customer, server.URL, "ada@example.com", "correct horse")

	catalogue := get(t, customer, server.URL+"/customer/products")

	mustContain(t, "the catalogue", catalogue, product.Image)
	mustContain(t, "the catalogue", catalogue, product.Video)
	mustContain(t, "the catalogue", catalogue, `class="catalogue-video"`)

	// The clip is served with a type a browser will play.
	request := httptest.NewRequest(http.MethodGet, product.Video, nil)
	recorder := httptest.NewRecorder()

	CatalogueVideoHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("the clip was not served: status %d", recorder.Code)
	}

	if mediaType := recorder.Header().Get("Content-Type"); mediaType != "video/mp4" {
		t.Fatalf("the clip was served as %q, want video/mp4", mediaType)
	}
}

// TestBarberKeepsTheProductClipWhenTheFormHasNoNewOne covers the edit form:
// saving without choosing a clip keeps the stored one, and choosing a new
// one replaces the file.
func TestBarberKeepsTheProductClipWhenTheFormHasNoNewOne(t *testing.T) {
	server := newServer(t)
	t.Cleanup(func() { os.RemoveAll("uploads") })

	barber := newBrowser(t)
	barber.Timeout = time.Minute

	id := registerBarber(t, barber, server.URL, "bob@example.com")

	form := get(t, barber, server.URL+"/barber/products/new")

	status, _ := uploadForm(t, barber, server.URL+"/barber/products/new",
		map[string]string{
			"csrf_token":  csrfToken(t, "product form", form),
			"name":        "Beard butter",
			"category":    "Care",
			"description": "A rich balm for beards.",
			"price":       "2500",
			"quantity":    "12",
			"available":   "on",
		},
		photoFile{"video", "clip.mp4", clipBytes(t)})

	if status != http.StatusOK {
		t.Fatalf("listing a product: status %d", status)
	}

	products, err := storage.ListProductsByBarber(context.Background(), id)
	if err != nil {
		t.Fatalf("read the products: %v", err)
	}

	if len(products) != 1 || products[0].Video == "" {
		t.Fatalf("the product clip was not stored: %+v", products)
	}

	kept := products[0].Video

	// An edit that chooses no clip keeps the stored one.
	edit := get(t, barber, server.URL+"/barber/products/"+strconv.Itoa(products[0].ID)+"/edit")

	mustContain(t, "the product form", edit, "Current video")

	status, body := uploadForm(t, barber,
		server.URL+"/barber/products/"+strconv.Itoa(products[0].ID)+"/update",
		map[string]string{
			"csrf_token":  csrfToken(t, "product form", edit),
			"name":        "Beard butter",
			"category":    "Care",
			"description": "A rich balm for beards.",
			"price":       "2600",
			"quantity":    "9",
			"available":   "on",
		})

	if status != http.StatusOK {
		t.Fatalf("editing a product without a clip: status %d\n%s", status, body)
	}

	products, err = storage.ListProductsByBarber(context.Background(), id)
	if err != nil {
		t.Fatalf("read the products: %v", err)
	}

	if products[0].Video != kept {
		t.Fatalf("an edit with no clip changed it: %q", products[0].Video)
	}

	// An edit that chooses a new clip replaces the file and drops the old.
	edit = get(t, barber, server.URL+"/barber/products/"+strconv.Itoa(products[0].ID)+"/edit")

	status, body = uploadForm(t, barber,
		server.URL+"/barber/products/"+strconv.Itoa(products[0].ID)+"/update",
		map[string]string{
			"csrf_token":  csrfToken(t, "product form", edit),
			"name":        "Beard butter",
			"category":    "Care",
			"description": "A rich balm for beards.",
			"price":       "2600",
			"quantity":    "9",
			"available":   "on",
		},
		photoFile{"video", "clip.mp4", clipBytes(t)})

	if status != http.StatusOK {
		t.Fatalf("editing a product with a clip: status %d\n%s", status, body)
	}

	products, err = storage.ListProductsByBarber(context.Background(), id)
	if err != nil {
		t.Fatalf("read the products: %v", err)
	}

	if products[0].Video == kept {
		t.Fatal("the new clip was not stored")
	}

	if _, err := os.Stat(storedClipPath(t, products[0].Video)); err != nil {
		t.Fatalf("the new clip is not on disk: %v", err)
	}

	if _, err := os.Stat(storedClipPath(t, kept)); !os.IsNotExist(err) {
		t.Fatalf("the replaced clip was left on disk: %v", err)
	}
}

// TestHairstyleClipReachesTheCustomerPage covers the hairstyle side: the
// barber's clip and photo are shown as a grid of cards the customer can
// pick from.
func TestHairstyleClipReachesTheCustomerPage(t *testing.T) {
	server := newServer(t)
	t.Cleanup(func() { os.RemoveAll("uploads") })

	barber := newBrowser(t)
	barber.Timeout = time.Minute

	id := registerBarber(t, barber, server.URL, "bob@example.com")

	form := get(t, barber, server.URL+"/barber/hairstyles/new")

	status, body := uploadForm(t, barber, server.URL+"/barber/hairstyles/new",
		map[string]string{
			"csrf_token":  csrfToken(t, "hairstyle form", form),
			"name":        "Low fade",
			"category":    "Fade",
			"description": "Tapered at the sides.",
			"price":       "2500",
			"duration":    "45",
			"available":   "on",
		},
		photoFile{"image", "fade.png", paddedPhoto(t, 6<<20)},
		photoFile{"video", "clip.mp4", clipBytes(t)})

	if status != http.StatusOK {
		t.Fatalf("listing a style with a photo and a clip: status %d\n%s", status, body)
	}

	styles := storage.ListAvailableHairstylesByBarber(id)

	if len(styles) != 1 || styles[0].Image == "" || styles[0].Video == "" {
		t.Fatalf("the style media was not stored: %+v", styles)
	}

	customer := newBrowser(t)
	registerCustomer(t, customer, server.URL, "ada@example.com", "correct horse")

	page := get(t, customer, server.URL+"/customer/barbers/"+strconv.Itoa(id))

	mustContain(t, "the barber's page", page, styles[0].Image)
	mustContain(t, "the barber's page", page, styles[0].Video)
	mustContain(t, "the barber's page", page, `class="hairstyle-grid"`)
	mustContain(t, "the barber's page", page, `class="catalogue-video"`)

	// The grid holds every style the barber offers, not only the one the
	// booking form happens to start on.
	if count := strings.Count(page, `name="hairstyle_id"`); count != len(styles) {
		t.Fatalf("the page offers %d styles for booking, want %d", count, len(styles))
	}
}
