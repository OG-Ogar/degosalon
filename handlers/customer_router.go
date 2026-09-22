package handlers

import (
	"net/http"
)

// CustomerRouter dispatches every /customer/ URL.
//
// Only login and register are public. Everything after requireCustomer
// has a signed-in customer, and each handler receives that customer
// rather than reading an ID out of the request, so one customer cannot
// act as another by editing a form field.
func CustomerRouter(w http.ResponseWriter, r *http.Request) {
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

	// A visitor who cannot log in has to be able to reach these two
	// without a session, since not knowing the password is the point.
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

	case len(parts) == 3 &&
		parts[1] == "profile" &&
		parts[2] == "edit":
		EditCustomerProfileHandler(w, r, customer, s)

	case len(parts) == 2 && parts[1] == "barbers":
		BrowseBarbersHandler(w, r, customer, s)

	case len(parts) == 3 && parts[1] == "barbers":
		ViewBarberHandler(w, r, customer, s, parts[2])

	case len(parts) == 2 && parts[1] == "products":
		CustomerProductsHandler(w, r, customer, s)

	case len(parts) == 2 && parts[1] == "book":
		BookHairstyleHandler(w, r, customer, s)

	case len(parts) == 3 &&
		parts[1] == "booking-confirmation":
		BookingConfirmationHandler(
			w,
			r,
			customer,
			s,
			parts[2],
		)

	case len(parts) == 2 && parts[1] == "bookings":
		CustomerBookingsHandler(w, r, customer, s)

	case len(parts) == 4 &&
		parts[1] == "bookings" &&
		parts[3] == "cancel":
		CancelBookingHandler(w, r, customer, s, parts[2])

	case len(parts) == 2 && parts[1] == "chats":
		CustomerChatsHandler(w, r, customer, s)

	case len(parts) == 3 && parts[1] == "chats":
		CustomerChatHandler(w, r, customer, s, parts[2])

	default:
		http.NotFound(w, r)
	}
}
