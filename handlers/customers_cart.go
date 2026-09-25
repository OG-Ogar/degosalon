package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

func AddToCartHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	id, ok := parseID(idSegment)
	if !ok {
		http.NotFound(w, r)
		return
	}

	product, found := storage.FindProductByID(id)

	if !found {
		http.NotFound(w, r)
		return
	}

	if !product.IsAvailable {
		http.Error(w, "This product is not available.", http.StatusConflict)
		return
	}

	if product.Quantity <= 0 {
		http.Error(w, "This product is out of stock.", http.StatusConflict)
		return
	}

	_, err := storage.AddToCart(customer.ID, product.ID)

	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}

	if err != nil {
		serverError(w, "add product to cart", err)
		return
	}

	http.Redirect(w, r, "/customer/products", http.StatusSeeOther)
}
