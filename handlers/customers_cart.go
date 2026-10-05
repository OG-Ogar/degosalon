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

	product, err := storage.FindProductByIDResult(id)
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, "load product for cart", err)
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

	_, err = storage.AddToCart(customer.ID, product.ID)

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

type customerCartItem struct {
	Product  models.Product
	Quantity int
	Subtotal float64
	Barber   models.Barber
}

type customerCartView struct {
	Items []customerCartItem
	Total float64
	CSRF  string
}

func CustomerCartHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	cartItems := storage.ListCartItems(customer.ID)

	items := make([]customerCartItem, 0, len(cartItems))

	var total float64

	for _, cartItem := range cartItems {
		product, err := storage.FindProductByIDResult(cartItem.ProductID)
		if errors.Is(err, storage.ErrNotFound) { continue }
		if err != nil { serverError(w, "load cart product", err); return }

		barber, err := storage.FindBarberByIDResult(product.BarberID)
		if errors.Is(err, storage.ErrNotFound) { continue }
		if err != nil { serverError(w, "load cart barber", err); return }

		subtotal := product.Price * float64(cartItem.Quantity)

		items = append(items, customerCartItem{
			Product:  product,
			Quantity: cartItem.Quantity,
			Subtotal: subtotal,
			Barber:   barber,
		})

		total += subtotal
	}

	view := customerCartView{
		Items: items,
		Total: total,
		CSRF:  s.CSRFToken,
	}

	render(w, "customer/cart.html", view)
}
