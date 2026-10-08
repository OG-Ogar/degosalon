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
- Archive barber accounts while retaining customer history and media; open appointments and uncollected orders must be resolved first. Archived accounts cannot sign in or reactivate.
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

Authentication sessions persist in PostgreSQL when `DATABASE_URL` is configured. Only SHA-256 session token digests are stored in the private server schema. Session cookies remain role-specific, HttpOnly, SameSite=Lax, and valid for 12 hours; use `SECURE_COOKIES=1` over HTTPS. Session operations have a five-second timeout. SQL failures return errors and never fall back to development memory. Expired records are removed in bounded batches during session creation. Password-reset token digests also persist in the private schema for 30 minutes. A new link replaces the previous one; completion atomically writes the password, revokes existing sessions, and consumes the link. Failed writes leave the link usable. Reset pages disable caching/referrer sharing, and raw links are never logged. Configured recovery requests enqueue encrypted mail jobs in the same transaction as token issuance. Local demos can use SHOW_RESET_LINKS=1.


Customer/barber login and customer registration recheck the verified password and current barber status inside session creation. PostgreSQL holds the account row lock until the session commits. A concurrent password reset either revokes the already-created session or causes the stale login to fail without a cookie. Registration keeps committed profile media if automatic login fails.

Production mode is selected by `APP_ENV=production` or Render's `RENDER=true`. Startup requires `DATABASE_URL`, a nonempty `ADMIN_USERNAME`, and an `ADMIN_PASSWORD` of 12–72 bytes; it never generates a production administrator password or uses memory storage. Production cookies are always Secure. Explicit `SECURE_COOKIES=0` or `SHOW_RESET_LINKS=1` stops startup, and reset links remain hidden even when the handler is invoked directly. The Blueprint sets production mode and secure cookies. Local development without these production markers may use memory storage, generated administrator credentials, and explicit demo reset links. Configuration errors name settings without printing secret values.

Authentication uses shared fixed-window throttles before expensive verification or token issuance. Login allows 10 attempts per account/role and 100 per client address per 15 minutes; forgot-password allows 3 per account/role and 30 per address per 30 minutes; reset submissions allow 10 per token/role and 60 per address per 30 minutes; registration allows 5 per email/role and 20 per address per hour. Responses include HTTP 429 and Retry-After. Counters persist in the private PostgreSQL schema with hashed identifiers and bounded expiry cleanup; SQL failures never bypass limits. Development memory is capped at 10,000 active counters.

Client addresses use the connecting peer by default. Behind a proxy, set `TRUSTED_PROXY_CIDRS` only to the actual trusted proxy networks (up to 32 specific CIDRs). Forwarded addresses are resolved from right to left, stopping at the first untrusted hop. Untrusted peers cannot spoof their address with forwarded headers. Without this setting, clients sharing a proxy/NAT also share the peer budget. Live proxy configuration must be verified for the deployment; no network range is implicitly trusted.


Reset email delivery uses durable encrypted jobs and a bounded SMTP transport. It accepts SMTP_HOST, SMTP_PORT (default 587), SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM (a bare address), SMTP_TLS_MODE (starttls or implicit), and an HTTPS SITE_URL without a path, query, or fragment. On Render, RENDER_EXTERNAL_URL supplies the canonical origin when SITE_URL is absent. TLS certificate verification is mandatory; delivery has a ten-second timeout. Set RESET_MAIL_KEY to a persistent random secret of at least 32 characters, shared by every instance; keep it stable while outstanding links remain valid. The server derives an AES-256-GCM key and stores recipient/token payloads only as authenticated ciphertext in a private RLS-enabled table. The Blueprint generates this secret for new Blueprint-managed deployments; existing services need their own persistent value. No credentials or reset links are logged.

A background worker leases jobs across instances, retries relay failures with exponential backoff (at most eight claims), and removes jobs after relay acceptance. A crash after acceptance may send the same single-use link again; SMTP does not provide exactly-once delivery. Token replacement, consumption, and expiry remove the associated queued ciphertext. Accepted requests show the same queued-delivery response for registered and unknown addresses; unknown addresses create no message. Missing or invalid SMTP/site/key configuration returns a generic 503 with a retry delay and preserves previous links. Relay acceptance is not an inbox-delivery guarantee: verify the provider's sender/domain and deliverability. The app never derives recovery origins from request headers. The Blueprint supplies SMTP secret placeholders and uses Render's HTTPS external URL by default; set SITE_URL for a custom canonical domain. Actual provider credentials, live delivery, and provider bounce handling are deployment acceptance requirements.


