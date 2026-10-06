package storage

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"

	"barber-booking/internal/mediastore"
)

var ErrInvalidMediaMigration = errors.New("invalid media migration reference")
var ErrMediaMigrationCommitUnknown = errors.New("media migration commit status is unknown")

const allMediaReferencesSQL = `select profile_image as uri from public.barbers union all
 select personal_image from public.barbers union all select profile_image from public.customers union all
 select image from public.hairstyles union all select video from public.hairstyles union all
 select image from public.products union all select image_url from degosalon_internal.site_media`

// LegacyMediaReferencesContext pages distinct local references with a stable
// bytewise cursor. Malformed local references are included so migration reports
// them rather than falsely claiming that all media moved successfully.
func LegacyMediaReferencesContext(ctx context.Context, after string, limit int) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidMediaMigration
	}
	if databaseEnabled() {
		rows, err := db.QueryContext(ctx, `select distinct uri collate "C" from (`+allMediaReferencesSQL+`) as refs where uri like '/uploads/%' and uri collate "C">$1::text collate "C" order by 1 limit $2`, after, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := make([]string, 0, limit)
		for rows.Next() {
			var uri string
			if err := rows.Scan(&uri); err != nil {
				return nil, err
			}
			out = append(out, uri)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return out, nil
	}
	mu.RLock()
	defer mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	set := map[string]bool{}
	add := func(uri string) {
		if strings.HasPrefix(uri, "/uploads/") && uri > after {
			set[uri] = true
		}
	}
	for _, b := range barbers {
		add(b.ProfileImage)
		add(b.PersonalImage)
	}
	for _, c := range customers {
		add(c.ProfileImage)
	}
	for _, h := range hairstyles {
		add(h.Image)
		add(h.Video)
	}
	for _, p := range products {
		add(p.Image)
	}
	for _, image := range siteImages {
		add(image)
	}
	out := make([]string, 0, len(set))
	for uri := range set {
		out = append(out, uri)
	}
	sort.Strings(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ReplaceMediaReferencesContext atomically changes only fields still pointing
// to old. Concurrent edits survive; passwords, account state, stock, scheduling
// and metadata are never rewritten. A failed/unknown commit must not cause an
// immediate provider deletion: the pre-upload intent remains for reconciliation.
func ReplaceMediaReferencesContext(ctx context.Context, old, next string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if _, ok := mediastore.LegacyObjectPath(old); !ok {
		return 0, ErrInvalidMediaMigration
	}
	u, err := url.Parse(next)
	if err != nil || len(next) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" {
		return 0, ErrInvalidMediaMigration
	}
	if databaseEnabled() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		defer tx.Rollback()
		total := int64(0)
		for _, query := range []string{
			`update public.barbers set profile_image=case when profile_image=$1 then $2 else profile_image end,personal_image=case when personal_image=$1 then $2 else personal_image end where profile_image=$1 or personal_image=$1`,
			`update public.customers set profile_image=$2 where profile_image=$1`,
			`update public.hairstyles set image=case when image=$1 then $2 else image end,video=case when video=$1 then $2 else video end where image=$1 or video=$1`,
			`update public.products set image=$2 where image=$1`,
			`update degosalon_internal.site_media set image_url=$2 where image_url=$1`,
		} {
			result, err := tx.ExecContext(ctx, query, old, next)
			if err != nil {
				return 0, err
			}
			count, err := result.RowsAffected()
			if err != nil {
				return 0, err
			}
			total += count
		}
		// Retire the old namespace in the same commit. These permanent local
		// tombstones never enter cloud deletion; originals remain backup files.
		if _, err := tx.ExecContext(ctx, `insert into degosalon_internal.media_objects(uri,state,next_check_at) values($1,'deleted','infinity') on conflict(uri) do update set state='deleted',lease_key='',next_check_at='infinity'`, old); err != nil {
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, errors.Join(ErrMediaMigrationCommitUnknown, err)
		}
		return int(total), nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := guardMediaLocked(next); err != nil {
		return 0, err
	}
	total := 0
	for i, b := range barbers {
		changed := false
		if b.ProfileImage == old {
			barbers[i].ProfileImage = next
			changed = true
		}
		if b.PersonalImage == old {
			barbers[i].PersonalImage = next
			changed = true
		}
		if changed {
			total++
		}
	}
	for i, c := range customers {
		if c.ProfileImage == old {
			customers[i].ProfileImage = next
			total++
		}
	}
	for i, h := range hairstyles {
		changed := false
		if h.Image == old {
			hairstyles[i].Image = next
			changed = true
		}
		if h.Video == old {
			hairstyles[i].Video = next
			changed = true
		}
		if changed {
			total++
		}
	}
	for i, p := range products {
		if p.Image == old {
			products[i].Image = next
			total++
		}
	}
	for name, image := range siteImages {
		if image == old {
			siteImages[name] = next
			total++
		}
	}
	mediaObjects[old] = mediaObject{state: "deleted", next: time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)}
	return total, nil
}
