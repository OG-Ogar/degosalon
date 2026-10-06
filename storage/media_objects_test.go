package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"barber-booking/internal/testdb"
	"barber-booking/models"
)

func mediaURI(n int) string {
	return fmt.Sprintf("https://media.example/storage/v1/object/public/degosalon-media/site/%032x.png", n)
}
func dueMedia(t *testing.T, uri string) {
	t.Helper()
	if databaseEnabled() {
		if _, err := db.Exec(`update degosalon_internal.media_objects set next_check_at=now()-interval '1 second' where uri=$1`, uri); err != nil {
			t.Fatal(err)
		}
		return
	}
	mu.Lock()
	defer mu.Unlock()
	object := mediaObjects[uri]
	object.next = time.Now().Add(-time.Second)
	mediaObjects[uri] = object
}
func mediaState(t *testing.T, uri string) string {
	t.Helper()
	if databaseEnabled() {
		var state string
		if err := db.QueryRow(`select state from degosalon_internal.media_objects where uri=$1`, uri).Scan(&state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	mu.RLock()
	defer mu.RUnlock()
	return mediaObjects[uri].state
}

func checkMediaCleanup(t *testing.T) {
	ctx := context.Background()
	uris := make([]string, 8)
	for i := range uris {
		uris[i] = mediaURI(i + 1)
		if err := TrackMediaContext(ctx, uris[i]); err != nil {
			t.Fatal(err)
		}
	}
	seller, err := CreateBarber(models.Barber{FullName: "Seller", PhoneNumber: "1", Email: "media-seller@example.com", PasswordHash: "hash", ShopName: "Shop", ShopAddress: "Address", ProfileImage: uris[0], PersonalImage: uris[1]})
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := CreateCustomer(models.Customer{FullName: "Buyer", PhoneNumber: "2", Email: "media-buyer@example.com", PasswordHash: "hash", ProfileImage: uris[2]})
	if err != nil {
		t.Fatal(err)
	}
	style, err := CreateHairstyle(models.Hairstyle{BarberID: seller.ID, Name: "Style", Category: "Style", Duration: 30, Image: uris[3], Video: uris[4]})
	if err != nil {
		t.Fatal(err)
	}
	product, err := CreateProduct(models.Product{BarberID: seller.ID, Name: "Product", Category: "Product", Quantity: 1, Image: uris[5]})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceSiteImageContext(ctx, SiteLandingImage, uris[6]); err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceSiteImageContext(ctx, SiteCustomerLandingImage, uris[7]); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	remove := func(context.Context, string) error { calls.Add(1); return nil }
	if found, err := ProcessMediaCleanup(ctx, remove); err != nil || found {
		t.Fatalf("grace period ignored: %v %v", found, err)
	}
	for _, uri := range uris {
		dueMedia(t, uri)
	}
	for range uris {
		if found, err := ProcessMediaCleanup(ctx, remove); err != nil || !found {
			t.Fatalf("referenced reconciliation: %v %v", found, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("deleted referenced media")
	}
	for _, uri := range uris {
		if mediaState(t, uri) != "active" {
			t.Fatal("reference not retained")
		}
	}
	seller.ProfileImage = ""
	seller.PersonalImage = ""
	if err := UpdateBarberProfile(ctx, seller); err != nil {
		t.Fatal(err)
	}
	buyer.ProfileImage = ""
	if err := UpdateCustomerProfile(ctx, buyer); err != nil {
		t.Fatal(err)
	}
	style.Image = ""
	style.Video = ""
	if err := UpdateHairstyle(style); err != nil {
		t.Fatal(err)
	}
	product.Image = ""
	if err := UpdateProduct(product); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{SiteLandingImage, SiteCustomerLandingImage} {
		if _, err := ReplaceSiteImageContext(ctx, name, "https://external.example/new.png"); err != nil {
			t.Fatal(err)
		}
	}
	for _, uri := range uris {
		dueMedia(t, uri)
	}
	for range uris {
		if found, err := ProcessMediaCleanup(ctx, remove); err != nil || !found {
			t.Fatalf("unreferenced cleanup: %v %v", found, err)
		}
	}
	if calls.Load() != 8 {
		t.Fatalf("wrong number of deletes: %d", calls.Load())
	}
	// Tombstones remain immutable even if a stale writer reuses the old URL.
	buyer.ProfileImage = uris[2]
	if err := UpdateCustomerProfile(ctx, buyer); err == nil {
		t.Fatal("retired customer image resurrected")
	}
	seller.ProfileImage = uris[0]
	if err := UpdateBarberProfile(ctx, seller); err == nil {
		t.Fatal("retired barber image resurrected")
	}
	style.Image = uris[3]
	if err := UpdateHairstyle(style); err == nil {
		t.Fatal("retired style image resurrected")
	}
	product.Image = uris[5]
	if err := UpdateProduct(product); err == nil {
		t.Fatal("retired product image resurrected")
	}
	if _, err := ReplaceSiteImageContext(ctx, SiteLandingImage, uris[6]); err == nil {
		t.Fatal("retired site image resurrected")
	}
	if err := ImportSiteImagesContext(ctx, map[string]string{SiteLandingImage: uris[6]}); err != nil {
		t.Fatal("import should preserve existing setting")
	}
	for _, uri := range uris {
		if err := TrackMediaContext(ctx, uri); err != nil {
			t.Fatal(err)
		}
		if mediaState(t, uri) != "deleted" {
			t.Fatal("tracking reopened tombstone")
		}
	}
	// Repeat daily deletion reconciles a provider that completes an upload late.
	dueMedia(t, uris[0])
	if found, err := ProcessMediaCleanup(ctx, remove); err != nil || !found {
		t.Fatal("late upload not reconciled")
	}

	abandoned := mediaURI(20)
	if err := TrackMediaContext(ctx, abandoned); err != nil {
		t.Fatal(err)
	}
	dueMedia(t, abandoned)
	var winners atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ProcessMediaCleanup(ctx, func(context.Context, string) error { winners.Add(1); return nil })
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("concurrent cleanup claimed %d times", winners.Load())
	}
	failed := mediaURI(21)
	if err := TrackMediaContext(ctx, failed); err != nil {
		t.Fatal(err)
	}
	dueMedia(t, failed)
	if found, err := ProcessMediaCleanup(ctx, func(context.Context, string) error { return errors.New("provider unavailable") }); !found || err == nil {
		t.Fatal("provider failure hidden")
	}
	if mediaState(t, failed) != "deleting" {
		t.Fatal("failed deletion not retained")
	}
	if found, err := ProcessMediaCleanup(ctx, remove); err != nil || found {
		t.Fatal("retry backoff ignored")
	}
	dueMedia(t, failed)
	if found, err := ProcessMediaCleanup(ctx, remove); err != nil || !found || mediaState(t, failed) != "deleted" {
		t.Fatal("failed deletion not retried")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := TrackMediaContext(cancelled, mediaURI(22)); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled intent persisted")
	}
	if found, err := ProcessMediaCleanup(cancelled, remove); found || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled worker claimed")
	}
}

func TestMediaCleanup(t *testing.T) { Reset(); t.Cleanup(Reset); checkMediaCleanup(t) }
func TestPostgresMediaCleanup(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	old := db
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	db = connection
	db.SetMaxOpenConns(8)
	t.Cleanup(func() { db = old; connection.Close() })
	checkMediaCleanup(t)
	// A claim remains a durable tombstone after reconnect and rejects direct SQL
	// assignments, not just the Go mutation helpers.
	uri := mediaURI(30)
	if err := TrackMediaContext(context.Background(), uri); err != nil {
		t.Fatal(err)
	}
	dueMedia(t, uri)
	_, lease, found, err := claimMedia(context.Background())
	if err != nil || !found {
		t.Fatal("claim failed", err)
	}
	connection.Close()
	connection, err = sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	db = connection
	if _, err := db.Exec(`update public.customers set profile_image=$1 where email='media-buyer@example.com'`, uri); err == nil {
		t.Fatal("direct SQL resurrected claimed object")
	}
	// Expired leases are recovered and a stale acknowledgment cannot finish the
	// new claim or clear its retry state.
	dueMedia(t, uri)
	_, newLease, found, err := claimMedia(context.Background())
	if err != nil || !found || lease == newLease {
		t.Fatal("lease not recovered")
	}
	if err := finishMedia(context.Background(), uri, lease, true); err != nil {
		t.Fatal(err)
	}
	if mediaState(t, uri) != "deleting" {
		t.Fatal("stale lease finished current claim")
	}
	if err := finishMedia(context.Background(), uri, newLease, true); err != nil {
		t.Fatal(err)
	}
	if mediaState(t, uri) != "deleted" {
		t.Fatal("current lease not acknowledged")
	}
}

func TestPostgresMediaCleanupTransactionSafety(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	previous := db
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	ctx := context.Background()
	uri := mediaURI(40)
	if err := TrackMediaContext(ctx, uri); err != nil {
		t.Fatal(err)
	}
	dueMedia(t, uri)
	tx, err := connection.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`insert into degosalon_internal.site_media(name,image_url) values('landing_image',$1)`, uri); err != nil {
		t.Fatal(err)
	}
	var called bool
	remove := func(context.Context, string) error { called = true; return nil }
	// An assignment holds the intent lock until commit or rollback. The worker
	// skips it, avoiding both a stale snapshot and a delete during an open write.
	if found, err := ProcessMediaCleanup(ctx, remove); err != nil || found || called {
		t.Fatal("uncommitted assignment was claimed", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if found, err := ProcessMediaCleanup(ctx, remove); err != nil || !found || called {
		t.Fatal("committed reference was deleted", err)
	}

	next := mediaURI(41)
	if err := TrackMediaContext(ctx, next); err != nil {
		t.Fatal(err)
	}
	dueMedia(t, next)
	ackLoss, err := sql.Open("degosalon-site-commit-ack-loss", address)
	if err != nil {
		t.Fatal(err)
	}
	defer ackLoss.Close()
	db = ackLoss
	_, err = ReplaceSiteImageContext(ctx, SiteLandingImage, next)
	db = connection
	if !errors.Is(err, ErrSiteImageCommitUnknown) {
		t.Fatal("expected uncertain real commit", err)
	}
	if found, err := ProcessMediaCleanup(ctx, remove); err != nil || !found || called {
		t.Fatal("uncertain but committed reference was deleted", err)
	}

	orphan := mediaURI(42)
	if err := TrackMediaContext(ctx, orphan); err != nil {
		t.Fatal(err)
	}
	dueMedia(t, orphan)
	tx, err = connection.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`insert into degosalon_internal.site_media(name,image_url) values('customer_landing_image',$1)`, orphan); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if found, err := ProcessMediaCleanup(ctx, remove); err != nil || !found || !called {
		t.Fatal("rolled-back object not cleaned", err)
	}
	connection.Close()
	if err := TrackMediaContext(ctx, mediaURI(43)); err == nil {
		t.Fatal("SQL outage fell back to memory")
	}
	if found, err := ProcessMediaCleanup(ctx, remove); found || err == nil {
		t.Fatal("SQL outage reported empty queue")
	}
}
