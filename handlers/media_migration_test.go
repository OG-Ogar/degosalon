package handlers

import (
	"barber-booking/internal/testdb"
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"os"
	"path/filepath"
	"testing"

	"barber-booking/models"
	"barber-booking/storage"
)

type fakeLegacyCloud struct {
	files    map[string][]byte
	fail     bool
	onUpload func()
}

func (cloud *fakeLegacyCloud) PublicURL(path string) (string, error) {
	return "https://media.example/storage/v1/object/public/degosalon-media/" + path, nil
}
func (cloud *fakeLegacyCloud) Upload(ctx context.Context, path, kind string, size int64, body io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if cloud.fail {
		return errors.New("private-provider-details")
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if int64(len(data)) != size {
		return errors.New("size mismatch")
	}
	cloud.files[path] = data
	if cloud.onUpload != nil {
		cloud.onUpload()
	}
	return nil
}
func TestLegacyFileMigrationAndReplay(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "site"), 0755); err != nil {
		t.Fatal(err)
	}
	data := photoBytes(t, 30, 30)
	original := filepath.Join(root, "site", "legacy.png")
	if err := os.WriteFile(original, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ReplaceSiteImageContext(context.Background(), storage.SiteLandingImage, "/uploads/site/legacy.png"); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ReplaceSiteImageContext(context.Background(), storage.SiteCustomerLandingImage, "/uploads/site/legacy.png"); err != nil {
		t.Fatal(err)
	}
	cloud := &fakeLegacyCloud{files: map[string][]byte{}}
	report, err := migrateLegacyMedia(context.Background(), root, cloud)
	if err != nil || report.Copied != 1 || report.UpdatedRecords != 2 || report.Failed != 0 || len(cloud.files) != 1 {
		t.Fatalf("migration: %+v %v", report, err)
	}
	for path, copied := range cloud.files {
		if !bytes.Equal(copied, data) {
			t.Fatal("small PNG changed")
		}
		uri, _ := cloud.PublicURL(path)
		for _, name := range []string{storage.SiteLandingImage, storage.SiteCustomerLandingImage} {
			if value, err := storage.SiteImageContext(context.Background(), name); err != nil || value != uri {
				t.Fatal("shared reference not moved")
			}
		}
	}
	if kept, err := os.ReadFile(original); err != nil || !bytes.Equal(kept, data) {
		t.Fatal("original changed or deleted")
	}
	report, err = migrateLegacyMedia(context.Background(), root, cloud)
	if err != nil || report.Copied != 0 || len(cloud.files) != 1 {
		t.Fatal("replay uploaded again")
	}
}

func TestLegacyMigrationPreservesConcurrentEditsAndFailures(t *testing.T) {
	for _, mode := range []string{"provider failure", "concurrent edit", "missing file", "invalid image", "symlink", "outside directory"} {
		t.Run(mode, func(t *testing.T) {
			storage.Reset()
			t.Cleanup(storage.Reset)
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "site"), 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "site", "legacy.png")
			data := photoBytes(t, 30, 30)
			if mode == "invalid image" {
				data = []byte("not an image")
			}
			if mode != "missing file" {
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "symlink" || mode == "outside directory" {
				outside := t.TempDir()
				if err := os.WriteFile(filepath.Join(outside, "legacy.png"), data, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if mode == "symlink" {
					if err := os.Symlink(filepath.Join(outside, "legacy.png"), path); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Remove(filepath.Dir(path)); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(outside, filepath.Dir(path)); err != nil {
						t.Fatal(err)
					}
				}
			}
			old := "/uploads/site/legacy.png"
			if _, err := storage.ReplaceSiteImageContext(context.Background(), storage.SiteLandingImage, old); err != nil {
				t.Fatal(err)
			}
			cloud := &fakeLegacyCloud{files: map[string][]byte{}, fail: mode == "provider failure"}
			if mode == "concurrent edit" {
				cloud.onUpload = func() {
					if _, err := storage.ReplaceSiteImageContext(context.Background(), storage.SiteLandingImage, "https://external.example/new.png"); err != nil {
						t.Fatal(err)
					}
				}
			}
			report, err := migrateLegacyMedia(context.Background(), root, cloud)
			value, readErr := storage.SiteImageContext(context.Background(), storage.SiteLandingImage)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if mode == "concurrent edit" {
				if err != nil || report.Unreferenced != 1 || report.UpdatedRecords != 0 || value != "https://external.example/new.png" {
					t.Fatal("concurrent edit overwritten")
				}
			} else {
				if !errors.Is(err, ErrLegacyMediaMigration) || report.Failed != 1 || value != old {
					t.Fatalf("failure changed reference: %+v %v", report, err)
				}
			}
		})
	}
}

func TestLegacyMigrationContinuesPastMissingFiles(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "customers"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateCustomer(models.Customer{Email: "missing@example.com", ProfileImage: "/uploads/customers/a-missing.png"}); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.CreateCustomer(models.Customer{Email: "valid@example.com", ProfileImage: "/uploads/customers/z-valid.png"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "customers", "z-valid.png"), photoBytes(t, 30, 30), 0600); err != nil {
		t.Fatal(err)
	}
	cloud := &fakeLegacyCloud{files: map[string][]byte{}}
	report, err := migrateLegacyMedia(context.Background(), root, cloud)
	if !errors.Is(err, ErrLegacyMediaMigration) || report.Copied != 1 || report.Failed != 1 || len(cloud.files) != 1 {
		t.Fatalf("partial migration: %+v %v", report, err)
	}
}

