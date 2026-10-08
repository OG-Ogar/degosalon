package storage

import (
	"barber-booking/internal/money"
	"barber-booking/internal/testdb"
	"barber-booking/models"
	"context"
	"database/sql"
	"testing"
)

func checkCartPages(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	seller := newBarber(t, "cart-page-seller@example.test")
	buyer := newCustomer(t, "cart-page-buyer@example.test")
	other := newCustomer(t, "cart-page-other@example.test")
	for i := 0; i < 26; i++ {
		product, err := CreateProduct(models.Product{BarberID: seller.ID, Name: "Cart product", Category: "Care", Price: 1250.25, Quantity: 2, IsAvailable: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := AddToCart(buyer.ID, product.ID); err != nil {
			t.Fatal(err)
		}
	}
	first, err := CartPage(ctx, buyer.ID, PageCursor{})
	if err != nil || len(first.Items) != 24 || !first.HasNext || first.Items[0].Barber.PasswordHash != "" {
		t.Fatalf("cart %+v %v", first, err)
	}
	next, err := CartPage(ctx, buyer.ID, PageCursor{After: first.Items[23].Product.ID})
	if err != nil || len(next.Items) != 2 || !next.HasPrevious || next.HasNext {
		t.Fatal("next cart")
	}
	back, err := CartPage(ctx, buyer.ID, PageCursor{Before: next.Items[0].Product.ID})
	if err != nil || len(back.Items) != 24 || back.Items[0].Product.ID != first.Items[0].Product.ID {
		t.Fatal("back cart")
	}
	summary, err := CustomerCartSummary(ctx, buyer.ID)
	if err != nil || summary.Lines != 26 || summary.Total != money.Amount(3250650) || summary.AmountUnavailable {
		t.Fatalf("summary %+v %v", summary, err)
	}
	empty, err := CartPage(ctx, other.ID, PageCursor{})
	if err != nil || len(empty.Items) != 0 {
		t.Fatal("cross customer cart")
	}
	expensive, err := CreateProduct(models.Product{BarberID: seller.ID, Name: "Expensive", Price: 9000000000, Quantity: 2, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := AddToCart(buyer.ID, expensive.ID); err != nil {
			t.Fatal(err)
		}
	}
	summary, err = CustomerCartSummary(ctx, buyer.ID)
	if err != nil || !summary.AmountUnavailable {
		t.Fatal("overflow omitted from total on another page")
	}
}
func TestCartPages(t *testing.T) { Reset(); t.Cleanup(Reset); checkCartPages(t) }
func TestPostgresCartPages(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	checkCartPages(t)
}
