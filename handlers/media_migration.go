package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"barber-booking/internal/mediastore"
	"barber-booking/storage"
)

var ErrLegacyMediaMigration = errors.New("legacy media migration incomplete; check available files and provider/database configuration")

type MediaMigrationReport struct {
	Copied, UpdatedRecords, Unreferenced, Failed int
	Remaining                                    bool
}
type legacyMediaBackend interface {
	PublicURL(string) (string, error)
	Upload(context.Context, string, string, int64, io.Reader) error
}

// MigrateLegacyMedia copies available referenced files without deleting originals.
// Run against the persistent database after importing legacy site pointers.
func MigrateLegacyMedia(ctx context.Context, root string, store *mediastore.Store) (MediaMigrationReport, error) {
	if store == nil {
		return MediaMigrationReport{}, mediastore.ErrConfiguration
	}
	return migrateLegacyMedia(ctx, root, store)
}
func migrateLegacyMedia(ctx context.Context, root string, store legacyMediaBackend) (report MediaMigrationReport, err error) {
	if err := ctx.Err(); err != nil {
		return report, err
	}
	initial, err := storage.LegacyMediaReferencesContext(ctx, "", 1)
	if err != nil {
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
		return report, ErrLegacyMediaMigration
	}
	if len(initial) == 0 {
		return report, nil
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return report, ErrLegacyMediaMigration
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return report, ErrLegacyMediaMigration
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return report, ErrLegacyMediaMigration
	}
	cursor := ""
	for {
		refs, err := storage.LegacyMediaReferencesContext(ctx, cursor, 50)
		if err != nil {
			if ctx.Err() != nil {
				return report, ctx.Err()
			}
			return report, ErrLegacyMediaMigration
		}
		if len(refs) == 0 {
			break
		}
		for _, uri := range refs {
			if ctx.Err() != nil {
				return report, ctx.Err()
			}
			records, err := migrateLegacyFile(ctx, resolved, uri, store)
			if err != nil {
				if ctx.Err() != nil {
					return report, ctx.Err()
				}
				report.Failed++
			} else {
				report.Copied++
				report.UpdatedRecords += records
				if records == 0 {
					report.Unreferenced++
				}
			}
			cursor = uri
		}
	}
	remaining, readErr := storage.LegacyMediaReferencesContext(ctx, "", 1)
	report.Remaining = len(remaining) > 0
	if readErr != nil || report.Remaining {
		return report, ErrLegacyMediaMigration
	}
	if report.Failed > 0 {
		return report, ErrLegacyMediaMigration
	}
	return report, nil
}

func migrateLegacyFile(ctx context.Context, root, uri string, store legacyMediaBackend) (int, error) {
	objectPath, ok := mediastore.LegacyObjectPath(uri)
	if !ok {
		return 0, ErrLegacyMediaMigration
	}
	path := filepath.Join(root, filepath.FromSlash(objectPath))
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return 0, ErrLegacyMediaMigration
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !strings.HasPrefix(resolved, root+string(os.PathSeparator)) {
		return 0, ErrLegacyMediaMigration
	}
	source, err := os.Open(resolved)
	if err != nil {
		return 0, ErrLegacyMediaMigration
	}
	defer source.Close()
	opened, err := source.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return 0, ErrLegacyMediaMigration
	}
	video := strings.HasPrefix(objectPath, "hairstyle-videos/")
	limit := int64(maxUploadBytes)
	if video {
		limit = maxVideoBytes
	}
	if opened.Size() < 1 || opened.Size() > limit {
		return 0, ErrLegacyMediaMigration
	}
	// Copy and normalize a private staging file, never the legacy original.
	stageDir, err := os.MkdirTemp("", "degosalon-legacy-media-")
	if err != nil {
		return 0, ErrLegacyMediaMigration
	}
	defer os.RemoveAll(stageDir)
	extension := filepath.Ext(objectPath)
	staged := filepath.Join(stageDir, "media"+extension)
	destination, err := os.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return 0, ErrLegacyMediaMigration
	}
	copied, copyErr := io.Copy(destination, io.LimitReader(source, limit+1))
	closeErr := destination.Close()
	if copyErr != nil || closeErr != nil || copied != opened.Size() || copied > limit {
		return 0, ErrLegacyMediaMigration
	}
	if !video {
		if err := normalizeStoredImage(staged); err != nil {
			return 0, ErrLegacyMediaMigration
		}
	}
	file, err := os.Open(staged)
	if err != nil {
		return 0, ErrLegacyMediaMigration
	}
	defer file.Close()
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	kind := http.DetectContentType(head[:n])
	if video && !strings.HasPrefix(kind, "video/") || !video && !strings.HasPrefix(kind, "image/") {
		return 0, ErrLegacyMediaMigration
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, ErrLegacyMediaMigration
	}
	stagedInfo, err := file.Stat()
	if err != nil {
		return 0, ErrLegacyMediaMigration
	}
	name, err := mediaName(extension)
	if err != nil {
		return 0, err
	}
	nextPath := objectPath[:strings.Index(objectPath, "/")+1] + name
	next, err := store.PublicURL(nextPath)
	if err != nil {
		return 0, err
	}
	if err := storage.TrackMediaContext(ctx, next); err != nil {
		return 0, err
	}
	if err := store.Upload(ctx, nextPath, kind, stagedInfo.Size(), file); err != nil {
		return 0, err
	}
	return storage.ReplaceMediaReferencesContext(ctx, uri, next)
}
