package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"testing"

	"barber-booking/internal/testdb"
	"github.com/jackc/pgx/v5/stdlib"
)

func checkSiteMedia(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if value, err := SiteImageContext(ctx, SiteLandingImage); err != nil || value != "" {
		t.Fatalf("absent setting: %q %v", value, err)
	}
	initial := "https://media.example.com/original.png"
	customer := "https://media.example.com/customer.png"
	if err := ImportSiteImagesContext(ctx, map[string]string{SiteLandingImage: initial, SiteCustomerLandingImage: customer}); err != nil {
		t.Fatal(err)
	}
	root := "https://media.example.com/new.png"
	if old, err := ReplaceSiteImageContext(ctx, SiteLandingImage, root); err != nil || old != initial {
		t.Fatalf("replace: %q %v", old, err)
	}
	if err := ImportSiteImagesContext(ctx, map[string]string{SiteLandingImage: initial}); err != nil {
		t.Fatal(err)
	}
	if value, err := SiteImageContext(ctx, SiteLandingImage); err != nil || value != root {
		t.Fatal("legacy import replaced an administrator edit")
	}
	if value, err := SiteImageContext(ctx, SiteCustomerLandingImage); err != nil || value != customer {
		t.Fatal("independent setting changed")
	}
	for _, value := range []string{"", "http://media.example.com/a.png", "https://user:secret@media.example.com/a.png", "/uploads/site/../outside.png", "javascript:alert(1)"} {
		if _, err := ReplaceSiteImageContext(ctx, SiteLandingImage, value); !errors.Is(err, ErrInvalidSiteImage) {
			t.Fatalf("unsafe setting accepted: %q %v", value, err)
		}
	}
	if err := ImportSiteImagesContext(ctx, map[string]string{SiteCustomerLandingImage: initial, "unknown": initial}); !errors.Is(err, ErrInvalidSiteImage) {
		t.Fatal("invalid import accepted")
	}
	if value, err := SiteImageContext(ctx, SiteCustomerLandingImage); err != nil || value != customer {
		t.Fatal("invalid batch partially imported")
	}
	type change struct{ next, previous string }
	changes := make(chan change, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			next := fmt.Sprintf("https://media.example.com/concurrent-%d.png", i)
			old, err := ReplaceSiteImageContext(ctx, SiteLandingImage, next)
			if err != nil {
				t.Error(err)
				return
			}
			changes <- change{next, old}
		}(i)
	}
	wg.Wait()
	close(changes)
	links := map[string]string{}
	for changed := range changes {
		links[changed.next] = changed.previous
	}
	if len(links) != 12 {
		t.Fatal("concurrent write lost")
	}
	current, err := SiteImageContext(ctx, SiteLandingImage)
	if err != nil {
		t.Fatal(err)
	}
	final := current
	for i := 0; i < 12; i++ {
		previous, exists := links[current]
		if !exists {
			t.Fatal("returned old references are not a complete replacement chain")
		}
		delete(links, current)
		current = previous
	}
	if current != root || len(links) != 0 {
		t.Fatal("replacement cleanup could remove a current reference")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := ReplaceSiteImageContext(cancelled, SiteLandingImage, initial); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled replacement committed")
	}
	if _, err := SiteImageContext(cancelled, SiteLandingImage); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled read returned a setting")
	}
	if value, err := SiteImageContext(ctx, SiteLandingImage); err != nil || value != final {
		t.Fatal("cancelled operation changed a setting")
	}
}

// Use the real PostgreSQL transaction, then suppress its successful commit
// acknowledgment. No SQL behavior or results are mocked.
type siteAckLossDriver struct{ base driver.Driver }

func (d siteAckLossDriver) Open(name string) (driver.Conn, error) {
	c, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return siteAckLossConn{c}, nil
}

type siteAckLossConn struct{ driver.Conn }

func (c siteAckLossConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return siteAckLossTx{tx}, nil
}

type siteAckLossTx struct{ driver.Tx }

func (tx siteAckLossTx) Commit() error {
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	return errors.New("commit acknowledgment lost")
}
func init() {
	sql.Register("degosalon-site-commit-ack-loss", siteAckLossDriver{stdlib.GetDefaultDriver()})
}

func TestPostgresSiteMediaUnknownCommitPreservesReference(t *testing.T) {
	databaseURL := testdb.New(t, "../supabase/schema.sql")
	conn, err := sql.Open("degosalon-site-commit-ack-loss", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = conn
	t.Cleanup(func() { db = previous; conn.Close() })
	value := "https://media.example.com/committed.png"
	if _, err := ReplaceSiteImageContext(context.Background(), SiteLandingImage, value); !errors.Is(err, ErrSiteImageCommitUnknown) {
		t.Fatalf("commit uncertainty lost: %v", err)
	}
	if got, err := SiteImageContext(context.Background(), SiteLandingImage); err != nil || got != value {
		t.Fatalf("real committed reference missing: %q %v", got, err)
	}
}

func TestSiteMediaImportAndConcurrentReplacement(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	checkSiteMedia(t)
}

func TestPostgresSiteMediaPersistenceAndRollback(t *testing.T) {
	databaseURL := testdb.New(t, "../supabase/schema.sql")
	conn, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	previousDB := db
	db = conn
	t.Cleanup(func() { db = previousDB; conn.Close() })
	checkSiteMedia(t)
	value, err := SiteImageContext(context.Background(), SiteLandingImage)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	conn, err = sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	db = conn
	if got, err := SiteImageContext(context.Background(), SiteLandingImage); err != nil || got != value {
		t.Fatal("reconnect lost setting")
	}
	if _, err := conn.Exec(`create function degosalon_internal.reject_site_media() returns trigger language plpgsql as $$ begin raise exception 'site media write failed'; end $$;
	create trigger reject_site_media before insert or update on degosalon_internal.site_media for each row execute function degosalon_internal.reject_site_media()`); err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceSiteImageContext(context.Background(), SiteLandingImage, "https://media.example.com/failure.png"); err == nil {
		t.Fatal("failed replacement reported success")
	}
	if got, err := SiteImageContext(context.Background(), SiteLandingImage); err != nil || got != value {
		t.Fatal("failed write changed current image")
	}
	if _, err := conn.Exec(`drop trigger reject_site_media on degosalon_internal.site_media;
	delete from degosalon_internal.site_media;
	create or replace function degosalon_internal.reject_site_media() returns trigger language plpgsql as $$ begin if new.name='landing_image' then raise exception 'second import failed'; end if; return new; end $$;
	create trigger reject_site_media before insert on degosalon_internal.site_media for each row execute function degosalon_internal.reject_site_media()`); err != nil {
		t.Fatal(err)
	}
	if err := ImportSiteImagesContext(context.Background(), map[string]string{SiteLandingImage: value, SiteCustomerLandingImage: "https://media.example.com/import.png"}); err == nil {
		t.Fatal("failed batch import reported success")
	}
	var rows int
	if err := conn.QueryRow("select count(*) from degosalon_internal.site_media").Scan(&rows); err != nil || rows != 0 {
		t.Fatal("failed batch import committed its first setting")
	}
	conn.Close()
	if _, err := SiteImageContext(context.Background(), SiteLandingImage); err == nil {
		t.Fatal("query failure returned empty/memory setting")
	}
	if _, err := ReplaceSiteImageContext(context.Background(), SiteLandingImage, "https://media.example.com/failure.png"); err == nil {
		t.Fatal("write outage fell back to memory")
	}
}
