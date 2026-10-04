package handlers

import (
	"bytes"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"barber-booking/storage"
)

// photoBytes encodes a picture the way a browser would send one.
func photoBytes(t *testing.T, width, height int) []byte {
	t.Helper()

	var buffer bytes.Buffer

	if err := png.Encode(&buffer, gradient(width, height)); err != nil {
		t.Fatalf("encode photo: %v", err)
	}

	return buffer.Bytes()
}

// photoFile is one file attached to a multipart form.
type photoFile struct {
	field    string
	filename string
	content  []byte
}

// uploadForm submits a multipart form with any number of files attached,
// which is what the photo forms post. It returns the status and body so a
// rejected upload can be checked as well as an accepted one.
func uploadForm(
	t *testing.T,
	browser *http.Client,
	target string,
	fields map[string]string,
	files ...photoFile,
) (int, string) {
	t.Helper()

	var body bytes.Buffer

	form := multipart.NewWriter(&body)

	for name, value := range fields {
		if err := form.WriteField(name, value); err != nil {
			t.Fatalf("write field %s: %v", name, err)
		}
	}

	for _, file := range files {
		part, err := form.CreateFormFile(file.field, file.filename)
		if err != nil {
			t.Fatalf("create file part: %v", err)
		}

		if _, err := part.Write(file.content); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}

	if err := form.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}

	response, err := browser.Post(target, form.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}

	defer response.Body.Close()

	page, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("POST %s: read body: %v", target, err)
	}

	return response.StatusCode, string(page)
}

// registerBarber creates a shop through the real form, signs in, and
// returns the shop's ID. Barber registration lands on the login page
// rather than opening a session, so the sign-in is part of the setup.
func registerBarber(
	t *testing.T,
	browser *http.Client,
	base string,
	email string,
) int {
	t.Helper()

	const password = "clippers and all"

	status, page := uploadForm(t, browser, base+"/barber/register", map[string]string{
		"fullname":         "Bob Barber",
		"phone":            "0700000000",
		"email":            email,
		"password":         password,
		"confirm_password": password,
		"shopname":         "Bob's Chair",
		"shopaddress":      "12 High Street",
		"state":            "Lagos",
		"local_area":       "Ikeja",
	}, photoFile{"personal_image", "bob.png", photoBytes(t, 64, 64)})
	if status != http.StatusOK {
		t.Fatalf("register barber: status %d\n%s", status, page)
	}

	login := post(t, browser, base+"/barber/login", url.Values{
		"email":    {email},
		"password": {password},
	})
	mustContain(t, "barber login", login, "Dashboard")

	barber, found := storage.FindBarberByEmail(email)
	if !found {
		t.Fatalf("registering %s stored no barber", email)
	}

	return barber.ID
}

// storedPath turns a public image path back into the file on disk, which
// is where the test checks what was actually kept.
func storedPath(t *testing.T, webPath string) string {
	t.Helper()

	for _, store := range imageStores {
		if strings.HasPrefix(webPath, store.urlPath) {
			return store.dir + "/" + strings.TrimPrefix(webPath, store.urlPath)
		}
	}

	t.Fatalf("%q is not under any upload directory", webPath)

	return ""
}

