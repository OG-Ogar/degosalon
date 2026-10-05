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
npm ci && npm run build && go build -tags netgo -ldflags '-s -w' -o app && go build -tags netgo -ldflags '-s -w' -o migrate ./cmd/migrate
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
npm install --no-audit --no-fund
npm run typecheck
npm run build
```

## PostgreSQL integration verification

CI runs Go tests/vet/race checks and a separate PostgreSQL 17 service job. The database suite covers overlapping booking races, guarded lifecycle races, checkout rollback after a later item fails, concurrent checkout/stock competition, order history after a storage reconnect, scoped messages, foreign-key protection, and authentication sessions after real SQL lookup failures.

To run it locally, set `TEST_DATABASE_URL` to a disposable PostgreSQL server URL with a role allowed to create databases, then run:

```sh
go test -race -v ./storage ./handlers ./internal/dbmigrate -run TestPostgres -count=1 -timeout=5m
```

Each test package creates a randomly named `degosalon_test_*` database and applies the baseline and versioned migrations there. Existing tables in the configured database are not modified. Tests close their connections and drop only the database they created. Without `TEST_DATABASE_URL`, the integration tests skip while ordinary tests continue using memory.

The CI service configuration follows [GitHub's PostgreSQL service-container guide](https://docs.github.com/en/actions/tutorials/use-containerized-services/create-postgresql-service-containers). This suite verifies the server-owned PostgreSQL access used by Go; it does not certify production Supabase Data API grants, Storage policies, or deployment settings.


### Database schema upgrades

SQL migrations in `supabase/migrations` are embedded in the Go binaries. Create each file with the pinned CLI, then review and commit its SQL:

```bash
npm exec --yes --package=supabase@2.119.0 -- supabase migration new describe_the_change
```

Apply upgrades using this application's runner with `DATABASE_URL` configured:

```bash
go run ./cmd/migrate
go run .
```

The runner uses a private `degosalon_internal.schema_migrations` ledger, records SHA-256 checksums, locks concurrent migration commands, and commits SQL and history together. Never edit a previously applied migration; add another file. It rejects unknown, reordered, or modified history. This ledger is separate from Supabase CLI's own migration history: use the Go runner consistently to apply this application's files. `supabase/schema.sql` remains the original baseline fixture, not the upgrade command.

App startup only verifies migration history and connectivity; it performs no schema changes. The Render Blueprint builds both binaries and starts with `./migrate && ./app`, so migration failures prevent the new app from starting. This startup sequence works without requiring a paid pre-deploy feature; concurrent starts serialize on the migration lock. Deployments managed outside this Blueprint must run the migration command before starting the new app.

The initial migration creates a fresh schema or adopts the existing baseline without deleting records, and adds the earlier discovery location columns where absent. CI checks fresh/concurrent execution, legacy data preservation, rejected checksum changes, RLS presence, and transaction rollback for an incompatible schema. Take and verify database backups before operational upgrades; production restore drills remain separate acceptance work. Migration file creation follows [Supabase's migration guide](https://supabase.com/docs/guides/deployment/database-migrations).


Checkout forms include a customer-scoped retry key. After a successful order, resubmitting that form returns the original confirmation and never consumes a newly added cart. Failed stock checks leave the cart and key available for retry. The server computes totals from product records; posted amounts are ignored. Keys persist with PostgreSQL orders and survive application reconnects.


Discovery is location-first for visitors and signed-in customers: choose a state to list shops, then optionally narrow by local area. Profile, sign-in, signup, and booking continuations retain selected filters and services. Public portfolio product actions open that barber's catalogue; users can return to all products explicitly.


New appointments preserve their service name, price, duration, and shop contact metadata at booking time. Later service edits affect new bookings; they do not change existing appointment intervals or displayed history. Existing appointments are backfilled from the metadata retained when the snapshot migration runs; earlier changes cannot be reconstructed.


Barbers manage weekly hours, breaks, and time off at `/barber/schedule` (linked from appointments). Calendars support named timezones, minimum notice, and up to 365 days of advance booking. Customers choose a service and date, select an available start time, and submit an appointment request; availability is rechecked on submission. New bookings record the calendar timezone. Schedule changes conflicting with existing future appointments must wait until those appointments are resolved.


Product checkout is a stock reservation for collection, with payment arranged directly with each shop. No online payment is taken. Sellers mark their own items Ready, then Collected, or cancel Pending/Ready items to restore stock exactly once. An order is Pending while any item is pending, Ready when remaining open items are ready, Completed when collected items remain and all items are closed, or Cancelled when every item is cancelled. Original order totals remain historical; cancelled items are not payable. The old `/customer/products/pay` POST remains compatible with previously loaded forms; current forms use `/customer/products/checkout`.

Authentication sessions persist in PostgreSQL when `DATABASE_URL` is configured. Only SHA-256 session token digests are stored in the private server schema. Session cookies remain role-specific, HttpOnly, SameSite=Lax, and valid for 12 hours; use `SECURE_COOKIES=1` over HTTPS. Session operations have a five-second timeout. SQL failures return errors and never fall back to development memory. Expired records are removed in bounded batches during session creation. Password-reset token digests also persist in the private schema for 30 minutes. A new link replaces the previous one; completion atomically writes the password, revokes existing sessions, and consumes the link. Failed writes leave the link usable. Reset pages disable caching/referrer sharing, and raw links are never logged. Delivery configuration remains pending; local demos can use SHOW_RESET_LINKS=1.
