package handlers

import (
	"context"
	"errors"
	"net/http"
	"os"

	"barber-booking/internal/runtimeconfig"
	"barber-booking/reset"
	"barber-booking/security"
	"barber-booking/session"
	"barber-booking/storage"
)

// resetFlow is everything the forgotten-password pages need to know
// about one role. Customers and barbers differ only in these values, so
// the two flows share the handlers below instead of being written twice.
type resetFlow struct {
	Role   session.Role
	Prefix string

	ForgotPage string
	ResetPage  string

	// FindID reports ErrNotFound for unknown accounts and preserves lookup failures.
	FindID func(ctx context.Context, email string) (int, error)

	// SetPassword stores an already hashed password.
	SetPassword func(ctx context.Context, id int, hash string) error
}

var customerResetFlow = resetFlow{
	Role:       session.RoleCustomer,
	Prefix:     "/customer",
	ForgotPage: "customer/forgot-password.html",
	ResetPage:  "customer/reset-password.html",

	FindID: func(ctx context.Context, email string) (int, error) {
		customer, err := storage.FindCustomerByEmailResult(ctx, email)
		return customer.ID, err
	},

	SetPassword: storage.SetCustomerPasswordContext,
}

var barberResetFlow = resetFlow{
	Role:       session.RoleBarber,
	Prefix:     "/barber",
	ForgotPage: "barber/forgot-password.html",
	ResetPage:  "barber/reset-password.html",

	// A suspended barber may still reset their password. Refusing here
	// would tell an outsider which accounts are suspended, and the login
	// itself already turns them away.
	FindID: func(ctx context.Context, email string) (int, error) {
		barber, err := storage.FindBarberByEmailResult(ctx, email)
		return barber.ID, err
	},

	SetPassword: storage.SetBarberPasswordContext,
}

// resetView backs both forgotten-password pages.
type resetView struct {
	Error string
	Email string
	Token string

	// Sent switches the request page from the form to its confirmation.
	Sent bool

	// Valid is true once a reset link has been checked, and is what the
	// reset page uses to decide whether to show the form at all.
	Valid bool

	// Link is the reset link itself, filled in only when
	// SHOW_RESET_LINKS=1 for a local demo.
	Link string

	MinLength int
}

// CustomerForgotPasswordHandler asks a customer for their email address
// and issues a reset link for it.
func CustomerForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	forgotPassword(w, r, customerResetFlow)
}

// CustomerResetPasswordHandler sets a new customer password from a link.
func CustomerResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	resetPassword(w, r, customerResetFlow)
}

// BarberForgotPasswordHandler is the barber side of the same request.
func BarberForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	forgotPassword(w, r, barberResetFlow)
}

// BarberResetPasswordHandler sets a new barber password from a link.
func BarberResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	resetPassword(w, r, barberResetFlow)
}

// forgotPassword takes an email address and, when it belongs to an
// account, issues a reset link for it.
//
// The confirmation page says the same thing either way. Reporting that
// an address is not registered would turn this form into a way of
// finding out who has an account.
func forgotPassword(w http.ResponseWriter, r *http.Request, flow resetFlow) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	switch r.Method {

	case http.MethodGet:
		render(w, flow.ForgotPage, resetView{})

	case http.MethodPost:
		if !beginAuthAttempt(w, r, "forgot") {
			return
		}
		email := normalisedEmail(r.PostFormValue("email"))
		if !authAccountAttempt(w, r, string(flow.Role), "forgot", email) {
			return
		}

		if email == "" {
			renderStatus(w, http.StatusBadRequest, flow.ForgotPage, resetView{
				Error: "Enter the email address you registered with.",
			})

			return
		}

		view := resetView{Email: email, Sent: true}

		id, err := flow.FindID(r.Context(), email)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			serverError(w, "look up password reset account", err)
			return
		}
		if err == nil {
			token, err := reset.NewContext(r.Context(), flow.Role, id)
			if err != nil {
				serverError(w, "issue reset token", err)

				return
			}

			path := flow.Prefix + "/reset-password?token=" + token

			if showResetLinks() {
				view.Link = path
			}
		}

		render(w, flow.ForgotPage, view)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// resetPassword checks a reset link and swaps the account's password for
// the one typed on the form.
func resetPassword(w http.ResponseWriter, r *http.Request, flow resetFlow) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	switch r.Method {

	case http.MethodGet:
		token := r.URL.Query().Get("token")

		if _, err := reset.LookupContext(r.Context(), flow.Role, token); err != nil {
			if !errors.Is(err, reset.ErrInvalidToken) {
				serverError(w, "look up reset token", err)
				return
			}
			expiredLink(w, flow)

			return
		}

		render(w, flow.ResetPage, resetView{
			Token:     token,
			Valid:     true,
			MinLength: minPasswordLength,
		})

	case http.MethodPost:
		if !beginAuthAttempt(w, r, "reset") {
			return
		}
		token := r.PostFormValue("token")
		if !authAccountAttempt(w, r, string(flow.Role), "reset", token) {
			return
		}
		password := r.PostFormValue("password")

		// The link is checked before the password so that a link which
		// went stale in an open tab says so, rather than complaining
		// about the password and then failing anyway.
		if _, err := reset.LookupContext(r.Context(), flow.Role, token); err != nil {
			if !errors.Is(err, reset.ErrInvalidToken) {
				serverError(w, "look up reset token", err)
				return
			}
			expiredLink(w, flow)

			return
		}

		problem := ""

		switch {
		case len(password) > 72:
			problem = "Use a password of at most 72 bytes."
		case len(password) < minPasswordLength:
			problem = passwordTooShortMessage
		case password != r.PostFormValue("confirm_password"):
			problem = "The two passwords do not match."
		}

		if problem != "" {
			renderStatus(w, http.StatusBadRequest, flow.ResetPage, resetView{
				Error:     problem,
				Token:     token,
				Valid:     true,
				MinLength: minPasswordLength,
			})

			return
		}

		hash, err := security.HashPassword(password)
		if err != nil {
			serverError(w, "hash reset password", err)

			return
		}

		_, err = reset.CompleteContext(r.Context(), flow.Role, token, hash, flow.SetPassword)
		if errors.Is(err, reset.ErrInvalidToken) {
			expiredLink(w, flow)
			return
		}
		if err != nil {
			serverError(w, "complete password reset", err)
			return
		}

		http.Redirect(w, r, flow.Prefix+"/login?reset=1", http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// expiredLink is the answer to a token that has run out, has already
// been used, or was never issued. All three get the same page, because
// which one it was is not the visitor's business to learn.
func expiredLink(w http.ResponseWriter, flow resetFlow) {
	renderStatus(w, http.StatusBadRequest, flow.ResetPage, resetView{
		Error: "That reset link has expired or has already been used. " +
			"Please request a new one.",
	})
}

// showResetLinks reports whether reset links should also be shown in the
// browser.
//
// It is off unless SHOW_RESET_LINKS=1, because a link on the page lets
// anyone who knows an email address take over that account. Turn it on
// for a local demo and nowhere else.
func showResetLinks() bool {
	return !runtimeconfig.Production() && os.Getenv("SHOW_RESET_LINKS") == "1"
}
