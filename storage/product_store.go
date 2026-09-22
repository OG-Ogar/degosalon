package storage

import "barber-booking/models"

func ListProductsByBarber(barberID int) []models.Product {
	mu.RLock()
	defer mu.RUnlock()

	var owned []models.Product

	for _, product := range products {
		if product.BarberID == barberID {
			owned = append(owned, product)
		}
	}

	return owned
}

func ListAvailableProducts() []models.Product {
	mu.RLock()
	defer mu.RUnlock()

	var available []models.Product

	for _, product := range products {
		if product.IsAvailable {
			available = append(available, product)
		}
	}

	return available
}

func FindProductByID(id int) (models.Product, bool) {
	mu.RLock()
	defer mu.RUnlock()

	return findProductByID(id)
}

func findProductByID(id int) (models.Product, bool) {
	for _, product := range products {
		if product.ID == id {
			return product, true
		}
	}

	return models.Product{}, false
}

func FindProductOwnedBy(id, barberID int) (models.Product, error) {
	mu.RLock()
	defer mu.RUnlock()

	product, found := findProductByID(id)

	if !found {
		return models.Product{}, ErrNotFound
	}

	if product.BarberID != barberID {
		return models.Product{}, ErrForbidden
	}

	return product, nil
}

func CreateProduct(product models.Product) (models.Product, error) {
	mu.Lock()
	defer mu.Unlock()

	product.ID = nextID(&lastProductID)

	products = append(products, product)

	return product, nil
}

func UpdateProduct(product models.Product) error {
	mu.Lock()
	defer mu.Unlock()

	for i := range products {
		if products[i].ID != product.ID {
			continue
		}

		if products[i].BarberID != product.BarberID {
			return ErrForbidden
		}

		products[i] = product

		return nil
	}

	return ErrNotFound
}

func DeleteProduct(id, barberID int) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	for i := range products {
		if products[i].ID != id {
			continue
		}

		if products[i].BarberID != barberID {
			return "", ErrForbidden
		}

		image := products[i].Image

		products = append(products[:i], products[i+1:]...)

		return image, nil
	}

	return "", ErrNotFound
}
