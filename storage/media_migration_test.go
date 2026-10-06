package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"barber-booking/internal/testdb"
	"barber-booking/models"
)

func checkMediaMigration(t *testing.T) {
	ctx := context.Background()
	old := "/uploads/site/legacy.png"
	next := mediaURI(60)
	if err := TrackMediaContext(ctx, next); err != nil {
		t.Fatal(err)
	}
	seller, err := CreateBarber(models.Barber{FullName: "Seller", PhoneNumber: "1", Email: "migration-seller@example.com", PasswordHash: "original-hash", ShopName: "Shop", ShopAddress: "Address", ProfileImage: old, PersonalImage: old})
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := CreateCustomer(models.Customer{FullName: "Buyer", PhoneNumber: "2", Email: "migration-buyer@example.com", PasswordHash: "buyer-hash", ProfileImage: old})
	if err != nil {
		t.Fatal(err)
	}
	style, err := CreateHairstyle(models.Hairstyle{BarberID: seller.ID, Name: "Style", Category: "Style", Duration: 30, Price: 10, Image: old, Video: old})
	if err != nil {
		t.Fatal(err)
	}
	product, err := CreateProduct(models.Product{BarberID: seller.ID, Name: "Product", Category: "Product", Quantity: 7, Price: 20, Image: old})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceSiteImageContext(ctx, SiteLandingImage, old); err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceSiteImageContext(ctx, SiteCustomerLandingImage, old); err != nil {
		t.Fatal(err)
	}
	refs, err := LegacyMediaReferencesContext(ctx, "", 1)
	if err != nil || len(refs) != 1 || refs[0] != old {
		t.Fatalf("distinct page: %v %v", refs, err)
	}
	if refs, err := LegacyMediaReferencesContext(ctx, old, 1); err != nil || len(refs) != 0 {
		t.Fatal("cursor repeated reference")
	}
	// Changes made after enumeration must survive conditional migration.
	buyer.ProfileImage = "https://external.example/new.png"
	if err := UpdateCustomerProfile(ctx, buyer); err != nil {
		t.Fatal(err)
	}
	count, err := ReplaceMediaReferencesContext(ctx, old, next)
	if err != nil || count != 5 {
		t.Fatalf("conditional replacement: %d %v", count, err)
	}
	gotSeller, err := FindBarberByIDContext(ctx, seller.ID)
	if err != nil || gotSeller.ProfileImage != next || gotSeller.PersonalImage != next || gotSeller.PasswordHash != "original-hash" {
		t.Fatal("seller metadata changed")
	}
	gotBuyer, err := FindCustomerByIDResult(ctx, buyer.ID)
	if err != nil || gotBuyer.ProfileImage != buyer.ProfileImage || gotBuyer.PasswordHash != "buyer-hash" {
		t.Fatal("concurrent buyer edit overwritten")
	}
	gotStyle, err := FindHairstyleByIDResult(ctx, style.ID)
	if err != nil || gotStyle.Image != next || gotStyle.Video != next || gotStyle.Price != 10 || gotStyle.Duration != 30 {
		t.Fatal("service metadata changed")
	}
	gotProduct, err := FindProductByIDResult(ctx, product.ID)
	if err != nil || gotProduct.Image != next || gotProduct.Quantity != 7 || gotProduct.Price != 20 {
		t.Fatal("stock or price changed")
	}
	gotBuyer.ProfileImage = old
	if err := UpdateCustomerProfile(ctx, gotBuyer); err == nil {
		t.Fatal("stale writer restored retired local URL")
	}
	if count, err := ReplaceMediaReferencesContext(ctx, old, next); err != nil || count != 0 {
		t.Fatal("migration replay rewrote records")
	}
	if refs, err := LegacyMediaReferencesContext(ctx, "", 50); err != nil || len(refs) != 0 {
		t.Fatal("migrated references still local")
	}
	for _, old := range []string{"/uploads/site/../secret.png", "/uploads/site/x.png?secret=key", "https://external.example/x.png"} {
		if _, err := ReplaceMediaReferencesContext(ctx, old, next); !errors.Is(err, ErrInvalidMediaMigration) {
			t.Fatal("invalid source accepted")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := LegacyMediaReferencesContext(cancelled, "", 50); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled enumeration succeeded")
	}
	if _, err := ReplaceMediaReferencesContext(cancelled, old, next); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled replacement succeeded")
	}
}
func TestMediaMigration(t *testing.T) { Reset(); t.Cleanup(Reset); checkMediaMigration(t) }
func TestPostgresMediaMigration(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	previous := db
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	checkMediaMigration(t)
	ctx := context.Background()
	old := "/uploads/site/rollback.png"
	next := mediaURI(61)
	if err := TrackMediaContext(ctx, next); err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceSiteImageContext(ctx, SiteLandingImage, old); err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Exec(`update public.barbers set profile_image=$1`, old); err != nil {
		t.Fatal(err)
	}
	// Fault after the barber write verifies full rollback across reference tables.
	if _, err := connection.Exec(`create function degosalon_internal.reject_migrated_site() returns trigger language plpgsql as $$ begin raise exception 'migration fault';end $$;create trigger reject_migrated_site before update on degosalon_internal.site_media for each row execute function degosalon_internal.reject_migrated_site()`); err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceMediaReferencesContext(ctx, old, next); err == nil {
		t.Fatal("failed batch reported success")
	}
	var retained string
	if err := connection.QueryRow(`select profile_image from public.barbers limit 1`).Scan(&retained); err != nil || retained != old {
		t.Fatal("failed batch partially committed")
	}
	if value, err := SiteImageContext(ctx, SiteLandingImage); err != nil || value != old {
		t.Fatal("failed migration changed site")
	}
	if _, err := connection.Exec(`drop trigger reject_migrated_site on degosalon_internal.site_media`); err != nil {
		t.Fatal(err)
	}
	ackLoss, err := sql.Open("degosalon-site-commit-ack-loss", address)
	if err != nil {
		t.Fatal(err)
	}
	defer ackLoss.Close()
	db = ackLoss
	_, err = ReplaceMediaReferencesContext(ctx, old, next)
	db = connection
	if !errors.Is(err, ErrMediaMigrationCommitUnknown) {
		t.Fatal("uncertain real commit not identified", err)
	}
	dueMedia(t, next)
	deleted := false
	if found, err := ProcessMediaCleanup(ctx, func(context.Context, string) error { deleted = true; return nil }); err != nil || !found || deleted {
		t.Fatal("committed migration object was deleted", err)
	}
	connection.Close()
	connection, err = sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	db = connection
	if value, err := SiteImageContext(ctx, SiteLandingImage); err != nil || value != next {
		t.Fatal("reconnect lost migration")
	}
	connection.Close()
	if _, err := LegacyMediaReferencesContext(ctx, "", 50); err == nil {
		t.Fatal("enumeration outage fell back to memory")
	}
	if _, err := ReplaceMediaReferencesContext(ctx, old, next); err == nil {
		t.Fatal("replacement outage fell back to memory")
	}
}

