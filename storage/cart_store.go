package storage

import (
	"barber-booking/models"
)

var carts = make(map[int]map[int]int)

func AddToCart(customerID, productID int) (int, error) {
	if databaseEnabled() {
		tx,err:=db.Begin(); if err!=nil{return 0,err}; defer tx.Rollback()
		var stock int
		if err=tx.QueryRow("select quantity from public.products where id=$1 for update",productID).Scan(&stock);err!=nil{return 0,ErrNotFound}
		if stock<=0{return 0,ErrSlotTaken}
		var quantity int
		err=tx.QueryRow("insert into public.carts(customer_id,product_id,quantity) values($1,$2,1) on conflict(customer_id,product_id) do update set quantity=public.carts.quantity+1 returning quantity",customerID,productID).Scan(&quantity);if err!=nil{return 0,err}
		if _,err=tx.Exec("update public.products set quantity=quantity-1 where id=$1",productID);err!=nil{return 0,err}
		if err=tx.Commit();err!=nil{return 0,err};return quantity,nil
	}
	mu.Lock()
	defer mu.Unlock()

	product, found := findProductByID(productID)
	if !found {
		return 0, ErrNotFound
	}

	if product.Quantity <= 0 {
		return 0, ErrSlotTaken
	}

	if carts[customerID] == nil {
		carts[customerID] = make(map[int]int)
	}

	carts[customerID][productID]++

	for i := range products {
		if products[i].ID == productID {
			products[i].Quantity--
			break
		}
	}

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
