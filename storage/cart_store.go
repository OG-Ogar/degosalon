package storage

import (
	"barber-booking/models"
)

var carts = make(map[int]map[int]int)

func AddToCart(customerID, productID int) (int, error) {
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

	return carts[customerID][productID], nil
}

func CartItemCount(customerID int) int {
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
