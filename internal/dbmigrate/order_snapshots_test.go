package dbmigrate_test

import (
	"context"
	"os"
	"testing"

	"barber-booking/internal/dbmigrate"
)

func TestPostgresOrderSnapshotBackfill(t *testing.T) {
	conn := migrationDatabase(t)
	baseline, err := os.ReadFile("../../supabase/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(string(baseline)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`
 insert into public.barbers(id,full_name,phone_number,email,password_hash,shop_name,shop_address) values(1,'Retained seller','08012345678','backfill_seller@example.com','hash','Retained shop','Retained address');
 insert into public.customers(id,full_name,phone_number,email,password_hash) values(1,'Retained customer','123','backfill_buyer@example.com','hash');
 insert into public.products(id,barber_id,name,category,description,price,quantity) values(1,1,'Edited catalogue name','Retained category','Retained description',9999,3);
 insert into public.product_orders(id,customer_id,customer_name,customer_phone,customer_email,total_amount) values(1,1,'Original buyer','123','backfill_buyer@example.com',2500);
 insert into public.product_order_items(id,order_id,product_id,barber_id,product_name,price,quantity,subtotal) values(1,1,1,1,'Original purchase name',1250,2,2500);
 `); err != nil {
		t.Fatal(err)
	}
	if err := dbmigrate.Apply(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	verify := func() {
		t.Helper()
		var seller, shop, address, phone, category, description, name string
		var price, subtotal float64
		var quantity int
		err := conn.QueryRow(`select seller_name,seller_shop_name,seller_shop_address,seller_phone,product_category,product_description,product_name,price,quantity,subtotal from public.product_order_items where id=1`).Scan(&seller, &shop, &address, &phone, &category, &description, &name, &price, &quantity, &subtotal)
		if err != nil || seller != "Retained seller" || shop != "Retained shop" || address != "Retained address" || phone != "08012345678" || category != "Retained category" || description != "Retained description" || name != "Original purchase name" || price != 1250 || quantity != 2 || subtotal != 2500 {
			t.Fatalf("backfill lost retained history: %q %q %q %q %q %q %q %v %v %v: %v", seller, shop, address, phone, category, description, name, price, quantity, subtotal, err)
		}
	}
	verify()
	if _, err := conn.Exec(`update public.barbers set full_name='Later seller',shop_name='Later shop' where id=1;update public.products set description='Later description' where id=1`); err != nil {
		t.Fatal(err)
	}
	if err := dbmigrate.Apply(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	verify()
	if _, err := conn.Exec(`update public.product_order_items set seller_name='Forged seller' where id=1`); err == nil {
		t.Fatal("backfilled snapshot could be rewritten")
	}
	if _, err := conn.Exec(`insert into public.product_order_items(order_id,product_id,barber_id,product_name,price,quantity,subtotal) values(1,1,1,'Unsnapshotted',1250,1,1250)`); err == nil {
		t.Fatal("new SQL write omitted required snapshots")
	}
}