// TestCustomerSeesTheBarbersPhotos is the whole point of the feature: a
// photo a barber uploads has to reach the page a customer opens from
// their name, both the shop picture and the one on each style.
func TestCustomerSeesTheBarbersPhotos(t *testing.T) {
	server := newServer(t)

	// Uploads are written relative to the working directory, which is
	// this package while the tests run.
	t.Cleanup(func() { os.RemoveAll("uploads") })

	barber := newBrowser(t)

	// Resizing is pixel work, and the race detector makes pixel work slow
	// enough to trip the default client timeout.
	barber.Timeout = time.Minute

	id := registerBarber(t, barber, server.URL, "bob@example.com")

	profileForm := get(t, barber, server.URL+"/barber/profile/edit")

	// Both test photos are over the edge limit in one direction only.
	// That is all it takes to exercise the resize, and it keeps the test
	// spending its time on the flow rather than on averaging pixels.
	//
	// The shop photo and the barber's own picture go up in one submission,
	// the way the form offers them.
	status, _ := uploadForm(t, barber, server.URL+"/barber/profile/edit",
		map[string]string{
			"csrf_token":  csrfToken(t, "profile form", profileForm),
			"fullname":    "Bob Barber",
			"phone":       "0700000000",
			"email":       "bob@example.com",
			"shopname":    "Bob's Chair",
			"shopaddress": "12 High Street",
		},
		photoFile{"profile_image", "shop.png", photoBytes(t, 1900, 300)},
		photoFile{"personal_image", "bob.png", photoBytes(t, 300, 1900)})

	if status != http.StatusOK {
		t.Fatalf("uploading the shop photo: status %d, want 200", status)
	}

	styleForm := get(t, barber, server.URL+"/barber/hairstyles/new")

	status, _ = uploadForm(t, barber, server.URL+"/barber/hairstyles/new",
		map[string]string{
			"csrf_token":  csrfToken(t, "hairstyle form", styleForm),
			"name":        "Low fade",
			"category":    "Fade",
			"description": "Tapered at the sides.",
			"price":       "2500",
			"duration":    "45",
			"available":   "on",
		},
		photoFile{"image", "fade.png", photoBytes(t, 400, 2000)})

	if status != http.StatusOK {
		t.Fatalf("uploading the style photo: status %d, want 200", status)
	}

	shop, found := storage.FindBarberByEmail("bob@example.com")
	if !found || shop.ProfileImage == "" {
		t.Fatalf("the shop photo was not stored on the barber: %+v", shop.ProfileImage)
	}

	if shop.PersonalImage == "" {
		t.Fatal("the barber's own picture was not stored")
	}

	if shop.PersonalImage == shop.ProfileImage {
		t.Fatalf("both photos were stored as the same file: %s", shop.ProfileImage)
	}

	styles := storage.ListAvailableHairstylesByBarber(id)
	if len(styles) != 1 || styles[0].Image == "" {
		t.Fatalf("the style photo was not stored: %+v", styles)
	}

	// Every file must have been shrunk on the way in, or the customer
	// downloads a phone camera photo to look at a 64 pixel square.
	stored := []string{shop.ProfileImage, shop.PersonalImage, styles[0].Image}

	for _, image := range stored {
		width, height := storedSize(t, storedPath(t, image))

		if width > maxImageEdge || height > maxImageEdge {
			t.Errorf("%s was stored at %dx%d, over the %d limit",
				image, width, height, maxImageEdge)
		}
	}

	customer := newBrowser(t)
	registerCustomer(t, customer, server.URL, "ada@example.com", "correct horse")

	page := get(t, customer, server.URL+"/customer/barbers/"+strconv.Itoa(id))

	for _, image := range stored {
		mustContain(t, "the barber's page", page, image)
	}

	// The shop photo is bounded rather than stretched, the barber's own
	// picture sits beside their name, and the style photo is the small
	// square beside its radio button.
	mustContain(t, "the barber's page", page, `class="photo"`)
	mustContain(t, "the barber's page", page, `class="avatar"`)
	mustContain(t, "the barber's page", page, `class="pick"`)

	// The list customers arrive from carries the photo too.
	mustContain(t, "the barber list",
		get(t, customer, server.URL+"/customer/barbers"), shop.ProfileImage)

	// A second edit that fills in neither file input has to keep both
	// photos rather than clearing the profile.
	kept := get(t, barber, server.URL+"/barber/profile/edit")

	status, _ = uploadForm(t, barber, server.URL+"/barber/profile/edit",
		map[string]string{
			"csrf_token":  csrfToken(t, "profile form", kept),
			"fullname":    "Bob Barber",
			"phone":       "0700000000",
			"email":       "bob@example.com",
			"shopname":    "Bob's Barbers",
			"shopaddress": "12 High Street",
		})

	if status != http.StatusOK {
		t.Fatalf("saving the profile without photos: status %d, want 200", status)
	}

	again, _ := storage.FindBarberByEmail("bob@example.com")

	if again.ProfileImage != shop.ProfileImage ||
		again.PersonalImage != shop.PersonalImage {

		t.Errorf("an edit with empty file inputs changed the photos: %+v", again)
	}
}
