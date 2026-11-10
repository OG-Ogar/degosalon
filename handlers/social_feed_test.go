package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"barber-booking/models"
	"barber-booking/storage"
)

func TestHomeFeedAuthorPhotoLinksToProfile(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)

	customer, err := storage.CreateCustomer(models.Customer{FullName: "Feed Author"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateSocialPost(context.Background(), models.SocialPost{
		AuthorRole:      models.SenderCustomer,
		AuthorID:        customer.ID,
		AuthorName:      customer.FullName,
		Text:            "A post from the feed author",
		BackgroundColor: "#fff0c2",
		TextStyle:       "normal",
	}); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	CustomerDashboardHandler(response, httptest.NewRequest(http.MethodGet, "/customer/dashboard", nil), customer, sessionData{CSRFToken: "test"})
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}

	body := response.Body.String()
	mustContain(t, "author photo link", body, `class="social-author-avatar"`)
	mustContain(t, "author photo link", body, `href="/customer/profile"`)
	mustContain(t, "author photo link", body, `class="social-author-name"`)
	mustContain(t, "author photo link", body, "A post from the feed author")
}

func TestSocialComposerOffersSoundtrackChoices(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)

	customer, err := storage.CreateCustomer(models.Customer{FullName: "Sound Author"})
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	SocialPostComposerHandler(response, httptest.NewRequest(http.MethodGet, "/customer/posts/new", nil), models.SenderCustomer, customer.FullName, customer.ProfileImage, sessionData{CSRFToken: "test"})
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}

	body := response.Body.String()
	mustContain(t, "sound picker", body, `data-sound-picker`)
	mustContain(t, "sound field", body, `name="music"`)
	mustContain(t, "bundled track", body, `value="sunrise-glow"`)
	mustContain(t, "preview audio", body, `/static/music/sunrise-glow.wav`)
}

func TestHomeFeedPlaysPostSoundtrack(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)

	customer, err := storage.CreateCustomer(models.Customer{FullName: "Music Author"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateSocialPost(context.Background(), models.SocialPost{
		AuthorRole:      models.SenderCustomer,
		AuthorID:        customer.ID,
		AuthorName:      customer.FullName,
		BackgroundColor: "#fff0c2",
		TextStyle:       "normal",
		Image:           "/uploads/posts/photo.jpg",
		Music:           "velvet-groove",
	}); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	CustomerDashboardHandler(response, httptest.NewRequest(http.MethodGet, "/customer/dashboard", nil), customer, sessionData{CSRFToken: "test"})
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}

	body := response.Body.String()
	mustContain(t, "music post card", body, `data-post-music`)
	mustContain(t, "music audio", body, `data-post-audio src="/static/music/velvet-groove.wav"`)
	mustContain(t, "music control", body, `data-post-sound-toggle`)
}
