package storage

import (
	"barber-booking/internal/money"
	"barber-booking/models"
	"context"
	"database/sql"
	"sort"
)

type CartCard struct {
	Product  models.Product
	Barber   models.Barber
	Quantity int
}
type CartSummary struct {
	Total             money.Amount
	AmountUnavailable bool
	Lines             int
}

func CartPage(ctx context.Context, customerID int, cursor PageCursor) (Page[CartCard], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[CartCard]{}, err
	}
	cursor, err := cursor.Normalize()
	if err != nil || customerID <= 0 {
		return Page[CartCard]{}, ErrInvalidPage
	}
	var items []CartCard
	if databaseEnabled() {
		args := []any{customerID}
		query := `select p.id,p.barber_id,p.name,p.category,p.description,p.price,p.quantity,p.image,p.is_available,p.is_archived,s.id,s.full_name,s.shop_name,c.quantity from public.carts c join public.products p on p.id=c.product_id join public.barbers s on s.id=p.barber_id where c.customer_id=$1` + pageSQL("c.product_id", cursor, true, &args)
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (CartCard, error) {
			var v CartCard
			err := rows.Scan(&v.Product.ID, &v.Product.BarberID, &v.Product.Name, &v.Product.Category, &v.Product.Description, &v.Product.Price, &v.Product.Quantity, &v.Product.Image, &v.Product.IsAvailable, &v.Product.Archived, &v.Barber.ID, &v.Barber.FullName, &v.Barber.ShopName, &v.Quantity)
			return v, err
		}, args...)
		if err != nil {
			return Page[CartCard]{}, err
		}
	} else {
		mu.RLock()
		defer mu.RUnlock()
		for productID, quantity := range carts[customerID] {
			if !cursorIncludes(productID, cursor) {
				continue
			}
			p, ok := findProductByID(productID)
			if !ok {
				continue
			}
			s, ok := findBarberByID(p.BarberID)
			if !ok {
				continue
			}
			items = append(items, CartCard{Product: p, Quantity: quantity, Barber: models.Barber{ID: s.ID, FullName: s.FullName, ShopName: s.ShopName}})
		}
		sort.Slice(items, func(i, j int) bool {
			if pageReverse(cursor, true) {
				return items[i].Product.ID > items[j].Product.ID
			}
			return items[i].Product.ID < items[j].Product.ID
		})
	}
	return finishPage(items, cursor, true), nil
}
func CustomerCartSummary(ctx context.Context, customerID int) (CartSummary, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return CartSummary{}, err
	}
	var result CartSummary
	if databaseEnabled() {
		var total string
		err := db.QueryRowContext(ctx, `select coalesce(sum(p.price*c.quantity),0)::text,coalesce(bool_or(c.quantity<=0),false),count(*) from public.carts c join public.products p on p.id=c.product_id join public.barbers s on s.id=p.barber_id where c.customer_id=$1`, customerID).Scan(&total, &result.AmountUnavailable, &result.Lines)
		if err != nil {
			return result, err
		}
		result.Total, err = money.Parse(total)
		if err != nil {
			result.AmountUnavailable = true
			result.Total = 0
		}
		return result, nil
	}
	mu.RLock()
	defer mu.RUnlock()
	for productID, quantity := range carts[customerID] {
		p, ok := findProductByID(productID)
		if !ok {
			continue
		}
		if _, ok := findBarberByID(p.BarberID); !ok {
			continue
		}
		result.Lines++
		line, err := money.Line(p.Price, quantity)
		if err != nil || quantity <= 0 {
			result.AmountUnavailable = true
			continue
		}
		result.Total, err = result.Total.Add(line)
		if err != nil {
			result.AmountUnavailable = true
		}
	}
	return result, nil
}
