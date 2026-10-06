package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"barber-booking/internal/testdb"
	"barber-booking/models"
	"barber-booking/storage"
)

func TestLegacySiteMediaImportPreservesAdministratorEdits(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	record := filepath.Join(t.TempDir(), "landing-image.txt")
	legacy := "https://media.example.com/legacy.png"
	if err := os.WriteFile(record, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	records := map[string]string{storage.SiteLandingImage: record, storage.SiteCustomerLandingImage: record + "-missing"}
	if err := importLegacySiteImages(context.Background(), records); err != nil {
		t.Fatal(err)
	}
	if got, err := currentLandingImage(context.Background()); err != nil || got != legacy {
		t.Fatal("legacy reference not imported")
	}
	updated := "https://media.example.com/administrator.png"
	if _, err := storage.ReplaceSiteImageContext(context.Background(), storage.SiteLandingImage, updated); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record, []byte("invalid retired record"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := importLegacySiteImages(context.Background(), records); err != nil {
		t.Fatal("retired legacy record affected durable setting")
	}
	if got, err := currentLandingImage(context.Background()); err != nil || got != updated {
		t.Fatal("legacy import overwrote updated setting")
	}
	if err := os.Remove(record); err != nil {
		t.Fatal(err)
	}
	if got, err := currentLandingImage(context.Background()); err != nil || got != updated {
		t.Fatal("normal read still depends on local text file")
	}
}

func TestSiteImageUnknownCommitRetainsUploadedObject(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	store := imageStore{dir: t.TempDir(), urlPath: "/uploads/site/"}
	previousStores := imageStores
	imageStores = []imageStore{store}
	t.Cleanup(func() { imageStores = previousStores })
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("landing_image", "committed.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(photoBytes(t, 32, 16)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/admin/landing-image", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	var committedReference string
	_, err = saveSiteImageWithWriter(r, store, "landing_image", storage.SiteLandingImage, func(_ context.Context, name, image string) (string, error) {
		if name != storage.SiteLandingImage {
			t.Error("incorrect setting name")
		}
		committedReference = image
		return "", storage.ErrSiteImageCommitUnknown
	})
	cleanupMultipart(r)
	if !errors.Is(err, errSiteImageWrite) || committedReference == "" {
		t.Fatal("commit uncertainty not returned")
	}
	files, err := os.ReadDir(store.dir)
	if err != nil || len(files) != 1 || committedReference != store.urlPath+files[0].Name() {
		t.Fatal("potentially committed image was deleted")
	}
}

func TestPostgresSiteMediaHTTPFailuresAndUploadRollback(t *testing.T) {
	databaseURL := testdb.New(t, "../supabase/schema.sql")
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("SUPABASE_URL", "")
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.CloseDatabase() })
	conn, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	old := "https://media.example.com/current.png"
	if _, err := storage.ReplaceSiteImageContext(context.Background(), storage.SiteLandingImage, old); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`create function degosalon_internal.fail_site_setting() returns trigger language plpgsql as $$ begin raise exception 'site setting write failure'; end $$;
	create trigger fail_site_setting before insert or update on degosalon_internal.site_media for each row execute function degosalon_internal.fail_site_setting()`); err != nil {
		t.Fatal(err)
	}
	previousStore, previousStores := landingImages, imageStores
	landingImages = imageStore{dir: t.TempDir(), urlPath: "/uploads/site/"}
	imageStores = []imageStore{landingImages}
	t.Cleanup(func() { landingImages = previousStore; imageStores = previousStores })
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("csrf_token", "valid")
	part, err := writer.CreateFormFile("landing_image", "setting.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(photoBytes(t, 32, 16)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/admin/landing-image", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	AdminLandingImageHandler(w, r, sessionData{CSRFToken: "valid"})
	cleanupMultipart(r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("failed setting write: %d", w.Code)
	}
	if got, err := currentLandingImage(context.Background()); err != nil || got != old {
		t.Fatal("failed upload changed current reference")
	}
	files, err := os.ReadDir(landingImages.dir)
	if err != nil || len(files) != 0 {
		t.Fatal("rolled-back uploaded file retained")
	}
	if _, err := conn.Exec("alter table degosalon_internal.site_media rename to unavailable_site_media"); err != nil {
		t.Fatal(err)
	}
	account, err := storage.CreateCustomer(models.Customer{FullName: "Settings", Email: "settings@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	for _, handler := range []func(http.ResponseWriter, *http.Request){HomeHandler,
		func(w http.ResponseWriter, r *http.Request) { AdminDashboardHandler(w, r, sessionData{}) },
		func(w http.ResponseWriter, r *http.Request) { CustomerDashboardHandler(w, r, account, sessionData{}) },
	} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("missing settings table rendered empty success: %d", w.Code)
		}
	}
}
