package handlers

import "net/http"

func CustomerRouter(w http.ResponseWriter, r *http.Request) {
	defer cleanupMultipart(r)
	parts := pathParts(r.URL.Path)

	if len(parts) == 1 {
		http.Redirect(w, r, "/customer/dashboard", http.StatusSeeOther)
		return
	}

	if len(parts) == 2 && parts[1] == "login" {
		CustomerLoginHandler(w, r)
		return
	}

	if len(parts) == 2 && parts[1] == "register" {
		CustomerRegisterHandler(w, r)
		return
	}

	if len(parts) == 2 && parts[1] == "forgot-password" {
		CustomerForgotPasswordHandler(w, r)
		return
	}

	if len(parts) == 2 && parts[1] == "reset-password" {
		CustomerResetPasswordHandler(w, r)
		return
	}

	customer, s, ok := requireCustomer(w, r)

	if !ok {
		return
	}

	switch {

	case len(parts) == 2 && parts[1] == "logout":
		CustomerLogoutHandler(w, r, s)

	case len(parts) == 2 && parts[1] == "dashboard":
		CustomerDashboardHandler(w, r, customer, s)

	case len(parts) == 2 && parts[1] == "profile":
		CustomerProfileHandler(w, r, customer, s)

	case len(parts) == 3 && parts[1] == "profile" && parts[2] == "edit":
		EditCustomerProfileHandler(w, r, customer, s)

	case len(parts) == 2 && parts[1] == "barbers":
		BrowseBarbersHandler(w, r, customer, s)

	case len(parts) == 4 && parts[1] == "barbers" && parts[3] == "reviews":
		CreateBarberReviewHandler(w, r, customer, s, parts[2])

	case len(parts) == 3 && parts[1] == "barbers":
		ViewBarberHandler(w, r, customer, s, parts[2])

	case len(parts) == 2 && parts[1] == "book":
		BookHairstyleHandler(w, r, customer, s)

	case len(parts) == 3 && parts[1] == "booking-confirmation":
		BookingConfirmationHandler(w, r, customer, s, parts[2])

	case len(parts) == 3 && parts[1] == "order-confirmation":
		CustomerProductOrderConfirmationHandler(w, r, customer, s, parts[2])

	case len(parts) == 2 && parts[1] == "bookings":
		CustomerBookingsHandler(w, r, customer, s)

	case len(parts) == 4 && parts[1] == "bookings" && parts[3] == "cancel":
		CancelBookingHandler(w, r, customer, s, parts[2])

	case len(parts) == 2 && parts[1] == "chats":
		CustomerChatsHandler(w, r, customer, s)

	case len(parts) == 3 && parts[1] == "chats":
		CustomerChatHandler(w, r, customer, s, parts[2])

	case len(parts) == 2 && parts[1] == "products":
		CustomerProductsHandler(w, r, customer, s)

	case len(parts) == 4 && parts[1] == "cart" && parts[3] == "update":
		UpdateCartItemHandler(w, r, customer, s, parts[2])

	case len(parts) == 2 && parts[1] == "cart":
		CustomerCartHandler(w, r, customer, s)

	case len(parts) == 3 &&
		parts[1] == "products" &&
		(parts[2] == "pay" || parts[2] == "checkout"):
		CustomerProductOrderHandler(w, r, customer, s)

	case len(parts) == 4 &&
		parts[1] == "products" &&
		parts[3] == "add-to-cart":
		AddToCartHandler(w, r, customer, s, parts[2])

	default:
		journeyError(w, "Page not found.", http.StatusNotFound)
	}
}
