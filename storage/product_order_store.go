package storage

import (
	"barber-booking/models"
)

// Product order storage.
var (
	productOrders     []models.ProductOrder
	productOrderItems []models.ProductOrderItem
)

// IDs for product orders and order items.
var (
	lastProductOrderID     int
	lastProductOrderItemID int
)

// CreateProductOrder creates and stores a new product order.
func CreateProductOrder(order models.ProductOrder) (models.ProductOrder, error) {
	if databaseEnabled(){err:=db.QueryRow("insert into public.product_orders(customer_id,customer_name,customer_phone,customer_email,total_amount,status,created_at) values($1,$2,$3,$4,$5,$6,$7) returning id",order.CustomerID,order.CustomerName,order.CustomerPhone,order.CustomerEmail,order.TotalAmount,order.Status,order.CreatedAt).Scan(&order.ID);return order,err}
	mu.Lock()
	defer mu.Unlock()

	order.ID = nextID(&lastProductOrderID)

	productOrders = append(productOrders, order)

	return order, nil
}

// CreateProductOrderItem creates and stores a product inside an order.
func CreateProductOrderItem(item models.ProductOrderItem) (models.ProductOrderItem, error) {
	if databaseEnabled(){err:=db.QueryRow("insert into public.product_order_items(order_id,product_id,barber_id,product_name,price,quantity,subtotal) values($1,$2,$3,$4,$5,$6,$7) returning id",item.OrderID,item.ProductID,item.BarberID,item.ProductName,item.Price,item.Quantity,item.Subtotal).Scan(&item.ID);return item,err}
	mu.Lock()
	defer mu.Unlock()

	item.ID = nextID(&lastProductOrderItemID)

	productOrderItems = append(productOrderItems, item)

	return item, nil
}

// FindProductOrderByID finds an order by its ID.
func FindProductOrderByID(id int) (models.ProductOrder, bool) {
	if databaseEnabled(){var v models.ProductOrder;err:=db.QueryRow("select id,customer_id,customer_name,customer_phone,customer_email,total_amount,status,created_at from public.product_orders where id=$1",id).Scan(&v.ID,&v.CustomerID,&v.CustomerName,&v.CustomerPhone,&v.CustomerEmail,&v.TotalAmount,&v.Status,&v.CreatedAt);return v,err==nil}
	mu.RLock()
	defer mu.RUnlock()

	for _, order := range productOrders {
		if order.ID == id {
			return order, true
		}
	}

	return models.ProductOrder{}, false
}

// ListProductOrderItems returns all items belonging to an order.
func ListProductOrderItems(orderID int) []models.ProductOrderItem {
	if databaseEnabled(){rows,err:=db.Query("select id,order_id,product_id,barber_id,product_name,price,quantity,subtotal from public.product_order_items where order_id=$1 order by id",orderID);if err==nil{defer rows.Close();var out []models.ProductOrderItem;for rows.Next(){var v models.ProductOrderItem;if rows.Scan(&v.ID,&v.OrderID,&v.ProductID,&v.BarberID,&v.ProductName,&v.Price,&v.Quantity,&v.Subtotal)==nil{out=append(out,v)}};return out}}
	mu.RLock()
	defer mu.RUnlock()

	items := make([]models.ProductOrderItem, 0)

	for _, item := range productOrderItems {
		if item.OrderID == orderID {
			items = append(items, item)
		}
	}

	return items
}

// ListProductOrdersByCustomer returns all orders belonging to a customer.
func ListProductOrdersByCustomer(customerID int) []models.ProductOrder {
	mu.RLock()
	defer mu.RUnlock()

	orders := make([]models.ProductOrder, 0)

	for _, order := range productOrders {
		if order.CustomerID == customerID {
			orders = append(orders, order)
		}
	}

	return orders
}

// ListProductOrderItemsByBarber returns all purchased product items
// belonging to a specific barber.
func ListProductOrderItemsByBarber(barberID int) []models.ProductOrderItem {
	if databaseEnabled() {
		rows,err:=db.Query(`select i.id,i.order_id,i.product_id,i.barber_id,i.product_name,i.price,i.quantity,i.subtotal,o.customer_name,o.customer_phone,o.customer_email,o.status
			from public.product_order_items i join public.product_orders o on o.id=i.order_id
			where i.barber_id=$1 order by i.id desc`,barberID)
		if err==nil {
			defer rows.Close(); var out []models.ProductOrderItem
			for rows.Next(){var v models.ProductOrderItem;if rows.Scan(&v.ID,&v.OrderID,&v.ProductID,&v.BarberID,&v.ProductName,&v.Price,&v.Quantity,&v.Subtotal,&v.CustomerName,&v.CustomerPhone,&v.CustomerEmail,&v.OrderStatus)==nil{out=append(out,v)}}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()

	items := make([]models.ProductOrderItem, 0)

	for _, item := range productOrderItems {
		if item.BarberID == barberID {
			items = append(items, item)
		}
	}

	return items
}
