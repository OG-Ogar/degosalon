# De Go Salon

**De Go Salon — Your complete barber ecosystem**

**Barber Multiverse** is the in-platform barber ecosystem: the discovery space where customers find barbers, explore their work and services, read reviews, and book appointments.

A Go web platform connecting barbers and customers through barber services, appointments, communication, product discovery, carts, and product orders.

## Current Status

**MVP deployed and in active development.**

The repository already contains functional application flows for administrators, barbers, and customers. The README previously described an early product plan and no longer matched the implementation.

The application is deployed on Render and uses Supabase PostgreSQL for persistent application data. Core records therefore survive application restarts.

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

### Location-first barber discovery

Customer selects **Explore Barbers** → chooses a structured location (state, then LGA/local area) → sees registered barbers serving that locality → compares barber profiles, services, pricing, portfolio, availability, ratings and reviews → opens a barber profile → books an appointment.

Barber discovery should use structured location data rather than relying only on free-text addresses. Barber profiles should store a state and LGA/local area, with an optional more specific address. This gives customers consistent locality matching now and leaves room for coordinate/distance-based discovery later.

Each public barber profile should provide enough information for a customer to make a booking decision, including the barber/shop identity, portfolio, services and prices, location, availability, customer reviews/comments, and a clear **Book Appointment** action. Reviews should be tied to genuine customer/booking activity as the review system matures.

**Product hierarchy:** De Go Salon is the public brand and product; Barber Multiverse is the barber ecosystem customers enter through discovery.


### Appointment flow

Customer selects Explore Barbers → selects a location → discovers barbers serving that area → compares profiles and reviews → views a barber → chooses a hairstyle/service → books a date/time → receives booking confirmation → barber manages the booking.

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
degosalon/
├── main.go
├── go.mod
├── handlers/     # HTTP routing, authentication and application flows
├── models/       # Domain models
├── storage/      # PostgreSQL persistence and data access
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
- **Database:** Supabase PostgreSQL
- **Database driver:** pgx v5
- **Hosting:** Render
- **Testing:** Go `testing` and `httptest`

## Configuration

The server uses environment configuration where applicable.

- `PORT` — overrides the default HTTP port (`8080`).
- `DATABASE_URL` — PostgreSQL connection string used by the application. The deployed Render service uses Supabase's IPv4-compatible session pooler.
- Admin credentials are initialized from environment configuration; when no admin password is supplied, the application generates one for that run.
- Booking hours are configurable without changing application code.

## Known MVP Gaps / Production Readiness

De Go Salon is now deployed with persistent PostgreSQL storage, but production hardening and feature verification are still ongoing.

Priority work includes:

1. **End-to-end verification of all user journeys** — validate admin, barber, customer, booking, chat, product, and password-reset flows against the live database.
2. **Durable upload storage** — uploaded images currently need a production-safe persistence strategy rather than relying on local instance storage.
3. **Expanded automated coverage** — especially authorization boundaries and complete booking/order lifecycle scenarios.
4. **Operational readiness** — strengthen logging/monitoring, database backup/recovery procedures, and deployment observability.
5. **Security review** — continue verifying authentication, authorization, secrets, session handling, and database access boundaries.

## Deployment

The live application architecture is:

```text
Browser
   ↓
Render (Go web application)
   ↓
Supabase Session Pooler
   ↓
Supabase PostgreSQL
```

Render builds the application with:

```bash
go mod tidy && go build -tags netgo -ldflags '-s -w' -o app
```

and starts it with:

```bash
./app
```

The Supabase session pooler is used for the deployed database connection because it provides IPv4-compatible connectivity for Render.

## Security Note

Never commit passwords, API keys, personal access tokens, or other credentials to this repository. Any credential accidentally committed should be revoked immediately and removed from the current source tree.

## MVP Definition of Done

For the MVP to be considered deployable for real users:

- Core admin, barber, and customer journeys work end to end.
- Persistent application data survives application restarts through Supabase PostgreSQL.
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
