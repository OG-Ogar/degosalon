package handlers

import (
	"net/http"

	"barber-booking/models"
)

// BarberRouter dispatches every /barber/ URL.
//
// Routing is done on whole path segments. The previous version matched
// with strings.HasSuffix, so any path ending in "/edit" reached the
// hairstyle editor regardless of what came before it.
func BarberRouter(w http.ResponseWriter, r *http.Request) {
	defer cleanupMultipart(r)
	parts := pathParts(r.URL.Path)

	if len(parts) == 1 {
		http.Redirect(w, r, "/barber/dashboard", http.StatusSeeOther)

		return
	}

	// The only pages a signed-out barber may reach.
	if len(parts) == 2 && parts[1] == "login" {
		BarberLoginHandler(w, r)

		return
	}

	if len(parts) == 2 && parts[1] == "register" {
		BarberRegisterHandler(w, r)

		return
	}

	if len(parts) == 2 && parts[1] == "forgot-password" {
		BarberForgotPasswordHandler(w, r)

		return
	}

	if len(parts) == 2 && parts[1] == "reset-password" {
		BarberResetPasswordHandler(w, r)

		return
	}

	// Passkey sign-in starts before the visitor has a session; the
	// remembered-device cookie selects the account to challenge.
	if len(parts) == 4 && parts[1] == "passkey" && parts[2] == "login" {
		switch parts[3] {
		case "begin":
			BarberPasskeyLoginBegin(w, r)
		case "finish":
			BarberPasskeyLoginFinish(w, r)
		default:
			journeyError(w, "Page not found.", http.StatusNotFound)
		}

		return
	}

	barber, s, ok := requireBarber(w, r)
	if !ok {
		return
	}

	switch {

	case len(parts) == 2 && parts[1] == "logout":
		BarberLogoutHandler(w, r, s)

	case len(parts) == 2 && parts[1] == "dashboard":
		BarberDashboardHandler(w, r, barber, s)

	case len(parts) == 2 && parts[1] == "profile":
		MyProfileHandler(w, r, barber, s)

	case len(parts) == 3 && parts[1] == "profile" && parts[2] == "edit":
		EditProfileHandler(w, r, barber, s)

	case len(parts) == 2 && parts[1] == "hairstyles":
		ListHairstylesHandler(w, r, barber, s)

	case len(parts) == 3 && parts[1] == "hairstyles" && parts[2] == "new":
		NewHairstyleHandler(w, r, barber, s)

	case len(parts) == 3 && parts[1] == "hairstyles":
		ViewHairstyleHandler(w, r, barber, s, parts[2])

	case len(parts) == 4 && parts[1] == "hairstyles":
		hairstyleAction(w, r, barber, s, parts[2], parts[3])

	case len(parts) == 2 && parts[1] == "products":
		ListProductsHandler(w, r, barber, s)

	case len(parts) == 3 &&
		parts[1] == "products" &&
		parts[2] == "new":
		NewProductHandler(w, r, barber, s)

	case len(parts) == 3 && parts[1] == "products":
		ViewProductHandler(w, r, barber, s, parts[2])

	case len(parts) == 4 && parts[1] == "products" && parts[3] == "edit":
		EditProductHandler(w, r, barber, s, parts[2])

	case len(parts) == 4 && parts[1] == "products" && parts[3] == "update":
		UpdateProductHandler(w, r, barber, s, parts[2])

	case len(parts) == 4 &&
		parts[1] == "products" &&
		parts[3] == "delete":
		DeleteProductHandler(w, r, barber, s, parts[2])

	case len(parts) == 4 && parts[1] == "order-items":
		BarberOrderItemActionHandler(w, r, barber, s, parts[2], parts[3])

	case len(parts) == 2 && parts[1] == "schedule":
		BarberCalendarHandler(w, r, barber, s)

	case len(parts) == 2 && parts[1] == "bookings":
		BarberBookingsHandler(w, r, barber, s)

	case len(parts) == 4 && parts[1] == "bookings":
		BarberBookingActionHandler(w, r, barber, s, parts[2], parts[3])

	case len(parts) == 2 && parts[1] == "chats":
		BarberChatsHandler(w, r, barber, s)

	case len(parts) == 3 && parts[1] == "chats":
		BarberChatHandler(w, r, barber, s, parts[2])

	case len(parts) == 3 && parts[1] == "settings" && parts[2] == "preferences":
		BarberPreferencesHandler(w, r, barber, s)

	case len(parts) == 4 && parts[1] == "passkey" && parts[2] == "register" && parts[3] == "begin":
		BarberPasskeyRegisterBegin(w, r, barber, s)

	case len(parts) == 4 && parts[1] == "passkey" && parts[2] == "register" && parts[3] == "finish":
		BarberPasskeyRegisterFinish(w, r, barber, s)

	default:
		journeyError(w, "Page not found.", http.StatusNotFound)
	}
}

// hairstyleAction handles /barber/hairstyles/{id}/{action}.
func hairstyleAction(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
	idSegment string,
	action string,
) {
	switch action {

	case "edit":
		EditHairstyleHandler(w, r, barber, s, idSegment)

	case "update":
		UpdateHairstyleHandler(w, r, barber, s, idSegment)

	case "delete":
		if r.Method == http.MethodPost {
			DeleteHairstyleHandler(w, r, barber, s, idSegment)
		} else {
			ConfirmDeleteHairstyleHandler(w, r, barber, s, idSegment)
		}

	default:
		journeyError(w, "Page not found.", http.StatusNotFound)
	}
}