func checkLegacyMediaPagination(t *testing.T) {
	ctx := context.Background()
	for i := 0; i < 53; i++ {
		if _, err := CreateCustomer(models.Customer{FullName: "Paged", PhoneNumber: "1", PasswordHash: "hash", Email: fmt.Sprintf("paged-%d@example.com", i), ProfileImage: fmt.Sprintf("/uploads/customers/paged-%02d.png", i)}); err != nil {
			t.Fatal(err)
		}
	}
	cursor := ""
	total := 0
	for {
		page, err := LegacyMediaReferencesContext(ctx, cursor, 7)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) > 7 {
			t.Fatal("page exceeds limit")
		}
		if len(page) == 0 {
			break
		}
		for _, uri := range page {
			if uri <= cursor {
				t.Fatal("cursor repeated or reordered reference")
			}
			cursor = uri
			total++
		}
	}
	if total != 53 {
		t.Fatalf("pagination omitted records: %d", total)
	}
	for _, limit := range []int{0, 101} {
		if _, err := LegacyMediaReferencesContext(ctx, "", limit); !errors.Is(err, ErrInvalidMediaMigration) {
			t.Fatal("invalid page limit accepted")
		}
	}
}
func TestLegacyMediaPagination(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	checkLegacyMediaPagination(t)
}
func TestPostgresLegacyMediaPagination(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	previous := db
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	checkLegacyMediaPagination(t)
}
