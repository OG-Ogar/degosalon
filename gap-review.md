# Gap reconciliation — 6 October 2026

This review supersedes the implementation status in the original `gap.md`; its design goals remain useful. Baseline: freshly fetched `origin/main`, `6377955019d4fe699885c95a156db5493a25332e` (PR #81). Local `main` is `8087ac0`, 99 commits behind, with uncommitted booking changes and an untracked original gap report. Review used `/tmp/degosalon-gap-review`; application files in the working checkout were preserved. Do not implement against the old checkout or interpret the original findings as current defects.

Scope: reconciled all 26 frontend and 24 backend IDs across routes/handlers, domain models, storage, templates/styles/scripts, authentication/recovery, media, migrations, startup/deployment and automated verification. This is a source and local-test audit, not production or browser certification. “Implemented” means the original defect has a source repair and relevant coverage; it does not assert live deployment acceptance. “Partial” means concrete requirements remain. “Open” means the reported behavior still exists. Product extensions require a decision rather than automatic implementation.

## Decision before implementation

**Subsequent implementation:** BE-15's administrator barber-deletion defect is repaired by persistent, terminal account archival with unfinished-work guards, retained history/media and atomic PostgreSQL session revocation. See the follow-up in `gap.md`; the matrix below records the PR #81 audit baseline.

The original P0 booking, checkout, cart-stock and request-read failure mechanisms have substantial merged repairs. Do not repeat them. The most urgent confirmed remaining backend defect is persisted account deletion (BE-15); the most immediate frontend defects are image-click interception, the inactive booking summary, mobile table labels and raw error pages. Shared navigation/forms should follow these functional repairs. Production acceptance for existing database, mail and media work remains necessary.

## Backend status matrix

| ID | Status on latest main | Evidence and remaining work |
| --- | --- | --- |
| BE-01 | Implemented | `handlers/booking.go`, `handlers/validate.go`, `storage/calendar.go`: submission validation, normalized time, elapsed/interval checks. Existing handler/calendar coverage; no repeat repair. |
| BE-02 | Implemented | `storage/calendar.go`, `models/calendar.go`, `handlers/barber_calendar.go`, `templates/barber/schedule.html`, customer detail: per-shop timezone/week hours, breaks/time off, lead time/horizon, server-rendered available slots and atomic overlap checks. Browser acceptance remains. |
| BE-03 | Implemented | `storage/booking_storage.go`, `storage/booking_lifecycle_test.go`, PostgreSQL integrity tests: atomic creation and guarded owner/lifecycle transitions. |
| BE-04 | Implemented | `storage/product_order_store.go`, `storage/checkout_replay_test.go`: transactional checkout and customer-scoped durable retry-key replay, fault/concurrency coverage. |
| BE-05 | Implemented | `storage/cart_store.go`, `handlers/customers_cart.go`, customer cart and cart-edit tests: stock consumed at checkout; owned quantity edits/removal. No abandoned reservation expiry is needed for this chosen policy. |
| BE-06 | Implemented | `handlers/customer_dashboard.go`, `storage/customer_order_history_test.go`, handler history tests: owned DB-backed orders populate dashboard; reconnect test exists. |
| BE-07 | Implemented via collection policy | `handlers/barber_order_fulfillment.go`, `storage/order_fulfillment.go`, cart/confirmation: explicit collection checkout and per-seller fulfillment. Online payment provider is optional product work, not an unrectified promise of payment. |
| BE-08 | Implemented core behavior | Cart/checkout revalidate active seller, availability and stock; checkout domain errors render cart at 409 (`handlers/customer_product_order.go`). Broader error-surface work belongs to FE-19. |
| BE-09 | Partial | Request reads now propagate errors, including auth, commerce, discovery, booking, reviews and messages. Legacy error-swallowing helpers still exist; the live admin deletion path still uses a memory-only mutation. Do not describe ordinary request lists as unfixed. Audit remaining mutation/helper callers and verify production outage behavior. |
| BE-10 | Partial | `storage/database_bounds.go`: five-second operations, configured pool/lifetime, pgx cancellation; bounded media and mail transports. Some storage APIs still derive contexts from `context.Background()` rather than the HTTP request; complete cancellation propagation for reachable mutations. |
| BE-11 | Implemented; deployment acceptance pending | Durable role sessions/reset hashes, atomic reset/password/session changes, encrypted leased mail jobs and bounded TLS SMTP (`session/`, `reset/`, `internal/resetmail/`). Verify real inbox delivery, sender setup and shared deployment keys. |
| BE-12 | Implemented; deployment acceptance pending | `internal/runtimeconfig/`, `handlers/auth_throttle.go`, private DB throttle/session state and Render secure-cookie settings. Production rejects volatile storage/unsafe reset links/weak or missing admin setup. Verify live cookie/proxy/throttle configuration. |
| BE-13 | Implemented tooling; deployment acceptance pending | `internal/mediastore/`, `handlers/site_media.go`, media ledger/worker, `cmd/media`, migrations and Render startup: durable settings/videos/images, owned cleanup, bucket verification, atomic legacy migration. Verify actual provider/bucket configuration and run legacy byte migration where required; code cannot recover already lost files. |
| BE-14 | Implemented | `handlers/form_limits.go`, upload-bounds tests: bounded parsing before CSRF/form use, multipart cleanup and image dimensions checked before full decode. |
| BE-15 | Partial — P1 first | Booking/order snapshots and catalogue archives now preserve history. Account deletion remains memory-only: `handlers/admin_delete_barber.go` calls `storage.DeleteBarber` in `storage/barbers_store.go`, which has no DB branch and reads only `barbers`. In DB mode it normally returns not-found for a real persisted account; a populated memory bridge could delete memory without the DB account. Decide account archive/retention, implement a checked persistent mutation, preserve historical references and verify parity. |
| BE-16 | Open — P2 | `models/product.go`, `models/product_order.go`, checkout: money remains `float64`; define currency, precision/rounding and upper bounds, then use minor units or a decimal contract. |
| BE-17 | Open — P2 | `handlers/customer_dashboard.go`, `customer_bookings.go`, `barber_bookings.go`: upcoming categorization still uses status, not appointment date/time. Use each booking's timezone/snapshot and define overdue/no-show behavior. |
| BE-18 | Partial — P2 | SQL message scoping and SQL review `avg/count` are fixed. Discovery still loads all active barbers and per-shop services; seller catalogue filters in Go; threads/lists remain unpaginated. Add scoped, bounded queries and stable pagination. |
| BE-19 | Open — P1 | `handlers/admin_manage_barber.go:readBarberForm` collects state/area but required-field check omits them; `customer_register.go:readCustomerForm` only checks nonempty identity fields. Canonical geography and sensible identity formats/lengths remain. Email/barber identity verification is a separate product decision. |
| BE-20 | Implemented | `storage/reviews_store.go`, `review_integrity_test.go`: atomic completed-owned-booking eligibility/duplicate handling in both stores. Moderation remains optional policy. |
| BE-21 | Implemented | Embedded versioned migrations, migration command/startup compatibility and production persistence guard (`internal/dbmigrate/`, `cmd/migrate/`, runtime config). Fresh-install/upgrade provider acceptance remains. |
| BE-22 | Open / limited progress — P2 | `/` now reads persisted site settings, so it can detect some DB failures; it is still the configured health path. No separate liveness/readiness endpoint, signal-driven `Server.Shutdown`, structured request correlation or durable admin audit events. Verify backup restore operationally. |
| BE-23 | Partial — P2 | CI now runs frontend build/typecheck, Go tests/vet/race and PostgreSQL 17 integration; bad discovery fixture fixed and session tests exist. Browser journeys and dedicated security-package tests remain absent. This review did not independently establish remote CI results. |
| BE-24 | Implemented original cleanup | Duplicate review route and repeated review reset assignments removed; residual compact storage style is discretionary maintenance, not a functional release blocker. |

## Frontend status matrix

| ID | Status | Evidence and remaining work |
| --- | --- | --- |
| FE-01 | Open — P1 | Role headers still duplicated and destinations vary. `handlers/render.go` parses each page independently; shared role partials are absent. |
| FE-02 | Open — P1 | `static/app.css` recolors legacy controls/layouts without a complete reusable primitive system. Establish shared sizing/spacing/layout, then migrate consumers. |
| FE-03 | Open — P1 | Barber recovery still uses legacy `.wrap/.stack` and lacks auth shell; customer reset differs from newer auth cards. Include expired/error/success states. |
| FE-04 | Open — P1 | Customer barber-list linked images retain `data-fullscreen-image`; `static/image-viewer.js` prevents the link action. Separate named enlargement controls from navigation/selection. |
| FE-05 | Open — P1 | Viewer remains a div overlay without dialog/focus/inert behavior or keyboard image triggers. |
| FE-06 | Open — P1 | `frontend/app.ts:wireBookingSummary` still seeks `.actions`; detail template still uses `.submit-area`. No shared stable summary hook or server-rendered selected summary. |
| FE-07 | Implemented / superseded | Booking form now has date bounds and an available-slot selector; GET availability form selects service/date with timezone. Test browser and JavaScript-disabled flow rather than restoring obsolete free-time bounds. |
| FE-08 | Implemented core policy | Public and customer discovery both require state; state/area context flows through profile/auth/results. Dashboard does not remember the customer's prior locality; persistence is follow-up product work. |
| FE-09 | Implemented | `handlers/discovery_context.go`, auth/register handlers, public/detail templates and context tests preserve validated returns, locality/service, login rejection and signup destination. |
| FE-10 | Partial — P2 | State options are now sorted. Canonical region IDs, dependent area choices, data backfill/normalization and broader-search affordance remain absent. |
| FE-11 | Open — P2 | BB/DG fallbacks, ecosystem/product title and navigation labels remain inconsistent. |
| FE-12 | Open — P2 | Discovery/booking prices still use bare `printf "%.2f"`; commerce uses naira. No common grouped currency formatter. Coordinate with BE-16. |
| FE-13 | Open — P2 | Barber messaging retains large inline style blocks; shared stylesheet still has extensive role overrides. |
| FE-14 | Open — P2 | Numbered inconsistent headers and horizontal mobile menus with hidden scrollbars remain. |
| FE-15 | Open — P1 | Admin mobile CSS hides table headers and stacks cells without per-cell labels. Browser/assistive-tech acceptance required after repair. |
| FE-16 | Open — P2 | Admin badge CSS still names `Declined`; domain status is `Rejected`. |
| FE-17 | Partial — P2 | Public barber profile now uses `CustomerSignedIn` for dashboard/login. Landing flags remain unused; public result header still offers customer login. |
| FE-18 | Partial — P2 | Service-specific booking and seller-filtered shopping links implemented. Public portfolio video/enlargement and product imagery/specific-product continuation remain absent. |
| FE-19 | Partial — P1 | Checkout key/stock errors now re-render cart with appropriate status. Generic server errors, booking conflicts/not-found, CSRF and expiry paths still frequently use plain `http.Error`; shared actionable recovery surfaces remain. |
| FE-20 | Open — P2 | Frontend submit guard still defers disable with a timer and lacks synchronous per-form lock/pending text. Durable checkout replay is already fixed separately. |
| FE-21 | Open — P2 | Password reveal exists; generated field notes remain unassociated with inputs and shared `.visually-hidden` utility is absent. Documentation and legacy coverage need reconciliation. |
| FE-22 | Open — P2 | Chat Enter handler omits `isComposing`; opening chat always focuses composer. |
| FE-23 | Open — P2 / policy | No receive subscription/polling/unread model or clear refresh contract. Choose refresh-first versus incremental delivery before implementation. SQL scoping does not fix this UX. |
| FE-24 | Open — P2 | No coherent responsive image variants/srcset/sizes or measured video/hero performance budget. |
| FE-25 | Open — P2 / approved content needed | Shared support/policy/footer and discovery/profile canonical/share metadata remain absent. Define genuine destinations/content. |
| FE-26 | Verification pending — P2 | No evidence from this review for contrast, zoom, phone layouts, touch targets, long content, broken images or focus acceptance. Source CSS and passing Go tests are insufficient. |

## Recommended implementation order and acceptance

1. **Persistent account lifecycle (BE-15, remaining BE-09/10):** decide archive/retention; make admin actions work on real persisted accounts and preserve snapshots/media. Test DB/memory parity, ownership, conflicts, rollback and cancellation. This comes before general styling.
2. **Discovery and booking interaction repairs (FE-04/05/06/19):** separate image navigation/enlargement, accessible dialog, stable server-rendered summary and actionable error surfaces. Verify chosen locality/service survives sign-in/signup and invalid submission, with and without JavaScript.
3. **Server data quality (BE-19/FE-10), then money/status truth (BE-16/17, FE-12/16):** canonical location migration requires a documented region policy; money needs bounded values/rounding; upcoming requires timezone/date semantics. Test forged POSTs and boundary/history cases.
4. **Shared role UI (FE-01/02/03/13/14/15/21):** navigation/auth/form/status primitives and labeled mobile data. Include the new schedule/fulfillment UI in the component inventory. Verify keyboard, desktop/phone, empty/error and long-content states.
5. **Operational readiness and scale (BE-22/18/23):** dedicated readiness/liveness, graceful shutdown including workers, structured logs/audit, paginated scoped queries, browser coverage and security tests. Run backup restore and production provider checks.
6. **Remaining P2 polish/product decisions:** branding, public account actions/media, chat receive policy, submit/IME behavior, responsive image delivery and approved public support metadata.

Production acceptance for repaired booking/checkout/auth/media/migrations should run alongside these stages. It is not a reason to reimplement merged fixes. Online payments, verification/moderation, unread/realtime chat and support/legal content require explicit product choices.

## Verification performed and limits

- Fresh `git fetch origin`; reviewed main at `6377955` in an isolated detached worktree.
- `go test ./...`: passed on that checkout. PostgreSQL tests skip without `TEST_DATABASE_URL`; no live DB result is claimed.
- `go vet ./...`: passed on that checkout.
- `npm run typecheck`: passed in the original workspace with its installed dependencies. `frontend/app.ts` and image-viewer source are identical across the two baselines. No new dependency install or frontend build was needed for this documentation review.
- Inspected CI configuration; `gh` is unavailable, so remote CI status was not independently verified.
- No implementation changes, merge, deployment, live account mutation, provider configuration, browser screenshots/accessibility/performance tests or production inspection performed.

The original report's 44-template inventory is stale: latest main adds the barber schedule template (45 HTML templates). Its executive description and earlier “remaining sequence” also predate merged repairs. Treat this matrix as the current backlog and retain the original as historical rationale.
