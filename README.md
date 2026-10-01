# Barber Booking

A Go web application for managing barber services, appointments, customer communication, and product orders.

## Current Status

**MVP in active development.**

The repository already contains functional application flows for administrators, barbers, and customers. The README previously described an early product plan and no longer matched the implementation.

> Important MVP limitation: application data is currently stored in memory and is lost whenever the server restarts. Persistent database storage is therefore a key requirement before production use.

## Users

### Admin
The admin manages the platform.

Current capabilities:
- Sign in and sign out
- View the admin dashboard
- Create and view barber accounts
- Edit barber information
- Delete barber accounts
- Suspend and reactivate barbers
- View bookings
- View customers
- Manage landing-page images

### Barber
Barbers have authenticated accounts for managing their business.

Current capabilities:
- Register, sign in, sign out, and reset passwords
- View a barber dashboard
- View and edit their profile
- Create, view, edit, and delete hairstyles
- Upload images
- Set hairstyle information and pricing
- View bookings and perform booking actions
- View conversations and chat with customers
- Create, view, and delete products

### Customer
The current implementation uses customer accounts, which is a change from the original plan where customers would not need accounts.

Current capabilities:
- Register, sign in, sign out, and reset passwords
- View a customer dashboard
- View and edit their profile
- Browse barbers and barber profiles
- Select a hairstyle and create a booking
- View booking confirmation and booking history
- Cancel bookings
- Chat with barbers
- Browse products
- Add products to a cart
- Place product orders and view order confirmation

## Core MVP Flows

### Appointment flow

Customer registers/signs in → browses barbers → views a barber → chooses a hairstyle → books a date/time → receives booking confirmation → barber manages the booking.

### Barber service flow

Barber registers/signs in → manages profile → creates hairstyle catalogue → manages prices/images → receives bookings → communicates with customers.

### Product flow

Barber creates products → customer browses products → adds products to cart → places order → receives order confirmation.

### Administration flow

Admin signs in → manages barbers → reviews customers/bookings → suspends or activates barber accounts → manages landing imagery.

## Implemented Application Areas

- Role-based routing for admin, barber, and customer areas
- Session-based authentication
- Password hashing/security utilities
- Password-reset flow
- CSRF protection in authenticated forms
- Admin access control
- Barber and customer profiles
- Hairstyle catalogue management
- Appointment booking and cancellation
- Barber booking actions
- Barber/customer chat
- Product catalogue
- Shopping cart and product ordering
- Image uploads and image serving
- Configurable booking hours
- HTTP server timeouts
- HTML templates and responsive styling
- Automated tests covering storage, validation, uploads/images, password reset, and application flows

## MVP Architecture

```text
barber-booking/
├── main.go
├── go.mod
├── handlers/     # HTTP routing, authentication and application flows
├── models/       # Domain models
├── storage/      # In-memory application data store
├── session/      # Session management
├── security/     # Password/security utilities
├── reset/        # Password-reset token handling
├── templates/    # Admin, barber and customer HTML templates
├── static/       # CSS and JavaScript
├── uploads/      # Uploaded application images
└── README.md
```

## Technology

- **Backend:** Go 1.22.2
- **HTTP:** Go standard library `net/http`
- **Frontend:** Server-rendered HTML, CSS and JavaScript
- **Storage:** In-memory Go data structures
- **Testing:** Go `testing` and `httptest`

## Configuration

The server uses environment configuration where applicable.

- `PORT` — overrides the default HTTP port (`8080`).
- Admin credentials are initialized from environment configuration; when no admin password is supplied, the application generates one for that run.
- Booking hours are configurable without changing application code.

## Known MVP Gaps / Production Readiness

The existing feature set is substantial, but the application should not yet be treated as production-ready.

Priority work includes:

1. **Persistent database storage** — current records live only in memory and disappear after a restart.
2. **Production configuration and secret management** — credentials must remain outside source control.
3. **End-to-end verification of all user journeys** — validate admin, barber, customer, booking, chat, product, and password-reset flows together.
4. **Deployment and persistence strategy for uploads** — local uploads require durable production storage.
5. **Expanded automated coverage** — especially authorization boundaries and complete booking/order lifecycle scenarios.
6. **Operational readiness** — deployment configuration, logging/monitoring, backups, and recovery procedures.

## Security Note

Never commit passwords, API keys, personal access tokens, or other credentials to this repository. Any credential accidentally committed should be revoked immediately and removed from the current source tree.

## MVP Definition of Done

For the MVP to be considered deployable for real users:

- Core admin, barber, and customer journeys work end to end.
- Data survives application restarts.
- Authentication and authorization boundaries are verified.
- Booking conflicts and ownership rules are enforced.
- Uploaded images persist safely.
- Secrets are supplied through environment/deployment configuration.
- Critical flows have automated tests.
- The application can be deployed reproducibly and monitored for failures.

## Running Locally

With Go installed:

```bash
go run .
```

By default the server listens on:

```text
http://localhost:8080
```

Run the test suite with:

```bash
go test ./...
```
