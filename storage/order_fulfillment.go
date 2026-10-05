package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"errors"
)

func fulfillmentSources(target string) []string {
	switch target {
	case models.FulfillmentReady:
		return []string{models.FulfillmentPending}
	case models.FulfillmentCollected:
		return []string{models.FulfillmentReady}
	case models.FulfillmentCancelled:
		return []string{models.FulfillmentPending, models.FulfillmentReady}
	}
	return nil
}
func fulfillmentAllowed(current, target string) bool {
	for _, source := range fulfillmentSources(target) {
		if source == current {
			return true
		}
	}
	return false
}
func aggregateFulfillment(states []string) string {
	pending, ready, collected, cancelled := 0, 0, 0, 0
	for _, s := range states {
		switch s {
		case models.FulfillmentPending:
			pending++
		case models.FulfillmentReady:
			ready++
		case models.FulfillmentCollected:
			collected++
		case models.FulfillmentCancelled:
			cancelled++
		}
	}
	if pending > 0 {
		return models.OrderPending
	}
	if ready > 0 {
		return models.OrderReady
	}
	if collected > 0 {
		return models.OrderCompleted
	}
	if cancelled > 0 {
		return models.OrderCancelled
	}
	return models.OrderPending
}

// FulfillOrderItem atomically changes one seller-owned item, restores cancelled
// stock once, and recomputes the whole order status across every seller.
func FulfillOrderItem(ctx context.Context, id, barberID int, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if barberID <= 0 {
		return ErrForbidden
	}
	if len(fulfillmentSources(target)) == 0 {
		return ErrInvalidTransition
	}
	if databaseEnabled() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var orderID int
		err = tx.QueryRowContext(ctx, "select order_id from public.product_order_items where id=$1 and barber_id=$2", id, barberID).Scan(&orderID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		// Serialize all items of an order before locking one item to avoid stale
		// aggregate states when two different sellers act concurrently.
		var locked int
		if err = tx.QueryRowContext(ctx, "select id from public.product_orders where id=$1 for update", orderID).Scan(&locked); err != nil {
			return err
		}
		var productID, quantity int
		var current string
		err = tx.QueryRowContext(ctx, "select product_id,quantity,fulfillment_status from public.product_order_items where id=$1 and barber_id=$2 for update", id, barberID).Scan(&productID, &quantity, &current)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !fulfillmentAllowed(current, target) {
			return ErrInvalidTransition
		}
		if target == models.FulfillmentCancelled {
			result, err := tx.ExecContext(ctx, "update public.products set quantity=quantity+$1 where id=$2 and barber_id=$3", quantity, productID, barberID)
			if err != nil {
				return err
			}
			n, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if n != 1 {
				return ErrNotFound
			}
		}
		if _, err = tx.ExecContext(ctx, "update public.product_order_items set fulfillment_status=$1 where id=$2", target, id); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, "select fulfillment_status from public.product_order_items where order_id=$1", orderID)
		if err != nil {
			return err
		}
		var states []string
		for rows.Next() {
			var state string
			if err = rows.Scan(&state); err != nil {
				rows.Close()
				return err
			}
			states = append(states, state)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "update public.product_orders set status=$1 where id=$2", aggregateFulfillment(states), orderID); err != nil {
			return err
		}
		return tx.Commit()
	}
	mu.Lock()
	defer mu.Unlock()
	for i := range productOrderItems {
		item := &productOrderItems[i]
		if item.ID != id {
			continue
		}
		if item.BarberID != barberID {
			return ErrNotFound
		}
		if !fulfillmentAllowed(item.FulfillmentStatus, target) {
			return ErrInvalidTransition
		}
		if target == models.FulfillmentCancelled {
			found := false
			for p := range products {
				if products[p].ID == item.ProductID && products[p].BarberID == barberID {
					products[p].Quantity += item.Quantity
					found = true
					break
				}
			}
			if !found {
				return ErrNotFound
			}
		}
		item.FulfillmentStatus = target
		var states []string
		for _, other := range productOrderItems {
			if other.OrderID == item.OrderID {
				states = append(states, other.FulfillmentStatus)
			}
		}
		for o := range productOrders {
			if productOrders[o].ID == item.OrderID {
				productOrders[o].Status = aggregateFulfillment(states)
				break
			}
		}
		return nil
	}
	return ErrNotFound
}
