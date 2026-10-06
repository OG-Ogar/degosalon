package storage

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

const SiteLandingImage = "landing_image"
const SiteCustomerLandingImage = "customer_landing_image"

var siteImages = map[string]string{}
var ErrInvalidSiteImage = errors.New("invalid site image setting")
var ErrSiteImageCommitUnknown = errors.New("site image commit status is unknown")

func validSiteImage(name, value string) bool {
	if name != SiteLandingImage && name != SiteCustomerLandingImage {
		return false
	}
	if value == "" || len(value) > 2048 {
		return false
	}
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.ForceQuery {
		return false
	}
	if u.Scheme == "https" && u.Hostname() != "" {
		return true
	}
	if u.Scheme != "" || u.Host != "" || !strings.HasPrefix(u.Path, "/uploads/site/") || u.RawPath != "" {
		return false
	}
	namePart := strings.TrimPrefix(u.Path, "/uploads/site/")
	return namePart != "" && path.Base(namePart) == namePart && !strings.ContainsAny(namePart, "/\\") && namePart != "." && namePart != ".."
}

func SiteImageContext(ctx context.Context, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if name != SiteLandingImage && name != SiteCustomerLandingImage {
		return "", ErrInvalidSiteImage
	}
	if databaseEnabled() {
		var value string
		err := db.QueryRowContext(ctx, "select image_url from degosalon_internal.site_media where name=$1", name).Scan(&value)
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		if err == nil && !validSiteImage(name, value) {
			return "", ErrInvalidSiteImage
		}
		return value, err
	}
	mu.RLock()
	defer mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return siteImages[name], nil
}

// ReplaceSiteImageContext returns the exact unreferenced image after committing.
// The owner lock also serializes the first insertion and legacy imports.
func ReplaceSiteImageContext(ctx context.Context, name, value string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !validSiteImage(name, value) {
		return "", ErrInvalidSiteImage
	}
	if databaseEnabled() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return "", err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", "site-media:"+name); err != nil {
			return "", err
		}
		var previous string
		err = tx.QueryRowContext(ctx, "select image_url from degosalon_internal.site_media where name=$1 for update", name).Scan(&previous)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, "insert into degosalon_internal.site_media(name,image_url) values($1,$2) on conflict(name) do update set image_url=excluded.image_url", name, value); err != nil {
			return "", err
		}
		if err := tx.Commit(); err != nil {
			return "", errors.Join(ErrSiteImageCommitUnknown, err)
		}
		return previous, nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := guardMediaLocked(value); err != nil {
		return "", err
	}
	previous := siteImages[name]
	siteImages[name] = value
	return previous, nil
}

// ImportSiteImagesContext preserves existing settings; it never overwrites a
// concurrent administrator edit. Validate every record before changing any.
func ImportSiteImagesContext(ctx context.Context, values map[string]string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	names := make([]string, 0, len(values))
	for name, value := range values {
		if !validSiteImage(name, value) {
			return ErrInvalidSiteImage
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if databaseEnabled() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		for _, name := range names {
			if _, err := tx.ExecContext(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", "site-media:"+name); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "insert into degosalon_internal.site_media(name,image_url) select $1,$2 where not exists (select 1 from degosalon_internal.site_media where name=$1) on conflict(name) do nothing", name, values[name]); err != nil {
				return err
			}
		}
		return tx.Commit()
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, name := range names {
		if _, exists := siteImages[name]; !exists {
			if err := guardMediaLocked(values[name]); err != nil {
				return err
			}
		}
	}
	for _, name := range names {
		if _, exists := siteImages[name]; !exists {
			siteImages[name] = values[name]
		}
	}
	return nil
}