Mutation forms are limited to 64 KiB before CSRF parsing. Multipart upload routes retain their file/total byte allowances, use a 1 MiB multipart file-memory allowance, and clean temporary files after dispatch. JPEG/PNG/GIF/WebP headers must match their filename type and fit within 16 million pixels and 10,000 pixels per side. JPEG/PNG decoding happens only after those checks; corrupt JPEG/PNG inputs are rejected. GIF/WebP encoded content is preserved after bounded header validation. Large phone photos may need resizing before upload even when their compressed file is below the byte limit.


Landing and customer-dashboard image references persist in the private PostgreSQL site-media settings table when DATABASE_URL is configured. Reads and replacements use bounded request contexts; database failures remain errors. Concurrent administrator replacements return the exact old reference after commit. Definite write failures preserve the old reference and discard the unused new local image. Uncertain commit acknowledgments retain the new object for reconciliation rather than risk deleting a referenced image. Startup imports available legacy uploads/site/landing-image.txt and customer-landing-image.txt pointers only when the corresponding setting is absent, and never overwrites administrator edits. New requests do not depend on those text files. Development memory resets with the process. Startup imports references only. The opt-in legacy migration command below copies object bytes while retaining originals.

### Cloud media upload transport

Set `SUPABASE_URL` to the HTTPS project origin and `SUPABASE_SECRET_KEY` to a server-only secret key. The legacy `SUPABASE_SERVICE_ROLE_KEY` remains a fallback when the preferred key is absent. Never expose either key to frontend code. Provision the public `degosalon-media` bucket before using uploads; it must allow the supported image/video MIME types and files up to 50 MiB. The Blueprint requests the preferred settings and runs the bucket provisioning command before starting the application.

Images and videos stream from validated staging files to random object names with overwrite disabled. Requests inherit cancellation and have a 45-second total deadline and 15-second response-header timeout. Redirects are refused and provider error details are hidden. Cloud staging files are removed on success and failure. Development without a cloud origin keeps local files; production requires cloud configuration for uploads rather than accepting ephemeral local assets. Existing local references are not automatically migrated. Durable remote cleanup is described below. Legacy local-byte migration uses the opt-in command below; live provider verification remains separate acceptance work.

### Durable cloud-media reconciliation

Cloud uploads first persist an object intent in the private PostgreSQL ledger. Replacements and rollbacks schedule cleanup instead of immediately deleting provider objects. The worker waits one hour before the first check, preserves references across account photos, portfolios, products, and site settings, and freezes an unreferenced URL before deleting it. Database triggers and memory-store guards prevent a stale writer from restoring a retired URL. SQL failures retain the intent and never switch to memory in production.

Deletes use the Storage API, require the exact configured project origin/application bucket, and accept only supported application-generated or legacy timestamp filenames. Multiple instances share bounded claims; expired leases and failed acknowledgments retry safely. Provider failures use backoff. Successful deletes retain tombstones and reconcile daily to cover late uploads after uncertain provider responses. An active reference is checked daily, or within one hour after explicit retirement. Cleanup errors are logged without URLs or provider bodies. Existing referenced cloud URLs are seeded by the migration, then validated against the configured origin before any network delete.

The worker requires the same persistent database and server-only cloud settings as uploads. Changing projects does not authorize deletion in the old project; such assets require a deliberate migration. Historical local files not present at deployment cannot be reconstructed. Migration of actual historical assets, live provider acceptance, and graceful shutdown/readiness remain pending acceptance work.

### Media bucket provisioning

Build the command with `go build -o media ./cmd/media`. Run `./media` to verify the configured bucket, or `./media -create-bucket` to create it only if missing, then verify the resulting configuration. Set the server-only cloud environment variables before running it. The Render Blueprint builds this binary and runs `./migrate && ./media -create-bucket && ./app`; missing credentials, an unreachable provider, or an incompatible bucket prevent that deployment from starting. Existing private buckets remain private and existing limits are never rewritten.

