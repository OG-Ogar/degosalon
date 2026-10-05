package storage

import (
	"barber-booking/models"
)

var carts = make(map[int]map[int]int)

func AddToCart(customerID, productID int) (int, error) {
	if databaseEnabled() {
		var stock int; var available bool; var barberStatus string
		err:=db.QueryRow(`select p.quantity,p.is_available,b.status from public.products p join public.barbers b on b.id=p.barber_id where p.id=$1`,productID).Scan(&stock,&available,&barberStatus)
		if err!=nil{return 0,ErrNotFound};if !available||barberStatus!="Active"{return 0,ErrUnavailable}
		var quantity int
		err=db.QueryRow(`insert into public.carts(customer_id,product_id,quantity) values($1,$2,1)
			on conflict(customer_id,product_id) do update set quantity=public.carts.quantity+1
			where public.carts.quantity < $3 returning quantity`,customerID,productID,stock).Scan(&quantity)
		if err!=nil{return 0,ErrSlotTaken};return quantity,nil
	}
	mu.Lock()
	defer mu.Unlock()

	product, found := findProductByID(productID)
	if !found {
		return 0, ErrNotFound
	}

	if !product.IsAvailable { return 0, ErrUnavailable }
	barber, found := findBarberByID(product.BarberID)
	if !found || !barber.IsActive() { return 0, ErrUnavailable }
	if product.Quantity <= 0 { return 0, ErrSlotTaken }

	if carts[customerID] == nil {
		carts[customerID] = make(map[int]int)
	}

	if carts[customerID][productID] >= product.Quantity { return 0, ErrSlotTaken }
	carts[customerID][productID]++
	return carts[customerID][productID], nil
}

func CartItemCount(customerID int) int {
	if databaseEnabled(){var total int;if db.QueryRow("select coalesce(sum(quantity),0) from public.carts where customer_id=$1",customerID).Scan(&total)==nil{return total};return 0}
	mu.RLock()
	defer mu.RUnlock()

	cart := carts[customerID]

	total := 0

	for _, quantity := range cart {
		total += quantity
	}

	return total
}

func ListCartItems(customerID int) []models.CartItem {
	if databaseEnabled(){rows,err:=db.Query("select product_id,quantity from public.carts where customer_id=$1 order by product_id",customerID);if err==nil{defer rows.Close();var out []models.CartItem;for rows.Next(){var v models.CartItem;if rows.Scan(&v.ProductID,&v.Quantity)==nil{out=append(out,v)}};return out}}
	mu.RLock()
	defer mu.RUnlock()

	cart := carts[customerID]

	items := make([]models.CartItem, 0, len(cart))

	for productID, quantity := range cart {
		items = append(items, models.CartItem{
			ProductID: productID,
			Quantity:  quantity,
		})
	}

	return items
}

func ClearCart(customerID int) error {
	if databaseEnabled(){r,err:=db.Exec("delete from public.carts where customer_id=$1",customerID);if err!=nil{return err};n,_:=r.RowsAffected();if n==0{return ErrNotFound};return nil}
	mu.Lock()
	defer mu.Unlock()

	if _, found := carts[customerID]; !found {
		return ErrNotFound
	}

	delete(carts, customerID)

	return nil
}
