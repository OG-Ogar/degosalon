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
	SellerName string
	SellerID   int
	Products   []customerProductRow
	CSRF       string
	CartCount  int
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

	productID := 0
    if value := r.URL.Query().Get("product"); value != "" {
        var ok bool
        productID, ok = parseID(value)
        if !ok { journeyError(w, "Product not found.", http.StatusNotFound); return }
    }
    sellerID := 0
	sellerName := ""
	if value := r.URL.Query().Get("barber"); value != "" {
		var ok bool
		sellerID, ok = parseID(value)
		if !ok {
			journeyError(w, "Page not found.", http.StatusNotFound)
			return
		}
		seller, err := storage.FindBarberByIDContext(r.Context(), sellerID)
		if errors.Is(err, storage.ErrNotFound) || (err == nil && !seller.IsActive()) {
			journeyError(w, "Page not found.", http.StatusNotFound)
			return
		}
		if err != nil {
			serverError(w, "load catalogue seller", err)
			return
		}
		sellerName = seller.ShopName
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
        if productID > 0 && product.ID != productID { continue }
		if sellerID > 0 && product.BarberID != sellerID {
			continue
		}
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

	if productID > 0 && len(rows) == 0 { journeyError(w, "This product is no longer available.", http.StatusNotFound); return }

	view := customerProductsView{
		Products:   rows,
		SellerName: sellerName,
		SellerID:   sellerID,
		CSRF:       s.CSRFToken,
		CartCount:  cartCount,
	}

	render(w, "customer/products.html", view)
}
