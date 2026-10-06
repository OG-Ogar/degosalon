package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"errors"
)

var carts = make(map[int]map[int]int)

var ErrInsufficientStock = errors.New("not enough product stock")

func AddToCart(customerID, productID int) (int, error) {
	return AddToCartContext(context.Background(), customerID, productID)
}

func lockCustomerCart(ctx context.Context, tx *sql.Tx, customerID int) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	var id int
	// Lock the customer, not just existing cart rows: an empty/new cart is
	// serialized too, including additions of products not yet in the cart.
	return recordError(tx.QueryRowContext(ctx, "select id from public.customers where id=$1 for update", customerID).Scan(&id))
}

func AddToCartContext(ctx context.Context, customerID, productID int) (int, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if databaseEnabled() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		defer tx.Rollback()
		if err := lockCustomerCart(ctx, tx, customerID); err != nil {
			return 0, err
		}
		var stock int
		var available bool
		var status string
		err = tx.QueryRowContext(ctx, `select p.quantity,p.is_available,b.status from public.products p join public.barbers b on b.id=p.barber_id where p.id=$1 for share of p,b`, productID).Scan(&stock, &available, &status)
		if err != nil {
			return 0, recordError(err)
		}
		if !available || status != models.BarberActive {
			return 0, ErrUnavailable
		}
		if stock <= 0 {
			return 0, ErrInsufficientStock
		}
		var quantity int
		err = tx.QueryRowContext(ctx, `insert into public.carts(customer_id,product_id,quantity) values($1,$2,1)
   on conflict(customer_id,product_id) do update set quantity=public.carts.quantity+1
   where public.carts.quantity < $3 returning quantity`, customerID, productID, stock).Scan(&quantity)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrInsufficientStock
		}
		if err != nil {
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return quantity, nil
	}
	mu.Lock()
	defer mu.Unlock()
	if _, found := findCustomerByID(customerID); !found {
		return 0, ErrNotFound
	}
	product, found := findProductByID(productID)
	if !found {
		return 0, ErrNotFound
	}
	barber, found := findBarberByID(product.BarberID)
	if !product.IsAvailable || !found || !barber.IsActive() {
		return 0, ErrUnavailable
	}
	if product.Quantity <= 0 || carts[customerID][productID] >= product.Quantity {
		return 0, ErrInsufficientStock
	}
	if carts[customerID] == nil {
		carts[customerID] = make(map[int]int)
	}
	carts[customerID][productID]++
	return carts[customerID][productID], nil
}

// SetCartQuantity edits an existing owned line. Zero removes it, even when the
// seller is suspended or the item is unavailable. Cart edits reserve no stock.
func SetCartQuantity(ctx context.Context, customerID, productID, quantity int) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}
	if quantity < 0 || quantity > 2147483647 {
		return ErrForbidden
	}
	if databaseEnabled() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err := lockCustomerCart(ctx, tx, customerID); err != nil {
			return err
		}
		var current int
		if err := tx.QueryRowContext(ctx, "select quantity from public.carts where customer_id=$1 and product_id=$2", customerID, productID).Scan(&current); err != nil {
			return recordError(err)
		}
		if quantity == 0 {
			if _, err := tx.ExecContext(ctx, "delete from public.carts where customer_id=$1 and product_id=$2", customerID, productID); err != nil {
				return err
			}
		} else {
			var stock int
			var available bool
			var status string
			err = tx.QueryRowContext(ctx, `select p.quantity,p.is_available,b.status from public.products p join public.barbers b on b.id=p.barber_id where p.id=$1 for share of p,b`, productID).Scan(&stock, &available, &status)
			if err != nil {
				return recordError(err)
			}
			if !available || status != models.BarberActive {
				return ErrUnavailable
			}
			if quantity > stock {
				return ErrInsufficientStock
			}
			if _, err := tx.ExecContext(ctx, "update public.carts set quantity=$1 where customer_id=$2 and product_id=$3", quantity, customerID, productID); err != nil {
				return err
			}
		}
		return tx.Commit()
	}
	mu.Lock()
	defer mu.Unlock()
	if _, found := carts[customerID][productID]; !found {
		return ErrNotFound
	}
	if quantity == 0 {
		delete(carts[customerID], productID)
		return nil
	}
	product, found := findProductByID(productID)
	if !found {
		return ErrUnavailable
	}
	barber, found := findBarberByID(product.BarberID)
	if !product.IsAvailable || !found || !barber.IsActive() {
		return ErrUnavailable
	}
	if quantity > product.Quantity {
		return ErrInsufficientStock
	}
	carts[customerID][productID] = quantity
	return nil
}

func CartItemCount(ctx context.Context, customerID int) (int, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

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
	ctx, cancel := databaseContext(ctx)
	defer cancel()

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

func ClearCart(customerID int) error { return ClearCartContext(context.Background(), customerID) }

func ClearCartContext(ctx context.Context, customerID int) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}

	if databaseEnabled() {

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err := lockCustomerCart(ctx, tx, customerID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, "delete from public.carts where customer_id=$1", customerID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count == 0 {
			return ErrNotFound
		}
		return tx.Commit()
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(carts[customerID]) == 0 {
		return ErrNotFound
	}
	delete(carts, customerID)
	return nil
}
