package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"sort"
)

func CustomerOrderPage(ctx context.Context, customerID int, cursor PageCursor) (Page[models.ProductOrder], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[models.ProductOrder]{}, err
	}
	cursor, err := cursor.Normalize()
	if err != nil || customerID <= 0 {
		return Page[models.ProductOrder]{}, ErrInvalidPage
	}
	var items []models.ProductOrder
	if databaseEnabled() {
		args := []any{customerID}
		query := `select id,customer_id,customer_name,customer_phone,customer_email,total_amount,status,created_at from public.product_orders where customer_id=$1`
		query += pageSQL("id", cursor, false, &args)
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (models.ProductOrder, error) {
			var v models.ProductOrder
			err := rows.Scan(&v.ID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.TotalAmount, &v.Status, &v.CreatedAt)
			return v, err
		}, args...)
		if err != nil {
			return Page[models.ProductOrder]{}, err
		}
	} else {
		all, err := ListProductOrdersByCustomer(ctx, customerID)
		if err != nil {
			return Page[models.ProductOrder]{}, err
		}
		for _, v := range all {
			if cursorIncludes(v.ID, cursor) {
				items = append(items, v)
			}
		}
		sort.Slice(items, func(i, j int) bool {
			if pageReverse(cursor, false) {
				return items[i].ID < items[j].ID
			}
			return items[i].ID > items[j].ID
		})
	}
	return finishPage(items, cursor, false), nil
}
func SellerOrderItemPage(ctx context.Context, barberID int, status string, cursor PageCursor) (Page[models.ProductOrderItem], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[models.ProductOrderItem]{}, err
	}
	cursor, err := cursor.Normalize()
	if err != nil || barberID <= 0 {
		return Page[models.ProductOrderItem]{}, ErrInvalidPage
	}
	if status != "" && status != "all" && status != "Pending" && status != "Ready" && status != "Collected" && status != "Cancelled" {
		return Page[models.ProductOrderItem]{}, ErrInvalidPage
	}
	var items []models.ProductOrderItem
	if databaseEnabled() {
		args := []any{barberID}
		query := `select i.id,i.order_id,i.product_id,i.barber_id,i.product_name,i.price,i.quantity,i.subtotal,o.customer_name,o.customer_phone,o.customer_email,o.status,i.fulfillment_status,i.seller_name,i.seller_shop_name,i.seller_shop_address,i.seller_phone,i.product_category,i.product_description,coalesce(p.image,'') from public.product_order_items i join public.product_orders o on o.id=i.order_id left join public.products p on p.id=i.product_id where i.barber_id=$1`
		if status != "" && status != "all" {
			args = append(args, status)
			query += " and i.fulfillment_status=$2"
		}
		query += pageSQL("i.id", cursor, false, &args)
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (models.ProductOrderItem, error) {
			var v models.ProductOrderItem
			err := rows.Scan(&v.ID, &v.OrderID, &v.ProductID, &v.BarberID, &v.ProductName, &v.Price, &v.Quantity, &v.Subtotal, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.OrderStatus, &v.FulfillmentStatus, &v.SellerName, &v.SellerShopName, &v.SellerShopAddress, &v.SellerPhone, &v.ProductCategory, &v.ProductDescription, &v.ProductImage)
			return v, err
		}, args...)
		if err != nil {
			return Page[models.ProductOrderItem]{}, err
		}
	} else {
		all, err := ListProductOrderItemsByBarber(ctx, barberID)
		if err != nil {
			return Page[models.ProductOrderItem]{}, err
		}
		for _, v := range all {
			if cursorIncludes(v.ID, cursor) && (status == "" || status == "all" || v.FulfillmentStatus == status) {
				items = append(items, v)
			}
		}
		sort.Slice(items, func(i, j int) bool {
			if pageReverse(cursor, false) {
				return items[i].ID < items[j].ID
			}
			return items[i].ID > items[j].ID
		})
	}
	return finishPage(items, cursor, false), nil
}
