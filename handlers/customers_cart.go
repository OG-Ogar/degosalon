package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

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
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}

	_, err := storage.AddToCartContext(r.Context(), customer.ID, id)
	if errors.Is(err, storage.ErrInsufficientStock) || errors.Is(err, storage.ErrUnavailable) {
		renderCustomerCart(w, r, customer, s, http.StatusConflict, "This product is unavailable or does not have enough stock. Adjust or remove it below, or continue shopping.")
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}
	if err != nil {
		serverError(w, "add product to cart", err)
		return
	}

	destination := "/customer/products"
	if seller, ok := parseID(r.URL.Query().Get("barber")); ok {
		destination = discoveryPath(destination, map[string][]string{"barber": {strconv.Itoa(seller)}})
	}
	http.Redirect(w, r, destination, http.StatusSeeOther)
}

type customerCartItem struct {
	Product  models.Product
	Quantity int
	Subtotal float64
	Barber   models.Barber
}

type customerCartView struct {
	Items       []customerCartItem
	Total       float64
	CSRF        string
	Problem     string
	CheckoutKey string
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

	renderCustomerCart(w, r, customer, s, http.StatusOK, "")
}

func renderCustomerCart(w http.ResponseWriter, r *http.Request, customer models.Customer, s sessionData, status int, problem string) {
	cartItems, err := storage.ListCartItems(r.Context(), customer.ID)
	if err != nil {
		serverError(w, "load customer cart", err)
		return
	}

	items := make([]customerCartItem, 0, len(cartItems))

	var total float64

	for _, cartItem := range cartItems {
		product, err := storage.FindProductByIDResult(r.Context(), cartItem.ProductID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			serverError(w, "load cart product", err)
			return
		}

		barber, err := storage.FindBarberByIDContext(r.Context(), product.BarberID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			serverError(w, "load cart barber", err)
			return
		}

		subtotal := product.Price * float64(cartItem.Quantity)

		items = append(items, customerCartItem{
			Product:  product,
			Quantity: cartItem.Quantity,
			Subtotal: subtotal,
			Barber:   barber,
		})

		total += subtotal
	}

	key := r.PostFormValue("checkout_key")
	if !storage.ValidCheckoutKey(key) {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			serverError(w, "create checkout retry key", err)
			return
		}
		key = hex.EncodeToString(random)
	}
	view := customerCartView{
		Items:       items,
		CheckoutKey: key,
		Total:       total,
		CSRF:        s.CSRFToken,
		Problem:     problem,
	}

	renderStatus(w, status, "customer/cart.html", view)
}

func UpdateCartItemHandler(w http.ResponseWriter, r *http.Request, customer models.Customer, s sessionData, idSegment string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !requireCSRF(w, r, s) {
		return
	}
	id, ok := parseID(idSegment)
	if !ok {
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}
	quantity, err := strconv.Atoi(r.PostFormValue("quantity"))
	if r.PostFormValue("remove") == "yes" {
		quantity = 0
		err = nil
	}
	if err != nil || quantity < 0 || quantity > 2147483647 {
		renderCustomerCart(w, r, customer, s, http.StatusBadRequest, "Choose a whole-number quantity of zero or more.")
		return
	}
	err = storage.SetCartQuantity(r.Context(), customer.ID, id, quantity)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		renderCustomerCart(w, r, customer, s, http.StatusNotFound, "That item is no longer in your cart.")
		return
	case errors.Is(err, storage.ErrUnavailable):
		renderCustomerCart(w, r, customer, s, http.StatusConflict, "This item is no longer available. Remove it to continue.")
		return
	case errors.Is(err, storage.ErrInsufficientStock):
		renderCustomerCart(w, r, customer, s, http.StatusConflict, "There is not enough stock for that quantity. Choose a smaller quantity or remove the item.")
		return
	case err != nil:
		serverError(w, "update cart quantity", err)
		return
	}
	http.Redirect(w, r, "/customer/cart", http.StatusSeeOther)
}
