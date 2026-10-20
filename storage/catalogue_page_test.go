package storage

import (
	"barber-booking/internal/testdb"
	"barber-booking/models"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func checkCataloguePages(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	seller, err := CreateBarber(models.Barber{FullName: "Public seller", ShopName: "Public shop", Email: "catalogue@example.test", PhoneNumber: "private phone", PasswordHash: "private hash", State: "Lagos", LocalArea: "Ikeja", Status: models.BarberActive})
	if err != nil {
		t.Fatal(err)
	}
	hidden, err := CreateBarber(models.Barber{FullName: "Hidden seller", ShopName: "Hidden", Email: "hidden-catalogue@example.test", Status: models.BarberSuspended})
	if err != nil {
		t.Fatal(err)
	}
	var products []models.Product
	create := func(owner int, available bool) models.Product {
		p, err := CreateProduct(models.Product{BarberID: owner, Name: fmt.Sprintf("Product %d", len(products)), Category: "Care", Description: "Details", Price: 1250.25, Quantity: 2, IsAvailable: available})
		if err != nil {
			t.Fatal(err)
		}
		products = append(products, p)
		return p
	}
	for i := 0; i < 26; i++ {
		create(seller.ID, true)
	}
	create(hidden.ID, true)
	create(seller.ID, false)
	archived := create(seller.ID, true)
	if _, err := DeleteProductContext(ctx, archived.ID, seller.ID); err != nil {
		t.Fatal(err)
	}
	first, err := CataloguePage(ctx, 0, 0, PageCursor{})
	if err != nil || len(first.Items) != 24 || first.HasPrevious || !first.HasNext {
		t.Fatalf("first catalogue page: %+v %v", first, err)
	}
	for _, row := range first.Items {
		if row.Barber.ID != seller.ID || row.Barber.FullName != "Public seller" || row.Barber.PasswordHash != "" || row.Barber.PhoneNumber != "" || row.Barber.Email != "" {
			t.Fatalf("private or ineligible seller returned: %+v", row.Barber)
		}
	}
	next, err := CataloguePage(ctx, seller.ID, 0, PageCursor{After: first.Items[23].Product.ID})
	if err != nil || len(next.Items) != 2 || !next.HasPrevious || next.HasNext {
		t.Fatalf("next page: %+v %v", next, err)
	}
	back, err := CataloguePage(ctx, seller.ID, 0, PageCursor{Before: next.Items[0].Product.ID})
	if err != nil || len(back.Items) != 24 || back.HasPrevious || !back.HasNext {
		t.Fatalf("back page: %+v %v", back, err)
	}
	for i, row := range back.Items {
		if row.Product.ID != first.Items[i].Product.ID {
			t.Fatal("backward page lost boundary")
		}
	}
	exact, err := CataloguePage(ctx, seller.ID, products[25].ID, PageCursor{})
	if err != nil || len(exact.Items) != 1 || exact.Items[0].Product.ID != products[25].ID {
		t.Fatalf("exact continuation: %+v %v", exact, err)
	}
	mismatch, err := CataloguePage(ctx, hidden.ID, products[25].ID, PageCursor{})
	if err != nil || len(mismatch.Items) != 0 {
		t.Fatal("cross-seller product returned")
	}
	added := create(seller.ID, true)
	if _, err := DeleteProductContext(ctx, first.Items[0].Product.ID, seller.ID); err != nil {
		t.Fatal(err)
	}
	changed, err := CataloguePage(ctx, seller.ID, 0, PageCursor{After: first.Items[23].Product.ID})
	if err != nil || len(changed.Items) != 3 || changed.Items[2].Product.ID != added.ID {
		t.Fatalf("insert/archive shifted cursor: %+v %v", changed, err)
	}
	if _, err := CataloguePage(ctx, 0, 0, PageCursor{After: 1, Before: 2}); !errors.Is(err, ErrInvalidPage) {
		t.Fatal("conflicting cursors accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := CataloguePage(canceled, 0, 0, PageCursor{}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}
}
func TestCataloguePages(t *testing.T) { Reset(); t.Cleanup(Reset); checkCataloguePages(t) }
func TestPostgresCataloguePages(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	checkCataloguePages(t)
}

type catalogueConnector struct{ state *discoveryDriverState }

func (c catalogueConnector) Driver() driver.Driver { return discoveryDriver{} }
func (c catalogueConnector) Connect(context.Context) (driver.Conn, error) {
	return &catalogueConn{&discoveryConn{c.state}}, nil
}

type catalogueConn struct{ *discoveryConn }

func (c *catalogueConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.queries++
	if c.state.cardError != nil {
		return nil, c.state.cardError
	}
	if !strings.Contains(query, "join public.barbers") || !strings.Contains(query, "p.barber_id=$1") || !strings.Contains(query, "p.id=$2") || !strings.Contains(query, "p.id>$3") || !strings.Contains(query, "limit $4") || len(args) != 4 || args[3].Value != int64(25) {
		return nil, errors.New("catalogue not scoped and bounded")
	}
	for _, private := range []string{"password_hash", "phone_number", "email"} {
		if strings.Contains(query, private) {
			return nil, errors.New("private fields selected")
		}
	}
	values := []driver.Value{int64(10), int64(1), "Product", "Care", "Details", float64(1250), int64(2), "", "", true, false, int64(1), "Seller", "Shop"}
	if c.state.badScan {
		values[0] = "invalid id"
	}
	return &discoveryRows{values: values, err: c.state.rowError}, nil
}
func TestCatalogueBoundedQueryAndFailures(t *testing.T) {
	outage := errors.New("catalogue outage")
	for name, state := range map[string]*discoveryDriverState{"bounded join": {}, "query failure": {cardError: outage}, "row failure": {rowError: outage}, "scan failure": {badScan: true}} {
		t.Run(name, func(t *testing.T) {
			connection := sql.OpenDB(catalogueConnector{state})
			previous := db
			db = connection
			t.Cleanup(func() { db = previous; connection.Close() })
			page, err := CataloguePage(context.Background(), 1, 10, PageCursor{After: 2})
			if name == "bounded join" {
				if err != nil || len(page.Items) != 1 || state.queries != 1 {
					t.Fatalf("catalogue queries=%d page=%+v err=%v", state.queries, page, err)
				}
				return
			}
			if err == nil || !state.badScan && !errors.Is(err, outage) {
				t.Fatalf("failure hidden: %v", err)
			}
		})
	}
}
