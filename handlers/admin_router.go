package handlers

import (
	"net/http"
)

// AdminRouter dispatches every /admin/ URL.
//
// Only the login page is public. Everything below it goes through
// requireAdmin first, which is the fix for the admin area previously
// having no access control at all.
func AdminRouter(w http.ResponseWriter, r *http.Request) {
	defer cleanupMultipart(r)
	parts := pathParts(r.URL.Path)

	if len(parts) == 1 {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)

		return
	}

	if len(parts) == 2 && parts[1] == "login" {
		AdminLoginHandler(w, r)

		return
	}

	s, ok := requireAdmin(w, r)
	if !ok {
		return
	}

	withAdminAudit(w, r, func(w http.ResponseWriter, r *http.Request) { dispatchAdmin(w, r, s, parts) })
}

func dispatchAdmin(w http.ResponseWriter, r *http.Request, s sessionData, parts []string) {
	switch {

	case len(parts) == 2 && parts[1] == "logout":
		AdminLogoutHandler(w, r, s)

	case len(parts) == 2 && parts[1] == "dashboard":
		AdminDashboardHandler(w, r, s)

	case len(parts) == 2 && parts[1] == "landing-image":
		AdminLandingImageHandler(w, r, s)

	case len(parts) == 2 && parts[1] == "customer-landing-image":
		AdminCustomerLandingImageHandler(w, r, s)

	case len(parts) == 2 && parts[1] == "bookings":
		AdminBookingsHandler(w, r, s)

	case len(parts) == 2 && parts[1] == "customers":
		AdminCustomersHandler(w, r, s)

	case len(parts) == 2 && parts[1] == "barbers":
		ListBarbersHandler(w, r, s)

	case len(parts) == 3 && parts[1] == "barbers" && parts[2] == "new":
		NewBarberHandler(w, r, s)

	case len(parts) == 3 && parts[1] == "barbers":
		BarberProfileHandler(w, r, s, parts[2])

	case len(parts) == 4 && parts[1] == "barbers":
		adminBarberAction(w, r, s, parts[2], parts[3])

	default:
		journeyError(w, "Page not found.", http.StatusNotFound)
	}
}

// adminBarberAction handles /admin/barbers/{id}/{action}.
func adminBarberAction(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
	idSegment string,
	action string,
) {
	switch action {

	case "edit":
		EditBarberHandler(w, r, s, idSegment)

	case "update":
		UpdateBarberHandler(w, r, s, idSegment)

	case "delete":
		if r.Method == http.MethodPost {
			DeleteBarberHandler(w, r, s, idSegment)
		} else {
			ConfirmDeleteBarberHandler(w, r, s, idSegment)
		}

	case "suspend":
		SetBarberStatusHandler(w, r, s, idSegment, false)

	case "activate":
		SetBarberStatusHandler(w, r, s, idSegment, true)

	default:
		journeyError(w, "Page not found.", http.StatusNotFound)
	}
}
