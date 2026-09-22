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

	render(
		w,
		"customer/products.html",
		struct {
			Products []customerProductRow
			CSRF     string
		}{
			Products: rows,
			CSRF:     s.CSRFToken,
		},
	)
}