func TestLegacyMigrationEmptyAndCancelled(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	cloud := &fakeLegacyCloud{files: map[string][]byte{}}
	if report, err := migrateLegacyMedia(context.Background(), filepath.Join(t.TempDir(), "absent"), cloud); err != nil || report.Copied != 0 {
		t.Fatal("empty migration required source files")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := migrateLegacyMedia(ctx, t.TempDir(), cloud); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled migration succeeded")
	}
}

func TestLegacyVideoMigrationAndByteLimits(t *testing.T) {
	for _, tooLarge := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid video", true: "oversized video"}[tooLarge], func(t *testing.T) {
			storage.Reset()
			t.Cleanup(storage.Reset)
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "hairstyle-videos"), 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "hairstyle-videos", "legacy.mp4")
			data := []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if tooLarge {
				if err := os.Truncate(path, maxVideoBytes+1); err != nil {
					t.Fatal(err)
				}
			}
			seller, err := storage.CreateBarber(models.Barber{Email: "video@example.com"})
			if err != nil {
				t.Fatal(err)
			}
			style, err := storage.CreateHairstyle(models.Hairstyle{BarberID: seller.ID, Video: "/uploads/hairstyle-videos/legacy.mp4"})
			if err != nil {
				t.Fatal(err)
			}
			cloud := &fakeLegacyCloud{files: map[string][]byte{}}
			stageRoot := t.TempDir()
			t.Setenv("TMPDIR", stageRoot)
			report, err := migrateLegacyMedia(context.Background(), root, cloud)
			if tooLarge {
				if !errors.Is(err, ErrLegacyMediaMigration) || report.Failed != 1 || len(cloud.files) != 0 {
					t.Fatal("oversized video uploaded")
				}
			} else {
				if err != nil || report.Copied != 1 {
					t.Fatalf("video migration: %+v %v", report, err)
				}
				for _, copied := range cloud.files {
					if !bytes.Equal(copied, data) {
						t.Fatal("video bytes changed")
					}
				}
				migrated, err := storage.FindHairstyleByIDResult(context.Background(), style.ID)
				if err != nil || migrated.Video == style.Video {
					t.Fatal("video reference did not move")
				}
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal("original video removed")
			}
			entries, err := os.ReadDir(stageRoot)
			if err != nil || len(entries) != 0 {
				t.Fatal("staging files not cleaned")
			}
		})
	}
}

func TestLegacyNormalizationPreservesOriginal(t *testing.T) {
	storage.Reset()
	t.Cleanup(storage.Reset)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "site"), 0755); err != nil {
		t.Fatal(err)
	}
	data := photoBytes(t, 1900, 30)
	path := filepath.Join(root, "site", "large.png")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ReplaceSiteImageContext(context.Background(), storage.SiteLandingImage, "/uploads/site/large.png"); err != nil {
		t.Fatal(err)
	}
	cloud := &fakeLegacyCloud{files: map[string][]byte{}}
	if _, err := migrateLegacyMedia(context.Background(), root, cloud); err != nil {
		t.Fatal(err)
	}
	for _, uploaded := range cloud.files {
		config, _, err := image.DecodeConfig(bytes.NewReader(uploaded))
		if err != nil || config.Width != maxImageEdge {
			t.Fatal("cloud image was not normalized")
		}
	}
	retained, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(retained, data) {
		t.Fatal("normalization changed original")
	}
}

func TestPostgresLegacyFileMigration(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	t.Setenv("DATABASE_URL", address)
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.CloseDatabase() })
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "site"), 0755); err != nil {
		t.Fatal(err)
	}
	original := photoBytes(t, 30, 30)
	path := filepath.Join(root, "site", "original.png")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	pointer := filepath.Join(root, "site", "landing-image.txt")
	if err := os.WriteFile(pointer, []byte("/uploads/site/original.png\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ImportLegacySiteImagesFromRoot(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	cloud := &fakeLegacyCloud{files: map[string][]byte{}}
	report, err := migrateLegacyMedia(context.Background(), root, cloud)
	if err != nil || report.Copied != 1 || report.UpdatedRecords != 1 || report.Remaining {
		t.Fatalf("persistent migration: %+v %v", report, err)
	}
	before, err := currentLandingImage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.CloseDatabase(); err != nil {
		t.Fatal(err)
	}
	if err := storage.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	if err := ImportLegacySiteImagesFromRoot(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	after, err := currentLandingImage(context.Background())
	if err != nil || after != before {
		t.Fatal("restart/import restored original pointer")
	}
	if _, err := storage.ReplaceSiteImageContext(context.Background(), storage.SiteLandingImage, "/uploads/site/original.png"); err == nil {
		t.Fatal("stale write restored migrated local reference")
	}
	if report, err := migrateLegacyMedia(context.Background(), root, cloud); err != nil || report.Copied != 0 || len(cloud.files) != 1 {
		t.Fatal("persistent replay uploaded twice")
	}
	kept, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(kept, original) {
		t.Fatal("original was not retained")
	}
	keptPointer, err := os.ReadFile(pointer)
	if err != nil || string(keptPointer) != "/uploads/site/original.png\n" {
		t.Fatal("original pointer backup changed")
	}
}
