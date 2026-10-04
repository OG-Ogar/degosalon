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
- **Frontend:** Server-rendered HTML/CSS with TypeScript-enhanced browser interactions
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
npm ci && npm run build && go mod tidy && go build -tags netgo -ldflags '-s -w' -o app
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

## Post-UI optimization backlog

During the full De Go Salon UI optimization pass, improvements that require product behavior or deeper interaction changes are recorded here rather than being mixed into the visual-only branch.

- **Shared authenticated navigation:** replace duplicated customer/barber page headers with reusable role-aware navigation so active states, mobile behavior, labels, and logout placement stay consistent.
- **Customer messages and barber messages:** move legacy page-specific inline CSS into the shared design system, then improve conversation states and the mobile chat experience.
- **Authentication usability:** add a password visibility control and consistent client-side field feedback across customer and barber sign-in/registration/reset flows.
- **Customer dashboard discovery:** make the dashboard's Find a Barber entry preserve and reuse the location-first discovery journey introduced on the public experience.
- **Booking management:** improve action/status feedback after booking, cancellation, acceptance, rejection, and completion instead of relying only on page refresh context.
- **Barber workspace navigation:** consolidate the duplicated numbered navigation and make the information architecture consistent across Dashboard, Hairstyles, Products, Bookings, Messages, and Profile.
- **Responsive data views:** review admin and barber table-heavy screens for card/list alternatives on narrow phones rather than horizontal table compression.
- **Empty states:** make empty bookings, messages, products, hairstyles, and search results consistently provide a useful next action.
- **Design-system cleanup:** remove remaining inline styles and consolidate repeated button, form, card, status, header, and responsive rules into shared components after the visual migration settles.

This list should be reviewed after the UI optimization branch is merged. Items are implementation follow-ups, not claims that the underlying feature is currently broken.


## TypeScript frontend strategy

De Go Salon uses a progressive-enhancement architecture rather than a separate single-page frontend. Go continues to own routing, authentication, authorization, server-rendered HTML, forms, and Supabase access. TypeScript source lives under `frontend/` and compiles to browser JavaScript under `static/`, which the existing Go server serves.

This keeps the current Render deployment as one web service while giving interactive surfaces typed client-side code. High-interaction areas such as location discovery, booking selection, chat, cart behavior, uploads, and role navigation can migrate incrementally without rewriting stable server-rendered pages.

Frontend checks:

```bash
npm ci
npm run typecheck
npm run build
```
