package handlers

import (
	"errors"
	"net/http"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

func CustomerProductOrderHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	cartItems := storage.ListCartItems(customer.ID)

	if len(cartItems) == 0 {
		http.Redirect(w, r, "/customer/cart", http.StatusSeeOther)
		return
	}

	type orderItemData struct {
		Product  models.Product
		Barber   models.Barber
		Quantity int
		Subtotal float64
	}

	items := make([]orderItemData, 0, len(cartItems))

	var total float64

	for _, cartItem := range cartItems {
		product, found := storage.FindProductByID(cartItem.ProductID)

		if !found {
			http.Error(w, "A product in your cart no longer exists.", http.StatusConflict)
			return
		}

		if cartItem.Quantity <= 0 {
			http.Error(w, "Invalid product quantity.", http.StatusConflict)
			return
		}

		barber, found := storage.FindBarberByID(product.BarberID)

		if !found {
			http.Error(w, "The barber for a product in your cart no longer exists.", http.StatusConflict)
			return
		}

		subtotal := product.Price * float64(cartItem.Quantity)

		items = append(items, orderItemData{
			Product:  product,
			Barber:   barber,
			Quantity: cartItem.Quantity,
			Subtotal: subtotal,
		})

		total += subtotal
	}

	order, err := storage.CreateProductOrder(models.ProductOrder{
		CustomerID:    customer.ID,
		CustomerName:  customer.FullName,
		CustomerPhone: customer.PhoneNumber,
		CustomerEmail: customer.Email,
		TotalAmount:   total,
		Status:        models.OrderPending,
		CreatedAt:     time.Now(),
	})

	if err != nil {
		serverError(w, "create product order", err)
		return
	}

	for _, item := range items {
		_, err := storage.CreateProductOrderItem(models.ProductOrderItem{
			OrderID:     order.ID,
			ProductID:   item.Product.ID,
			BarberID:    item.Product.BarberID,
			ProductName: item.Product.Name,
			Price:       item.Product.Price,
			Quantity:    item.Quantity,
			Subtotal:    item.Subtotal,
		})

		if err != nil {
			serverError(w, "create product order item", err)
			return
		}
	}

	err = storage.ClearCart(customer.ID)

	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "clear customer cart", err)
		return
	}

	http.Redirect(
		w,
		r,
		"/customer/order-confirmation/"+itoa(order.ID),
		http.StatusSeeOther,
	)
}
