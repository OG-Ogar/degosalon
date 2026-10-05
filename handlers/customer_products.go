package handlers

import (
	"errors"
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

	products, err := storage.ListAvailableProducts(r.Context())
	if err != nil {
		serverError(w, "load product catalogue", err)
		return
	}
	cartCount, err := storage.CartItemCount(r.Context(), customer.ID)
	if err != nil {
		serverError(w, "load cart count", err)
		return
	}

	rows := make([]customerProductRow, 0, len(products))

	for _, product := range products {
		barber, err := storage.FindBarberByIDContext(r.Context(), product.BarberID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			serverError(w, "load product seller", err)
			return
		}
		if errors.Is(err, storage.ErrNotFound) || !barber.IsActive() {
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
		CartCount: cartCount,
	}

	render(w, "customer/products.html", view)
}
