package handlers

import (
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

type customerProductRow struct {
	Product models.Product
	Barber  models.Barber
}

type customerProductsView struct {
	Products  []customerProductRow
	CSRF      string
	CartCount int
}

func CustomerProductsHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	products := storage.ListAvailableProducts()

	rows := make([]customerProductRow, 0, len(products))

	for _, product := range products {
		barber, found := storage.FindBarberByID(product.BarberID)

		if !found || !barber.IsActive() {
			continue
		}

		rows = append(rows, customerProductRow{
			Product: product,
			Barber:  barber,
		})
	}

	view := customerProductsView{
		Products:  rows,
		CSRF:      s.CSRFToken,
		CartCount: storage.CartItemCount(customer.ID),
	}

	render(w, "customer/products.html", view)
}
