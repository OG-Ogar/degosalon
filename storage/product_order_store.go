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
	mu.Lock()
	defer mu.Unlock()

	order.ID = nextID(&lastProductOrderID)

	productOrders = append(productOrders, order)

	return order, nil
}

// CreateProductOrderItem creates and stores a product inside an order.
func CreateProductOrderItem(item models.ProductOrderItem) (models.ProductOrderItem, error) {
	mu.Lock()
	defer mu.Unlock()

	item.ID = nextID(&lastProductOrderItemID)

	productOrderItems = append(productOrderItems, item)

	return item, nil
}

// FindProductOrderByID finds an order by its ID.
func FindProductOrderByID(id int) (models.ProductOrder, bool) {
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
