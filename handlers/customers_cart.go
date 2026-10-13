package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"barber-booking/internal/money"
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
		if wantsJSON(r) {
			writeCartJSON(w, http.StatusConflict, map[string]any{"ok": false, "message": "No more stock available for this product."})
			return
		}
		renderCustomerCart(w, r, customer, s, http.StatusConflict, "This product is unavailable or does not have enough stock. Adjust or remove it below, or continue shopping.")
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		if wantsJSON(r) {
			writeCartJSON(w, http.StatusNotFound, map[string]any{"ok": false, "message": "Product not found."})
			return
		}
		journeyError(w, "Page not found.", http.StatusNotFound)
		return
	}
	if err != nil {
		if wantsJSON(r) {
			writeCartJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Could not add to cart. Try again."})
			return
		}
		serverError(w, "add product to cart", err)
		return
	}

	if wantsJSON(r) {
		count, err := storage.CartItemCount(r.Context(), customer.ID)
		if err != nil {
			writeCartJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "message": "Could not read your cart."})
			return
		}
		writeCartJSON(w, http.StatusOK, map[string]any{"ok": true, "cartCount": count})
		return
	}

	destination := "/customer/products"
	if seller, ok := parseID(r.URL.Query().Get("barber")); ok {
		destination = discoveryPath(destination, map[string][]string{"barber": {strconv.Itoa(seller)}})
	}
	http.Redirect(w, r, destination, http.StatusSeeOther)
}

type customerCartItem struct {
	Product           models.Product
	Quantity          int
	Subtotal          float64
	Barber            models.Barber
	AmountUnavailable bool
}

type customerCartView struct {
	Pagination        listPagination
	LineCount         int
	Items             []customerCartItem
	Total             float64
	CSRF              string
	Problem           string
	CheckoutKey       string
	AmountUnavailable bool
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
	cursor := storage.PageCursor{}
	if r.Method == http.MethodGet {
		var ok bool
		cursor, ok = readPageCursor(w, r)
		if !ok {
			return
		}
	}
	page, err := storage.CartPage(r.Context(), customer.ID, cursor)
	if err != nil {
		serverError(w, "load cart page", err)
		return
	}
	summary, err := storage.CustomerCartSummary(r.Context(), customer.ID)
	if err != nil {
		serverError(w, "load cart total", err)
		return
	}
	first, last := 0, 0
	if len(page.Items) > 0 {
		first, last = page.Items[0].Product.ID, page.Items[len(page.Items)-1].Product.ID
	}
	listRequest := r.Clone(r.Context())
	address := *r.URL
	address.Path = "/customer/cart"
	if r.Method != http.MethodGet {
		address.RawQuery = ""
	}
	listRequest.URL = &address
	pagination := paginationLinks(listRequest, cursor, first, last, true, page.HasPrevious, page.HasNext)
	if len(page.Items) == 0 && (cursor.Before > 0 || cursor.After > 0) {
		http.Redirect(w, r, pagination.FirstLink, 303)
		return
	}
	items := make([]customerCartItem, 0, len(page.Items))
	amountUnavailable := summary.AmountUnavailable
	if amountUnavailable {
		problem = "Cart amount exceeds the supported price or quantity limit. Reduce quantities or remove items."
	}
	for _, row := range page.Items {
		subtotal, err := money.Line(row.Product.Price, row.Quantity)
		items = append(items, customerCartItem{Product: row.Product, Barber: row.Barber, Quantity: row.Quantity, Subtotal: subtotal.Float(), AmountUnavailable: err != nil || row.Quantity <= 0})
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
		Items:             items,
		CheckoutKey:       key,
		Total:             summary.Total.Float(),
		Pagination:        pagination,
		LineCount:         summary.Lines,
		CSRF:              s.CSRFToken,
		Problem:           problem,
		AmountUnavailable: amountUnavailable,
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

// wantsJSON is true for the background "quick add" request from the products page.
func wantsJSON(r *http.Request) bool {
	return r.Header.Get("X-Quick-Add") == "1"
}

func writeCartJSON(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
