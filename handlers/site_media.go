package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"barber-booking/storage"
)

var errSiteImageWrite = errors.New("site image setting write failed")

func currentLandingImage(ctx context.Context) (string, error) {
	return storage.SiteImageContext(ctx, storage.SiteLandingImage)
}
func currentCustomerLandingImage(ctx context.Context) (string, error) {
	return storage.SiteImageContext(ctx, storage.SiteCustomerLandingImage)
}
func saveLandingImage(r *http.Request) (string, error) {
	return saveSiteImage(r, landingImages, "landing_image", storage.SiteLandingImage)
}
func saveCustomerLandingImage(r *http.Request) (string, error) {
	return saveSiteImage(r, customerLandingImages, "customer_landing_image", storage.SiteCustomerLandingImage)
}

func saveSiteImage(r *http.Request, store imageStore, field, name string) (string, error) {
	return saveSiteImageWithWriter(r, store, field, name, storage.ReplaceSiteImageContext)
}
func saveSiteImageWithWriter(r *http.Request, store imageStore, field, name string, write func(context.Context, string, string) (string, error)) (string, error) {
	image, err := store.save(r, field)
	if err != nil {
		return "", err
	}
	previous, err := write(r.Context(), name, image)
	if err != nil {
		// A failed commit acknowledgment may still have committed the reference.
		// Retain that object until its state can be reconciled; never break it.
		if !errors.Is(err, storage.ErrSiteImageCommitUnknown) {
			removeImageIgnoringError(image)
		}
		return "", fmt.Errorf("%w: %w", errSiteImageWrite, err)
	}
	if previous != "" && previous != image {
		removeImageIgnoringError(previous)
	}
	return image, nil
}

// ImportLegacySiteImages reads old text pointers only when the durable setting
// is absent. Normal requests never depend on those files after import.
func ImportLegacySiteImages(ctx context.Context) error {
	return importLegacySiteImages(ctx, map[string]string{storage.SiteLandingImage: landingImageRecord, storage.SiteCustomerLandingImage: customerLandingImageRecord})
}
func importLegacySiteImages(ctx context.Context, records map[string]string) error {
	values := map[string]string{}
	for name, record := range records {
		current, err := storage.SiteImageContext(ctx, name)
		if err != nil {
			return err
		}
		if current != "" {
			continue
		}
		file, err := os.Open(record)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read legacy site setting: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(file, 2049))
		_ = file.Close()
		if err != nil {
			return fmt.Errorf("read legacy site setting: %w", err)
		}
		value := strings.TrimSpace(string(data))
		if value != "" {
			values[name] = value
		}
	}
	return storage.ImportSiteImagesContext(ctx, values)
}
