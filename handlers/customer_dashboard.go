package handlers

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

// customerEditView is used by the customer's profile edit page.
type customerEditView struct {
	formView

	ProfileImage string
}

// customerDashboardView is the customer's social home.
type customerDashboardView struct {
	models.Customer
	Feed socialFeedView

	CSRF            string
	BackgroundImage string
}

// customerAccountView carries the summary that used to be folded in at the
// bottom of the customer home. It now has its own page behind the menu.
type customerAccountView struct {
	models.Customer

	CSRF          string
	UpcomingCount int
	SelectedState string
	SelectedArea  string
	Barbers       int
	States        []string
	Upcoming      []bookingRow
	Pending       int
	Conversations int
	CartCount     int
}

// CustomerDashboardHandler is the page a customer lands on after login.
func CustomerDashboardHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method == http.MethodPost {
		createSocialPost(w, r, models.SenderCustomer, customer.ID, customer.FullName, customer.ProfileImage, s)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	backgroundImage, err := currentCustomerLandingImage(r.Context())
	if err != nil {
		serverError(w, "load dashboard image setting", err)
		return
	}
	feed, err := loadSocialFeed(r, models.SenderCustomer, customer.ID, customer.FullName, customer.ProfileImage, s.CSRFToken)
	if err != nil {
		serverError(w, "load social home feed", err)
		return
	}
	customer.PasswordHash = ""
	render(w, "customer/dashboard.html", customerDashboardView{
		Customer:        customer,
		CSRF:            s.CSRFToken,
		BackgroundImage: backgroundImage,
		Feed:            feed,
	})
}

// CustomerAccountHandler is the account summary page that the home menu
// opens. It holds the welcome, search and tallies that used to sit below
// the feed on the home page.
func CustomerAccountHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	now := time.Now()
	filter := storage.BookingFilter{CustomerID: customer.ID, Now: now, LegacyTimezone: BookingTimezone()}
	counts, err := storage.DashboardTotals(r.Context(), filter)
	if err != nil {
		serverError(w, "load customer account totals", err)
		return
	}
	filter.Group = "upcoming"
	preview, err := storage.BookingPage(r.Context(), filter, storage.PageCursor{Limit: 6})
	if err != nil {
		serverError(w, "load upcoming preview", err)
		return
	}
	cartCount, err := storage.CartItemCount(r.Context(), customer.ID)
	if err != nil {
		serverError(w, "load account cart count", err)
		return
	}
	conversations, err := storage.ConversationCountForCustomer(r.Context(), customer.ID)
	if err != nil {
		serverError(w, "load account conversations", err)
		return
	}
	activeBarbers, err := storage.ListActiveBarbersResult(r.Context())
	if err != nil {
		serverError(w, "load account states", err)
		return
	}
	var states []string
	seenStates := map[string]bool{}
	for _, b := range activeBarbers {
		key := strings.ToLower(b.State)
		if b.State != "" && !seenStates[key] {
			states = append(states, b.State)
			seenStates[key] = true
		}
	}
	sort.Strings(states)
	customer.PasswordHash = ""
	render(w, "customer/account.html", customerAccountView{
		Customer:      customer,
		CSRF:          s.CSRFToken,
		UpcomingCount: counts.Upcoming,
		Upcoming:      preview.Items,
		Pending:       counts.Pending,
		Barbers:       counts.Barbers,
		States:        states,
		Conversations: conversations,
		CartCount:     cartCount,
	})
}

// CustomerProfileHandler shows the customer their own details.
func CustomerProfileHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	customer.PasswordHash = ""

	render(
		w,
		"customer/profile.html",
		customerView{
			Customer: customer,
			CSRF:     s.CSRFToken,
		},
	)
}

// EditCustomerProfileHandler lets a customer change their own details.
func EditCustomerProfileHandler(
	w http.ResponseWriter,
	r *http.Request,
	customer models.Customer,
	s sessionData,
) {
	switch r.Method {

	case http.MethodGet:
		renderCustomerForm(
			w,
			s,
			formFromCustomer(customer),
			"",
			customer.ProfileImage,
		)

	case http.MethodPost:
		extendUploadDeadline(w)
		limitRequestBodyFor(w, r, 1)
		if !requireCSRF(w, r, s) {
			return
		}

		form, problem := readCustomerForm(r)

		if problem != "" {
			renderCustomerForm(
				w,
				s,
				form,
				problem,
				customer.ProfileImage,
			)

			return
		}

		oldProfileImage := customer.ProfileImage
		newProfileImage := ""

		uploadedImage, err := customerImages.save(
			r,
			"profile_image",
		)

		if err != nil && !errors.Is(err, errNoUpload) {
			renderCustomerForm(
				w,
				s,
				form,
				err.Error(),
				customer.ProfileImage,
			)

			return
		}

		if err == nil {
			newProfileImage = uploadedImage
			customer.ProfileImage = uploadedImage
		}

		customer.FullName = form.FullName
		customer.PhoneNumber = form.Phone
		customer.Email = form.Email

		err = storage.UpdateCustomerProfile(r.Context(), customer)

		if errors.Is(err, storage.ErrEmailTaken) {
			removeImageIgnoringError(newProfileImage)

			renderCustomerForm(
				w,
				s,
				form,
				"That email belongs to another account.",
				oldProfileImage,
			)

			return
		}

		if err != nil {
			removeImageIgnoringError(newProfileImage)

			serverError(
				w,
				"update customer profile",
				err,
			)

			return
		}

		if newProfileImage != "" &&
			oldProfileImage != "" &&
			oldProfileImage != newProfileImage {
			removeImageIgnoringError(oldProfileImage)
		}

		http.Redirect(
			w,
			r,
			"/customer/profile",
			http.StatusSeeOther,
		)

	default:
		methodNotAllowed(
			w,
			http.MethodGet,
			http.MethodPost,
		)
	}
}

// formFromCustomer pre-fills the edit form with the customer's
// currently stored details.
func formFromCustomer(customer models.Customer) customerForm {
	return customerForm{
		FullName: customer.FullName,
		Phone:    customer.PhoneNumber,
		Email:    customer.Email,
	}
}

// renderCustomerForm shows the customer's profile edit form.
func renderCustomerForm(
	w http.ResponseWriter,
	s sessionData,
	form customerForm,
	problem string,
	profileImage string,
) {
	status := http.StatusOK

	if problem != "" {
		status = http.StatusBadRequest
	}

	renderStatus(
		w,
		status,
		"customer/edit-profile.html",
		customerEditView{
			formView: formView{
				CSRF:   s.CSRFToken,
				Error:  problem,
				Values: form.values(),
			},
			ProfileImage: profileImage,
		},
	)
}
