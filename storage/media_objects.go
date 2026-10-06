package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

var ErrRetiredMedia = errors.New("media object is retired")

const mediaGrace = time.Hour
const mediaLease = time.Minute

type mediaObject struct {
	state, lease string
	next         time.Time
	attempts     int
}

var mediaObjects = map[string]mediaObject{}

// TrackMediaContext must commit before bytes are sent to the provider. Repeated
// cleanup requests retain a tombstone and never re-open a retired object.
func TrackMediaContext(ctx context.Context, uri string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if uri == "" || len(uri) > 2048 {
		return ErrRetiredMedia
	}
	if databaseEnabled() {
		_, err := db.ExecContext(ctx, `insert into degosalon_internal.media_objects(uri) values($1) on conflict(uri) do update set next_check_at=case when media_objects.state in ('pending','active') then least(media_objects.next_check_at, now()+interval '1 hour') else media_objects.next_check_at end`, uri)
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	object, exists := mediaObjects[uri]
	if !exists {
		mediaObjects[uri] = mediaObject{state: "pending", next: time.Now().Add(mediaGrace)}
	} else if (object.state == "pending" || object.state == "active") && object.next.After(time.Now().Add(mediaGrace)) {
		object.next = time.Now().Add(mediaGrace)
		mediaObjects[uri] = object
	}
	return nil
}

func guardMediaLocked(uris ...string) error {
	for _, uri := range uris {
		if object, ok := mediaObjects[uri]; ok && (object.state == "deleting" || object.state == "deleted") {
			return ErrRetiredMedia
		}
	}
	return nil
}
func mediaReferencedLocked(uri string) bool {
	for _, b := range barbers {
		if b.ProfileImage == uri || b.PersonalImage == uri {
			return true
		}
	}
	for _, c := range customers {
		if c.ProfileImage == uri {
			return true
		}
	}
	for _, h := range hairstyles {
		if h.Image == uri || h.Video == uri {
			return true
		}
	}
	for _, p := range products {
		if p.Image == uri {
			return true
		}
	}
	for _, image := range siteImages {
		if image == uri {
			return true
		}
	}
	return false
}

const mediaReferenceSQL = `select exists(
 select 1 from public.barbers where profile_image=$1 or personal_image=$1 union all
 select 1 from public.customers where profile_image=$1 union all
 select 1 from public.hairstyles where image=$1 or video=$1 union all
 select 1 from public.products where image=$1 union all
 select 1 from degosalon_internal.site_media where image_url=$1
)`

func claimMedia(ctx context.Context) (uri, lease string, found bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = ctx.Err(); err != nil {
		return
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return
	}
	lease = hex.EncodeToString(random[:])
	if databaseEnabled() {
		var tx *sql.Tx
		tx, err = db.BeginTx(ctx, nil)
		if err != nil {
			return
		}
		defer tx.Rollback()
		var referenced bool
		err = tx.QueryRowContext(ctx, `select uri from degosalon_internal.media_objects where next_check_at<=now() order by next_check_at,uri for update skip locked limit 1`).Scan(&uri)
		if errors.Is(err, sql.ErrNoRows) {
			err = nil
			return
		}
		if err != nil {
			return
		}
		// A separate statement after acquiring the intent lock takes a fresh READ
		// COMMITTED snapshot, including assignments that committed while we waited.
		err = tx.QueryRowContext(ctx, mediaReferenceSQL, uri).Scan(&referenced)
		if err != nil {
			return
		}
		if referenced {
			_, err = tx.ExecContext(ctx, `update degosalon_internal.media_objects set state='active',lease_key='',attempts=0,next_check_at=now()+interval '1 day' where uri=$1`, uri)
		} else {
			_, err = tx.ExecContext(ctx, `update degosalon_internal.media_objects set state='deleting',lease_key=$2,attempts=least(attempts+1,30),next_check_at=now()+interval '1 minute' where uri=$1`, uri, lease)
		}
		if err != nil {
			return
		}
		err = tx.Commit()
		if err != nil {
			return
		}
		found = true
		if referenced {
			uri = ""
			lease = ""
		}
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if err = ctx.Err(); err != nil {
		return
	}
	now := time.Now()
	var candidate mediaObject
	for key, object := range mediaObjects {
		if !object.next.After(now) && (uri == "" || object.next.Before(candidate.next) || object.next.Equal(candidate.next) && key < uri) {
			uri = key
			candidate = object
		}
	}
	if uri == "" {
		return
	}
	found = true
	if mediaReferencedLocked(uri) {
		candidate = mediaObject{state: "active", next: now.Add(24 * time.Hour)}
		mediaObjects[uri] = candidate
		uri = ""
		lease = ""
		return
	}
	candidate.state = "deleting"
	candidate.lease = lease
	candidate.next = now.Add(mediaLease)
	if candidate.attempts < 30 {
		candidate.attempts++
	}
	mediaObjects[uri] = candidate
	return
}

func finishMedia(ctx context.Context, uri, lease string, success bool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if databaseEnabled() {
		_, err := db.ExecContext(ctx, `update degosalon_internal.media_objects set state=case when $3 then 'deleted' else 'deleting' end,lease_key='',next_check_at=now()+case when $3 then interval '1 day' else make_interval(secs=>least(86400,power(2,least(attempts,10))*60)::int) end where uri=$1 and state='deleting' and lease_key=$2`, uri, lease, success)
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	object, exists := mediaObjects[uri]
	if !exists || object.state != "deleting" || object.lease != lease {
		return nil
	}
	delay := 24 * time.Hour
	if success {
		object.state = "deleted"
	} else {
		shift := object.attempts
		if shift > 10 {
			shift = 10
		}
		delay = time.Duration(1<<shift) * time.Minute
		if delay > 24*time.Hour {
			delay = 24 * time.Hour
		}
	}
	object.lease = ""
	object.next = time.Now().Add(delay)
	mediaObjects[uri] = object
	return nil
}

// ProcessMediaCleanup claims one due intent. A claim freezes new references
// before remote deletion; successful deletes keep a daily reconciliation
// tombstone, covering late uploads after uncertain provider acknowledgments.
func ProcessMediaCleanup(ctx context.Context, remove func(context.Context, string) error) (bool, error) {
	uri, lease, found, err := claimMedia(ctx)
	if err != nil || !found || uri == "" {
		return found, err
	}
	err = remove(ctx, uri)
	finishErr := finishMedia(context.Background(), uri, lease, err == nil)
	return true, errors.Join(err, finishErr)
}