New buckets are public `STANDARD` buckets named `degosalon-media`, limited to 50 MiB, accepting JPEG/PNG/GIF/WebP and MP4/WebM/QuickTime. Existing buckets must be public and allow all these types and sizes; unrestricted settings are compatible with the application's own validation. The provider's global upload limit may still be lower and must be checked during live acceptance. Creation is followed by readback, including after a concurrent creator or uncertain acknowledgment. Requests share the bounded, redirect-refusing server transport and return safe errors. Bucket provisioning alone does not copy local assets; use the migration option below. Live provider credentials are unavailable in the audit workspace, so real bucket/upload/delete acceptance remains unverified.

### Migrating legacy uploads

With the current database schema and server-only cloud configuration, run `./media -migrate-legacy -uploads-dir /path/to/original/uploads`. Add `-create-bucket` only if provisioning is also needed. The root contains the original `barbers`, `customers`, `hairstyles`, `hairstyle-videos`, `products`, and `site` subdirectories. The command imports the two known legacy site pointer files only when their durable settings are absent, then processes distinct local references in bounded pages.

Available regular files inside the resolved root are copied to private staging files, checked against the current size/content limits, and uploaded under new random names after a durable intent is recorded. Image normalization applies to the staged copy. Each source URI is conditionally replaced across all matching reference columns in one bounded database transaction; newer edits, passwords, account status, stock and unrelated metadata are preserved. The original files and text pointers are retained. Migrated local URI names receive permanent retirement records so stale writes cannot restore them; those records never enter cloud deletion.

The command prints counts and returns an error if any file failed, a database/provider operation is uncertain, or local references remain. Missing or invalid files do not prevent independent valid files from migrating. Rerunning skips references already moved; unused uploaded copies remain covered by reconciliation. Interrupt/termination cancels processing. Restore missing originals from a verified backup and rerun; the command cannot reconstruct bytes lost on earlier ephemeral deployments. Legacy migration is an explicit operation, not an automatic startup job. Real asset inventory, provider delivery and production migration acceptance remain unverified until the existing deployment and backups are available.

Database operations use caller cancellation with a five-second default deadline, including pool acquisition and transaction commits. Connections request PostgreSQL cancellation with a 500 ms socket-deadline fallback; a lost commit acknowledgment remains uncertain and requires the existing idempotency/reconciliation contracts. Startup database verification has a ten-second overall deadline. Pool defaults are 12 open connections, 6 idle connections, a 30-minute lifetime and a five-minute idle lifetime. Override with DB_MAX_OPEN_CONNS (1–100), DB_MAX_IDLE_CONNS (0–open limit), DB_CONN_MAX_LIFETIME and DB_CONN_MAX_IDLE_TIME (Go durations from 1m through 24h, idle lifetime no greater than connection lifetime). Size the pool against the provider's limits across all application instances. Real session-pooler cancellation and production capacity require deployment verification.

Product order items retain seller name, shop name/address/phone, and product category/description at checkout, alongside their existing purchase name/price/quantity/subtotal. Confirmations use those snapshots without fetching the current catalogue or shop profile. Photos are not part of the historical record. PostgreSQL prevents updates to original order-item details while permitting fulfillment state changes. The upgrade backfills newly added details from currently retained records and cannot reconstruct earlier edits. Account/catalogue archival and retention policy remain separate work.

Catalogue removal archives products and hairstyles: rows, media and existing booking/order references remain retained, while active catalogues exclude them. Archived entries cannot be reopened or edited through the catalogue APIs, and new bookings/cart additions/checkout reject them. Existing appointments can complete; outstanding product items can still fulfill or cancel, restoring stock to the retained product exactly once. Existing carts expose archived entries with removal controls. No automatic purge or media deletion occurs on archival. Account archival and any future approved retention/purge policy remain separate operations.


Public help and policy destinations are `/help`, `/support`, `/privacy` and `/terms`, linked by a shared footer on all template-rendered page families. Support publishes the owner-approved numbers 08128162561 and 09164226427 with international telephone links. Privacy and terms remain clearly labeled placeholders by owner request; no policy or agreement is invented. Help includes current booking, messaging, product collection and recovery guidance. Help and legal placeholders send `noindex, follow`; approved support contacts are indexable.

Public home, discovery, shop profiles and content pages share description/Open Graph/Twitter metadata. Absolute canonical/share URLs use the configured HTTPS `SITE_URL`, or `RENDER_EXTERNAL_URL` when absent, never request/forwarded Host headers. Configure `SITE_URL` for the preferred custom domain. Invalid or missing origins omit absolute URLs; discovery filters and account/session parameters are excluded from canonical URLs. Verify the deployed domain and generated metadata during deployment acceptance. No social share image is invented.
