package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"errors"
)

var carts = make(map[int]map[int]int)

func AddToCart(customerID, productID int) (int, error) {
	if databaseEnabled() {
		var stock int
		var available bool
		var barberStatus string
		err := db.QueryRow(`select p.quantity,p.is_available,b.status from public.products p join public.barbers b on b.id=p.barber_id where p.id=$1`, productID).Scan(&stock, &available, &barberStatus)
		if err != nil {
			return 0, recordError(err)
		}
		if !available || barberStatus != "Active" {
			return 0, ErrUnavailable
		}
		var quantity int
		err = db.QueryRow(`insert into public.carts(customer_id,product_id,quantity) values($1,$2,1)
			on conflict(customer_id,product_id) do update set quantity=public.carts.quantity+1
			where public.carts.quantity < $3 returning quantity`, customerID, productID, stock).Scan(&quantity)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, ErrSlotTaken
			}
			return 0, err
		}
		return quantity, nil
	}
	mu.Lock()
	defer mu.Unlock()

	product, found := findProductByID(productID)
	if !found {
		return 0, ErrNotFound
	}

	if !product.IsAvailable {
		return 0, ErrUnavailable
	}
	barber, found := findBarberByID(product.BarberID)
	if !found || !barber.IsActive() {
		return 0, ErrUnavailable
	}
	if product.Quantity <= 0 {
		return 0, ErrSlotTaken
	}

	if carts[customerID] == nil {
		carts[customerID] = make(map[int]int)
	}

	if carts[customerID][productID] >= product.Quantity {
		return 0, ErrSlotTaken
	}
	carts[customerID][productID]++
	return carts[customerID][productID], nil
}

func CartItemCount(ctx context.Context, customerID int) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if databaseEnabled() {
		var total int
		err := db.QueryRowContext(ctx, "select coalesce(sum(quantity),0) from public.carts where customer_id=$1", customerID).Scan(&total)
		return total, err
	}
	mu.RLock()
	defer mu.RUnlock()
	total := 0
	for _, quantity := range carts[customerID] {
		total += quantity
	}
	return total, nil
}

func ListCartItems(ctx context.Context, customerID int) ([]models.CartItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if databaseEnabled() {
		return queryRecords(ctx, "select product_id,quantity from public.carts where customer_id=$1 order by product_id", func(rows *sql.Rows) (models.CartItem, error) {
			var v models.CartItem
			err := rows.Scan(&v.ProductID, &v.Quantity)
			return v, err
		}, customerID)
	}
	mu.RLock()
	defer mu.RUnlock()
	items := make([]models.CartItem, 0, len(carts[customerID]))
	for productID, quantity := range carts[customerID] {
		items = append(items, models.CartItem{ProductID: productID, Quantity: quantity})
	}
	return items, nil
}

func ClearCart(customerID int) error {
	if databaseEnabled() {
		r, err := db.Exec("delete from public.carts where customer_id=$1", customerID)
		if err != nil {
			return err
		}
		n, _ := r.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	mu.Lock()
	defer mu.Unlock()

	if _, found := carts[customerID]; !found {
		return ErrNotFound
	}

	delete(carts, customerID)

	return nil
}
