package storage

import (
	"context"
	"errors"
	"testing"

	"barber-booking/internal/testdb"
	"barber-booking/models"
)

func TestOrderSnapshots(t *testing.T) { Reset(); t.Cleanup(Reset); checkOrderSnapshots(t) }

func TestPostgresOrderSnapshots(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	previous := db
	connection, err := openDatabase(address)
	if err != nil {
		t.Fatal(err)
	}
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	orderID := checkOrderSnapshots(t)
	replacement, err := openDatabase(address)
	if err != nil {
		t.Fatal(err)
	}
	db = replacement
	t.Cleanup(func() { replacement.Close() })
	connection.Close()
	items, err := ListProductOrderItems(context.Background(), orderID)
	if err != nil || len(items) != 1 || items[0].SellerShopName != "Original shop" || items[0].ProductDescription != "Original description" {
		t.Fatalf("restart lost snapshots: %+v %v", items, err)
	}
	if _, err := db.Exec("update public.product_order_items set seller_name='Rewritten' where order_id=$1", orderID); err == nil {
		t.Fatal("SQL rewrote seller history")
	}
	if _, err := db.Exec("update public.product_order_items set quantity=quantity+1 where order_id=$1", orderID); err == nil {
		t.Fatal("SQL rewrote original quantities")
	}
	if err := FulfillOrderItem(context.Background(), items[0].ID, items[0].BarberID, models.FulfillmentReady); err != nil {
		t.Fatal("immutable details prevented fulfillment", err)
	}
}

func checkOrderSnapshots(t *testing.T) int {
	t.Helper()
	ctx := context.Background()
	seller, buyer, _ := postgresBookingFixture(t)
	seller.FullName = "Original seller"
	seller.ShopName = "Original shop"
	seller.ShopAddress = "Original address"
	seller.PhoneNumber = "08012345678"
	if err := UpdateBarber(seller); err != nil {
		t.Fatal(err)
	}
	product := postgresProduct(t, seller.ID, 5)
	product.Name = "Original product"
	product.Description = "Original description"
	product.Category = "Original category"
	product.Price = 1234.50
	if err := UpdateProduct(product); err != nil {
		t.Fatal(err)
	}
	if _, err := AddToCart(buyer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	const key = "0123456789abcdef0123456789abcdef"
	order, err := CheckoutCartWithKey(ctx, buyer, key)
	if err != nil {
		t.Fatal(err)
	}
	original, err := ListProductOrderItems(ctx, order.ID)
	if err != nil || len(original) != 1 {
		t.Fatalf("items %+v %v", original, err)
	}
	want := original[0]
	if want.SellerName != seller.FullName || want.SellerShopName != seller.ShopName || want.SellerShopAddress != seller.ShopAddress || want.SellerPhone != seller.PhoneNumber || want.ProductName != product.Name || want.ProductCategory != product.Category || want.ProductDescription != product.Description || want.Price != product.Price || want.Subtotal != product.Price || want.Quantity != 1 {
		t.Fatalf("missing snapshots: %+v", want)
	}
	seller.FullName = "Edited seller"
	seller.ShopName = "Edited shop"
	seller.ShopAddress = "Edited address"
	seller.PhoneNumber = "08087654321"
	if err := UpdateBarber(seller); err != nil {
		t.Fatal(err)
	}
	product, err = FindProductByIDResult(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	product.Name = "Edited product"
	product.Description = "Edited description"
	product.Category = "Edited category"
	product.Price = 2000
	if err := UpdateProduct(product); err != nil {
		t.Fatal(err)
	}
	if _, err := AddToCart(buyer.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	replay, err := CheckoutCartWithKey(ctx, buyer, key)
	if err != nil || replay.ID != order.ID {
		t.Fatal("replay changed order", err)
	}
	unchanged, err := ListProductOrderItems(ctx, order.ID)
	if err != nil || len(unchanged) != 1 || unchanged[0] != want {
		t.Fatalf("edits/replay rewrote history: %+v %v", unchanged, err)
	}
	next, err := CheckoutCartContext(ctx, buyer)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := ListProductOrderItems(ctx, next.ID)
	if err != nil || len(changed) != 1 || changed[0].SellerName != seller.FullName || changed[0].SellerShopAddress != seller.ShopAddress || changed[0].ProductDescription != product.Description || changed[0].Price != product.Price {
		t.Fatalf("new order did not adopt current metadata: %+v %v", changed, err)
	}
	sales, err := ListProductOrderItemsByBarber(ctx, seller.ID)
	if err != nil || len(sales) != 2 {
		t.Fatal("seller history missing", err)
	}
	for _, item := range sales {
		if item.OrderID == order.ID && (item.SellerName != want.SellerName || item.ProductDescription != want.ProductDescription) {
			t.Fatal("seller history lost snapshots")
		}
	}
	supplied := models.ProductOrderItem{OrderID: next.ID, ProductID: product.ID, BarberID: seller.ID, ProductName: product.Name, Quantity: 1, Price: product.Price, Subtotal: product.Price, SellerName: "Forged seller", SellerShopAddress: "Forged address", ProductDescription: "Forged description"}
	stored, err := CreateProductOrderItem(supplied)
	if err != nil || stored.SellerName != seller.FullName || stored.SellerShopAddress != seller.ShopAddress || stored.ProductDescription != product.Description {
		t.Fatalf("compatibility insertion trusted supplied metadata: %+v %v", stored, err)
	}
	supplied.BarberID = seller.ID + 99999
	if _, err := CreateProductOrderItem(supplied); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross seller insertion accepted", err)
	}
	return order.ID
}
