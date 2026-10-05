# De Go Salon — frontend and backend gap diagnosis

Audit date: 5 October 2026. Baseline: local `main` checkout, initially clean.


## Remote reconciliation — 5 October 2026

Compared the supplied audit with remote `main` at `8e90e7c` (merged PRs #46, #47, and #48). The original findings below describe the earlier local baseline and must be read with this status update. Local uncommitted changes were preserved in the original checkout; implementation uses an isolated remote-based worktree.

| Gap | Remote status | Remaining acceptance work |
| --- | --- | --- |
| BE-01 | Booking POST invokes date/hour validators. | Retain regression coverage; avoid repeating the existing repair. |
| BE-02 | Partial: business timezone, elapsed same-day and closing-overrun validation, and duration overlap checks exist. | Available-slot UI/API, per-barber schedules/breaks/time off, durable duration snapshots, and PostgreSQL concurrency coverage. |
| BE-03 | Guarded handler transitions and transactional booking overlap checks exist. | Verify live PostgreSQL races, bookability changes during creation, and any other mutation callers. |
| BE-04 | Partial: order/items/stock/cart writes use one transaction. | Explicit idempotency key and replay result; transaction fault injection and PostgreSQL concurrent cart-add/checkout coverage. An empty cart on retry is not a replayable checkout result. |
| BE-05 | Partial: adding to cart no longer consumes inventory. | Quantity editing and removal. |
| BE-06 | Partial on remote: database listing exists, but dashboard never assigns orders and query failures can fall back to memory. | First change below wires the dashboard and propagates query/scan/iteration errors. Live database restart verification remains open. |
| BE-08 | Partial: checkout rechecks seller/product eligibility; add-to-cart checks eligibility. | Recoverable styled stock responses, stock-specific error semantics, and concurrent seller suspension verification. |
| BE-09 | Partial: PR #48 adds error-returning barber/hairstyle/product ID lookups and uses them in booking/cart handlers. Authentication and many list reads remain open. | The first change fixes customer order listing; remaining reads must stop hiding query/scan failures or falling back to memory. |
| BE-23 | Partial: committed GitHub Actions workflow runs TypeScript checks/build and Go tests. | Go vet/race checks in CI, PostgreSQL integration, browser coverage, session/security tests, and fixing the discovery negative-test fixture. |
| FE-07 | Remote booking form still needs the date/hour HTML constraints from the local audit patch. | Apply the preserved local change in a separate focused PR. |
| BE-24 | Local duplicate-route repair is not part of merged remote changes. | Carry the preserved fix separately; redundant reset assignments remain. |

The remote changes touch 16 files; templates, frontend scripts, authentication, media, schema, and storage modules outside booking/cart/orders and the new barber/hairstyle/product lookup helpers have not changed since the supplied audit baseline. Their unresolved findings remain the backlog, subject to the original evidence limits. No production, live PostgreSQL, provider configuration, or browser acceptance is claimed here.

### First implementation: customer-owned order history (BE-06)

The customer dashboard loads orders with the request context. Database query, scan, and row-iteration failures propagate instead of returning empty or in-memory history. Both stores return newest order IDs first and scope history to the signed-in customer.

Validation: Go tests and vet pass. New regression tests cover dashboard rendering, customer ownership, newest-first ordering, empty history, and failure of the database handle without memory fallback. A live PostgreSQL restart test has not been executed, so that acceptance requirement stays open.

### Second implementation: authentication lookup failures (BE-09)

Customer/barber session checks, login, and forgot-password requests now use error-returning account lookups with the request context. Query/scan failures return a generic server error and stop the request. A failed session lookup does not delete the session, expire its cookie, or redirect to login; the session remains usable when the lookup recovers. Only missing customers or missing/suspended barbers are signed out. Unknown accounts and incorrect passwords retain the generic credential rejection; unknown reset addresses retain the generic confirmation.

Regression coverage exercises query errors, invalid rows, missing rows, case-insensitive memory lookups, canceled requests, session preservation/recovery, missing/suspended account rejection, credential rejection, and reset-token preservation. SQL responses are controlled test fixtures rather than a live PostgreSQL server. Real database integration and the remaining boolean/list lookups stay open; BE-09 is not fully closed.

A further recovery defect was confirmed while tracing these paths: `SetCustomerPassword` and `SetBarberPassword` still write only to memory. Persistent password updates must be included in the recovery completion work (BE-11), alongside mail delivery and durable tokens.

### Third implementation: commerce read errors (BE-09)

Catalogue, seller inventory, cart lists/counts, order confirmation/items, portfolio products, and seller sales now return query/scan/iteration errors and use request contexts. Seller sales hydrate customer details consistently in both storage modes. Product ownership/deletion and add-to-cart preserve real database errors rather than relabeling every failure as missing records or stock conflicts.

Tests cover query/scan failures, interruption after a valid row without partial success, scoped seller sales, and failed commerce requests. Booking, discovery, reviews, messaging, and admin list/record reads remain to migrate before closing BE-09.

### Fourth implementation: booking, discovery, and admin read errors (BE-09)

Booking lists/actions/confirmation, appointment descriptions, barber/service discovery, review lists/ratings/eligibility, and admin views now propagate lookup/query/scan/iteration failures with request contexts. Unknown records retain missing-record responses. Admin dashboard upload-error rendering also uses the same checked dashboard loader, preventing false zero counts during outages. Review creation preserves eligibility-query errors instead of misreporting them as forbidden.

Regression coverage exercises storage query/scan failures and failed GET/POST handlers. Messaging reads remain to migrate before closing BE-09; live database concurrency/transaction verification stays open.

### Fifth implementation: scoped messaging queries (BE-09 / part of BE-18)

Message threads, conversation summaries, message writes, and dashboard conversation counts now use request contexts and propagate errors. Conversation SQL filters by the signed-in account before choosing the latest message and counting each pair; it no longer loads all platform messages into Go. Participant lookup failures also stop rendering. Tests cover SQL scope, pair isolation, ordering/counts, database failures, and failed thread/list rendering.

All request paths now use error-aware reads; legacy boolean/list helpers remain only as memory bridges/test APIs and must not be introduced into new request handlers. Remaining BE-09 acceptance is live database outage verification and any database mutations encountered in later integrity work. Message pagination, incremental receipt/unread behavior, and query bounds remain in BE-18 / FE-23.

### Sixth implementation: PostgreSQL integrity verification (BE-23 / acceptance for BE-02/03/04/06/09)

CI now adds Go vet/race checks and a dedicated PostgreSQL 17 job with isolated temporary databases. Integration tests exercise real booking/transition races, transactional checkout fault injection after an earlier item write, concurrent checkout and stock competition, storage reconnect history, scoped message SQL, foreign-key protection, RLS presence, and authentication sessions after injected SQL errors. The discovery negative fixture now proves the seeded barber appears in the matching region before checking exclusion.

Local unit/vet checks pass; live PostgreSQL results are established by the PR's dedicated CI job before merge. Integration tests skip locally when no test server is configured. Browser journeys, production provider settings, backup recovery, and the remaining scheduling/checkout idempotency behavior still need their own acceptance evidence.

### Remaining sequence

1. Add PostgreSQL integration/CI coverage and verify database outage behavior (BE-09/23). Request reads now propagate errors; continue removing legacy storage fallbacks and checking mutations during integrity work.
2. Make checkout retries replayable and verify transactional failure/concurrency against PostgreSQL (BE-04/08).
3. Add cart quantity editing/removal (BE-05).
4. Finish scheduling/available slots (BE-02), preserving guarded transitions.
5. Repair discovery/auth return context and booking/media hooks (Stage 2).
6. Consolidate navigation/components/accessibility (Stage 3).
7. Complete recovery, durable storage, bounded operations, migrations/readiness, and production/browser/database verification (Stage 4).

Use one focused branch and PR per bounded item. Wait for that PR's CI on its current commit to succeed, then merge before starting the next implementation. Do not add co-author trailers.

## Executive assessment

De Go Salon has a credible public design direction and a useful Go server-rendered foundation. The landing page and public Find a Barber experience establish forest surfaces, cream text, gold actions, human photography and a discovery → compare → book journey. Most authenticated pages now use this palette, but changing colors has not completed the design-system migration. Navigation, layout primitives, recovery forms, messages, data tables and interaction behavior remain fragmented.

The highest risks are functional: booking submissions bypassed existing date/hour validators; checkout writes orders in separate steps; carts consume stock indefinitely; order history is not connected to the customer dashboard; database failures can masquerade as missing records or empty lists. These problems directly undermine frontend promises and should precede a broad cosmetic redesign.

Recommendation: retain Go templates and progressive TypeScript enhancement. Stabilize domain behavior and establish reusable template/CSS components before another page-wide restyling pass. A SPA rewrite would not resolve the underlying transaction and availability problems.

## Scope, evidence and limits

- Inventoried all 44 HTML templates: public 3, customer 16, barber 17, admin 8. Reviewed their stylesheets, scripts, links and forms alongside routing, handlers, models, storage, sessions, reset tokens, schema and deployment configuration.
- Findings marked **confirmed** follow directly from source or executed tests. **Needs verification** means production settings, visual rendering or a product decision is required. Proposed enhancements are identified separately from defects.
- No production data, accounts, deployment or schema was changed. No live database integration or browser screenshot audit was performed. Responsive layout, exact contrast, browser compatibility and performance measurements therefore remain verification work; this report does not claim visual certification.
- Existing automated tests render real templates and cover selected flows, but do not establish full customer/barber/admin journey coverage or PostgreSQL behavior.
- `frontreadme.md` is a plan/checklist, not implementation evidence. Several checked items describe more consistency than the current source delivers.

Priorities: **P0** data integrity/security or invalid core behavior; **P1** broken journey/major consistency issue; **P2** usability, maintainability and scale; **P3** polish or optional product extension. P0 identifies engineering urgency, not a claim of a confirmed production incident.

## Reference design and consistency contract

Use `templates/home.html`, `templates/explore.html`, `static/app.css` token definitions and `static/customer-barbers.css` as the visual reference. Operational dashboards need denser layouts than marketing pages, but should preserve:

1. De Go Salon brand; Barber Multiverse as ecosystem language; DG fallback mark.
2. Forest/cream/gold tokens, semantic destructive/status treatments, consistent focus states.
3. One role-aware navigation model with stable destinations and active state.
4. Shared button, field, card, avatar, empty/error/success and price conventions.
5. Predictable discovery context, explicit appointment-request status and truthful stock/payment language.
6. Readable mobile layouts, keyboard-operable controls and accessible status feedback.

## Frontend findings

| ID | Priority / evidence | Gap and source | Impact and completion criteria |
| --- | --- | --- | --- |
| FE-01 | P1 · Confirmed | Duplicated, inconsistent role navigation across `templates/customer/*`, `templates/barber/*`, `templates/admin/*`. Customer dashboard exposes Products/Cart; barber-list/detail/confirmation omit them. Barber dashboard/products expose Products; hairstyles/bookings/profile omit it. Messages/profile/cart use separate header patterns. | Users lose destinations between screens. Introduce reusable role-aware Go template partials with stable ordering, account actions and `aria-current="page"`. Keep page-specific contextual back links. |
| FE-02 | P1 · Confirmed | Shared styling does not supply complete legacy primitives. Templates still use `.stack`, `.inline`, `.actions`, `.wrap`, `.button`, `.topbar` and `.nav`; `static/app.css` largely recolors them without complete shared layout/control definitions. | Admin/recovery/destructive-confirmation pages lack a reliable baseline. Define actual primitive layout, spacing, display, padding and control sizing before removing overrides; verify all consuming pages. |
| FE-03 | P1 · Confirmed | `templates/barber/forgot-password.html` and `barber/reset-password.html` use legacy `.wrap/.stack` layouts with `modern-role-page` but no `auth-page`. Customer reset uses `auth-page` with a legacy form; customer forgot/login/register use the newer auth-card structure. | Recovery is visually disconnected from sign-in and onboarding. Move all recovery states to shared auth layout, field and button components; check invalid/expired/submitted states too. |
| FE-04 | P1 · Confirmed | Customer barber-card photos sit inside profile links and carry `data-fullscreen-image`. `static/image-viewer.js` globally prevents the image click's default action. | Clicking a photo that says “View profile” opens the viewer instead. Keep the profile link as navigation; make enlargement a separate named button. Apply the same separation to service selection labels. |
| FE-05 | P1 · Confirmed | Image viewer has no dialog semantics, focus transfer/trap/restoration, inert background or keyboard-operable image trigger (`static/image-viewer.js`). Escape alone is supported. | Mouse users can enlarge images; keyboard and assistive-technology users lack an equivalent reliable flow. Use accessible dialog behavior and a labeled button trigger; test close/backdrop/Escape/focus return. |
| FE-06 | P1 · Confirmed | `frontend/app.ts:wireBookingSummary` searches for `.actions`, but the booking form ends with `.submit-area` in `customer/barber-detail.html`. | Price/duration summary enhancement silently does nothing despite rendered data attributes. Adopt explicit stable data hooks and render a usable selected-service summary without JavaScript. |
| FE-07 | P1 · Confirmed; initially repaired | Booking date/time inputs omitted `.Today`, `.Opening`, `.Closing` constraints even though the handler supplied them. | Browser and server accepted an unconstrained-looking appointment request. This audit adds date `min` and time `min/max`; remaining work is actual available slots, duration and timezone policy (BE-02). |
| FE-08 | P1 · Confirmed | Public discovery requires location before results; authenticated discovery lists every active barber until filtered (`handlers/public_explore.go`, `customer_barbers.go`). | The journey changes after login. Choose one documented initial-results policy and preserve selected state/area through profile, sign-in and back navigation. |
| FE-09 | P1 · Confirmed | Public profile's “Back to barber results” points to bare `/explore`; its login/registration/header paths do not preserve the selected location. Login failure drops `next` through generic `loginError`; registration always ends at dashboard. | Users must rediscover the shop/location after back navigation, a mistyped password or signup. Preserve validated return context throughout those paths and errors. |
| FE-10 | P2 · Confirmed | Location selection is a state dropdown built from existing barbers plus free-text area; registration/admin/editor state and locality are free text. Matching is exact except case. | Typos, inconsistent state names and area spelling create false no-results. Use canonical region identifiers/dependent area choices, normalize existing data, sort choices and provide a clear broader-search action. |
| FE-11 | P2 · Confirmed | Brand fallbacks vary between DG and BB in customer barber list/detail; barber dashboard title uses “Barber Multiverse” as product title. Labels vary between Find Barbers/Find a Barber/Explore Barbers and Services/Hairstyles/Handwork. | Weakens product identity and predictability. Define a naming glossary and shared brand/avatar component; preserve stylistic marketing copy without changing navigation names. |
| FE-12 | P2 · Confirmed | Public and authenticated discovery/service prices use bare decimal numbers; commerce uses ₦. | A customer cannot consistently identify currency or compare amounts. Use one server-side currency formatter with grouping and a defined currency policy throughout cards, summaries, bookings and orders. |
| FE-13 | P2 · Confirmed | Large inline stylesheet blocks remain in barber chat/list (711/684 total template lines); `app.css` uses many `!important` overrides over page CSS and retains a light legacy baseline. | Visual changes depend on selector order; the two messaging roles drift. Extract shared messaging styles and explicit layout variants, remove overrides incrementally and verify rather than deleting page CSS wholesale. |
| FE-14 | P2 · Confirmed | Numbered navigation differs with the number of links; mobile menus often scroll horizontally with hidden scrollbars (`app.css`). | Hidden destinations and unstable numbers impair navigation. Implement the agreed mobile drawer/overflow pattern with focus management, visible active state and safe-area support. Browser testing is still required. |
| FE-15 | P1 · Confirmed | Admin mobile tables hide the header and display cells as blocks without adding visible per-cell labels (`app.css`, admin bookings/customers/barber-list). Similar header removal occurs on barber dashboard data views. | Values become ambiguous on phones. Provide explicitly labeled responsive rows/cards while preserving meaningful table semantics where appropriate. |
| FE-16 | P2 · Confirmed | Admin badge CSS styles `.Declined`, but booking model uses `Rejected` (`app.css`, `models/booking.go`). | Rejected bookings get generic styling. Map actual domain states to centralized status components; keep text labels and verify every lifecycle state. |
| FE-17 | P2 · Confirmed | Public profile shows “Customer Login” even when `CustomerSignedIn` is true; landing receives signed-in flags but does not use them. | Returning users see onboarding actions instead of a route back to their workspace. Make public account navigation session-aware, without hiding discovery. |
| FE-18 | P2 · Confirmed | Public service cards have no service-specific booking link, no video rendering and no enlargement; public products are text-only and do not link to the specific seller/product shopping context (`explore-barber.html`). | The compare → choose → book/shop flow loses context. Preserve chosen service ID through auth and add deliberate portfolio/media and seller-filtered shopping actions. |
| FE-19 | P1 · Confirmed | Many conflicts and expiry responses use plain `http.Error`: cancellation/status conflict, stock errors, CSRF expiry, missing barber and checkout failure. | A styled journey ends on a raw text page without a useful recovery action. Render shared error/retry surfaces at the correct HTTP status; retain safe form values/context. |
| FE-20 | P2 · Confirmed | Double-submit guard disables buttons on a timer rather than synchronously locking submissions; it has no explicit action-specific progress text (`frontend/app.ts`). | Rapid or scripted repeats can still occur; users get weak progress feedback. Add immediate per-form submission state and a visible/announced pending message; backend idempotency remains essential. |
| FE-21 | P2 · Confirmed | Password reveal/feedback already exists, despite README listing it as future work. New field notes do not connect via `aria-describedby`/`aria-invalid`; `.visually-hidden` used by customer chat has no definition in its loaded styles. | Accessibility and documentation lag implementation. Associate feedback with inputs and standardize utilities; verify password controls on legacy auth/admin forms. |
| FE-22 | P2 · Confirmed | Chat Enter-to-send does not check `event.isComposing`; opening a conversation focuses the composer automatically (`frontend/app.ts`). | IME composition can unexpectedly send; mobile keyboards can displace the conversation. Respect composing events and make autofocus conditional/deliberate. |
| FE-23 | P2 · Confirmed | Messages are server-rendered on navigation/submission; there is no polling/stream subscription/unread model. | A received reply does not appear until refresh. Either label the refresh behavior clearly or implement incremental receive/unread handling with stable scroll and keyboard behavior. |
| FE-24 | P2 · Confirmed | Image sizing/lazy loading exists in some pages, but no coherent responsive image `srcset/sizes` strategy. Video and hero media lack measured delivery budgets. | Portfolio-heavy discovery can cost unnecessary bandwidth. Add image variants, reserved dimensions and intentional media loading; measure LCP/CLS/INP before assigning performance targets. |
| FE-25 | P2 · Confirmed | Public pages have no shared footer/help/contact/privacy/terms links; description metadata exists on landing but not public discovery/profile. | Users lack support/legal entry points and public profiles have weak search/share context. Define real support/policy destinations and add page-specific metadata/canonical/share output where approved. |
| FE-26 | P2 · Needs verification | Color tokens and reduced-motion handling are present, but exact contrast, 200% zoom, touch target sizes, 320px layout, long names, broken images and focus visibility have no demonstrated browser evidence. | Source-level responsiveness is not a usability acceptance test. Verify representative pages and every distinct component/state on desktop and phone viewports. |

## Page-family diagnosis and required work

This inventory covers every template. “Aligned” means the source uses the intended visual family, not that browser validation passed.

| Pages | Diagnosis relative to landing / Find a Barber | Priority work |
| --- | --- | --- |
| `home.html` | Strongest marketing reference; clear primary CTA and token system. Signed-in flags unused; no direct customer sign-in in main header. | Account-aware entry and shared public support/footer. |
| `explore.html`, `explore-barber.html` | Strong visual continuity; location-first results and clear review/portfolio sections. Back links lose filters; unsorted/free-text geography; service choice not carried to booking. | Discovery context, location normalization, service-specific continuation and honest availability. |
| Customer `login.html`, `register.html`, `forgot-password.html` | New auth-card family generally matches the palette. Login/new-account return intent is fragile. Forgot success is unsupported by email delivery. | Keep `next` through errors/signup; connect real recovery delivery. |
| Customer `reset-password.html`; barber `forgot-password.html`, `reset-password.html` | Legacy layout structures remain despite modernization labels. | Shared auth shell for all valid/expired/success/error states. |
| Barber `login.html`, `register.html`; admin `login.html` | New auth family; signup requires personal image and collects free-text geography. Admin recovery/MFA are absent. | Consistent password/field feedback, location quality and explicit admin access policy. |
| Customer `dashboard.html` | Forest design and broad navigation; orders section always receives an empty slice. Pending/accepted appointments classified by status alone. | Real order data, truthful counters, date-aware upcoming lists and repeat-user discovery context. |
| Customer `barber-list.html`, `barber-detail.html` | Close to public discovery but different navigation, BB fallbacks and initial search policy. Photo click and summary hooks are defective. | FE-04/06/08/09/11/12; repair availability flow before further polish. |
| Customer `booking-confirmation.html`, `bookings.html` | Forest cards, but different headers and action feedback. Confirmation depends on live service records; booking history uses status rather than appointment time. | Shared shell, clear request versus acceptance language, durable history and actionable conflicts. |
| Customer `chats.html`, `chat.html` | Forest colors with separate header/composer patterns; no incremental received-message flow. | Shared messages components, accessible composer/dialogs, receive/unread policy and pagination. |
| Customer `products.html`, `cart.html`, `order-confirmation.html` | Commerce family resembles the palette but loses full customer navigation. Cart is add/submit-only; stock and payment semantics incomplete. | Cart editing/removal, atomic/idempotent checkout, consistent currency and actual order status. |
| Customer `profile.html`, `edit-profile.html` | Forest profile family, reduced header navigation and inconsistent edit feedback. | Shared shell/forms, durable images, input quality and save confirmation. |
| Barber `dashboard.html`, `profile.html`, `edit-profile.html` | Forest operational family; navigation differs across pages and missing destinations. Dense profile/data views need phone verification. | Shared workspace navigation, readable labeled rows, valid business/location data and account actions. |
| Barber `hairstyle-list.html`, `hairstyle-profile.html`, `hairstyle-form.html`, `edit-hairstyle.html`, `confirm-delete-hairstyle.html` | Forest service family; Products destination absent. Delete flow and legacy confirmation styling diverge; service deletion conflicts with historical bookings. | Unified forms/confirmations; archive services and retain booking snapshots. |
| Barber `products.html`, `product-form.html` | New/edit behavior exists; README understates it. Inventory editing has no demonstrated reservation-aware contract. | Preserve order history, handle stock races, unify navigation and operational sales workflow. |
| Barber `bookings.html` | Large table-driven workflow with accept/reject/complete actions; global opening hours and exact-start-time conflicts are insufficient for a real schedule. | Interval availability, atomic transitions, mobile row labels and post-action feedback. |
| Barber `chats.html`, `chat.html` | Largest inline-style outliers and separate minimal headers. | Extract shared messaging UI and improve scoped data loading. |
| Admin `dashboard.html` | Forest overrides/KPI panels, but relies on incomplete legacy primitives; landing settings remain local files. | Complete base components, persistent settings, meaningful operational health and audit events. |
| Admin `barber-list.html`, `barber-profile.html`, `customers.html`, `bookings.html` | Forest data views; mobile header removal loses visible field labels. Filtering/data sets are unbounded. | Labeled responsive views, pagination, actual status styling and secure operational workflows. |
| Admin `edit-barber.html`, `confirm-delete.html` | Legacy form/confirmation structures with forest overrides; hard deletion can erase or conflict with relational history. | Consistent forms, archived account policy, clear impact/recovery and verified data constraints. |

## Backend and frontend-support findings

| ID | Priority / evidence | Gap and source | Required engineering outcome |
| --- | --- | --- | --- |
| BE-01 | P0 · Confirmed; initial fix complete | `readBookingRequest` accepted any nonempty date/time; `validBookingDate/Time` were tested but unused by the submission path. PostgreSQL date/time types do not enforce future dates or operating hours. | Fixed in this audit: invoke existing validators, normalize time, preserve rejected input and add handler regression coverage. Same-day elapsed time, timezone and interval validation remain open. |
| BE-02 | P1 · Confirmed | Booking collision logic/SQL unique index only compare identical barber/date/start time (`booking_storage.go`, `schema.sql`). Service duration is never used for collision detection; no per-barber hours, breaks, time off, lead time or timezone. | Define business timezone and schedule model; reject overlapping intervals atomically, including elapsed same-day slots and closing overruns. Provide available slots to the frontend. |
| BE-03 | P0 · Confirmed | Booking action handlers check the old status before calling unconditional `SetBookingStatus`. Concurrent accept/cancel/complete requests can overwrite a newer state. DB booking creation separately reads availability before insert. | Enforce allowed source states in the storage mutation/transaction; return conflict on stale actions. Lock/check bookability in the same transaction as creation. Test coordinated concurrent requests. |
| BE-04 | P0 · Confirmed | Checkout creates order, inserts each item and clears cart in separate calls (`customer_product_order.go`). There is no transaction or idempotency key. | All-or-nothing checkout with consistent pricing/eligibility and atomic cart consumption; safe retry/double-request handling. Fault injection must prove no partial order/cart-loss state. |
| BE-05 | P0 · Confirmed | `AddToCart` decreases product quantity immediately; no remove/change quantity/expiry/release flow exists (`cart_store.go`, customer router/cart template). | Choose explicit reservation or checkout-time stock accounting. Support removal/quantity edits and release abandoned reservations if used. Prevent indefinite stock exhaustion from carts. |
| BE-06 | P1 · Confirmed | Customer dashboard defines `ProductOrders` and renders them but never assigns the field. `ListProductOrdersByCustomer` only reads the in-memory slice even when PostgreSQL is enabled. | Implement DB-backed customer order listing and populate dashboard/history with owned orders. Verify orders survive restart and appear after checkout. |
| BE-07 | P1 · Confirmed | Route named `/customer/products/pay` only creates Pending orders. No payment provider/webhook, paid transition, customer order list route or seller fulfillment action exists. | Either rename/document “Place order” with collection/payment expectations or implement reconciled payments and per-seller fulfillment. Never imply completed payment from an order-write redirect. |
| BE-08 | P1 · Confirmed | Catalogue filters suspended sellers; add-to-cart and checkout do not consistently recheck barber active status/product availability. Stock race returns `ErrSlotTaken`, treated as server error by handler. | Revalidate seller/product eligibility atomically at mutation; use stock-specific domain errors and a recoverable 409 response. Test suspension/unavailability between browse and submit. |
| BE-09 | P0 · Confirmed | Numerous DB reads collapse query errors into “not found,” return nil or fall back to empty memory; row scan/iteration errors are skipped (`product_store.go`, `reviews_store.go`, `messages_store.go`, `booking_storage.go`, others). | Return typed errors; distinguish empty/missing from DB failure. Do not silently use memory when persistence is enabled. Log context and render retry/unavailable UI without signing users out for a DB outage. |
| BE-10 | P1 · Confirmed | Runtime storage queries generally lack request contexts/deadlines; DB pool limits/lifetime not set (`store.go`). Media uses `http.DefaultClient` with no dedicated timeout. | Propagate request context and establish bounded database/media operations, pool settings and cancellation. Server timeouts alone do not cancel every downstream operation. |
| BE-11 | P1 · Confirmed | Forgot-password writes token-bearing links/email to server logs; no mail delivery. Tokens and sessions are process-local (`password_reset.go`, `reset/tokens.go`, `session/manager.go`). | Deliver reset links through configured mail using a canonical site URL; persist expiring token hashes; prevent token logging. Support durable/shared sessions if restart or multi-instance continuity is required. |
| BE-12 | P1 · Confirmed | `render.yaml` omits `SECURE_COOKIES`; cookie Secure flag defaults off. No code-level auth/reset throttling or production guard for `SHOW_RESET_LINKS=1`/generated admin password. | Explicit HTTPS cookie configuration, throttling and safe production startup requirements. Blueprint has SHOW_RESET_LINKS=0, which is good; actual live cookie settings remain unverified. |
| BE-13 | P1 · Confirmed | Optional Supabase image upload exists, but videos stay local and landing image pointers are local text files. Bucket/env setup is absent from committed deployment configuration. Cloud delete helper only recognizes local URLs. | Persist videos/settings, configure bucket and environment requirements, and implement remote object cleanup/rollback. Do not report all uploads as local-only or assume optional cloud mode is deployed. |
| BE-14 | P1 · Confirmed | Customer profile edit calls `requireCSRF` (which parses form data) before applying request-body limit. Some other non-upload/auth handlers lack explicit limits. Image decode happens before bounding decoded pixel count. | Apply appropriate limits before parsing; reject excessive dimensions before full decode; bound all relevant forms. Test large multipart and highly compressed oversized image inputs. |
| BE-15 | P1 · Confirmed | Booking confirmation/history depend on live hairstyle records; SQL foreign keys block deleting referenced hairstyles/products, while in-memory deletion permits broken references. Barber/customer cascading deletion can remove history or encounter other restricting references (`schema.sql`, storage deletes). | Define archive/deletion/retention policy, snapshot service/seller details, and make database and test-store behavior consistent. Surface deletion conflicts honestly rather than treating all DB errors as missing rows. |
| BE-16 | P2 · Confirmed | Money is represented/calculated as `float64` before being stored in SQL `numeric`; currency is implicit (`models/product.go`, order handler). | Use integer minor units or an explicit decimal contract; define rounding, maximum quantities/totals and currency formatting. Test boundary totals. |
| BE-17 | P2 · Confirmed | Booking/dashboard “Upcoming” groups include all Pending/Accepted records regardless of date (`customer_bookings.go`, `customer_dashboard.go`, `barber_bookings.go`). | Compare appointment time in business timezone, define overdue/no-show treatment and separate historical unresolved requests. Avoid false upcoming counts. |
| BE-18 | P2 · Confirmed | Discovery loads all active barbers, then services and review lists per barber; summaries average full review arrays. Messaging summaries query all platform messages then filter in Go. Lists lack pagination. | Push filtering/aggregation/scoping to SQL; bound results and paginate messages/reviews/bookings/products. Preserve authorization predicates in every scoped query. |
| BE-19 | P1 · Confirmed | `readBarberForm` reads State/LocalArea but does not require either on the server, despite required HTML fields; a direct POST can create a shop invisible to public location discovery. Customer identity validation checks nonempty fields rather than valid email/phone. No email verification or barber identity/moderation gate beyond active status. | Require canonical location fields server-side, normalize/backfill data and apply sensible field length/format rules. Decide verification requirements as product policy; do not imply completed-booking reviews prove barber identity. |
| BE-20 | P2 · Confirmed | `CreateReview` chooses eligibility before the memory-store write lock, allowing duplicate concurrent reviews there; DB unique constraint catches duplicates but raw errors are not friendly domain conflicts. | Enforce one review per completed owned booking atomically in both stores and return a useful duplicate/conflict response. Add moderation/reporting only if product policy requires it. |
| BE-21 | P1 · Confirmed | Base schema is a manual SQL file; startup only creates/updates discovery fields/reviews, not a complete versioned schema migration. `DATABASE_URL` absence silently starts volatile in-memory mode. | Version migrations and document fresh install/upgrade; make production require persistence. Validate schema compatibility before serving traffic with clear diagnostics. |
| BE-22 | P2 · Confirmed | Health path `/` renders landing without exercising the database; no dedicated readiness endpoint, structured request correlation, graceful shutdown or durable admin audit log. | Separate liveness/readiness, bounded DB readiness checks, structured error/request logs and shutdown. Audit suspension/deletion/status changes and verify backup recovery operationally. |
| BE-23 | P2 · Confirmed | No committed CI workflow; session/security packages have no tests. Current storage tests use memory; no PostgreSQL concurrency/FK/transaction test harness or browser tests. One discovery “outside location” test calls `newServer` after seeding; `newServer` resets storage so it tests an empty store. | Fix the fixture and test negative/ownership/error journeys meaningfully. Add CI with frontend checks, Go tests/vet, database integration and targeted browser flow coverage. |
| BE-24 | P3 · Confirmed; cleanup complete | Customer router contained duplicate review route case; Reset repeats review clearing assignments; compressed DB code makes error/transaction boundaries hard to review. | Duplicate route removed here. Format/refactor storage incrementally around tested behavior; remove redundant resets without conflating cleanup with feature completeness. |

## What already works well

- Password hashing, generic login errors and dummy verification are implemented; this is not a plaintext-password system.
- Role-specific sessions, authenticated route gates and POST/CSRF checks protect many mutations. Booking/order confirmations check customer ownership.
- Public discovery hides inactive barbers and exposes portfolios without forcing login.
- Completed-booking reviews have a database uniqueness constraint and an eligibility model.
- PostgreSQL schema includes foreign keys, nonnegative numeric checks, RLS statements and exact-slot uniqueness. Exact-slot enforcement is useful but does not cover duration overlap.
- Images have size/type checks and JPEG/PNG normalization; optional cloud upload exists. Global focus/reduced-motion styles are also present.
- Template parsing at startup and buffered rendering reduce broken/half-written page failures.

The committed schema enables RLS with no public access policies, consistent with server-owned access. That is not evidence that live grants/policies or the direct database role have been audited. Supabase documents grants and RLS as separate controls; inspect live configuration before claiming exposure or safety. See [Supabase API security](https://supabase.com/docs/guides/api/securing-your-api) and [Storage bucket access](https://supabase.com/docs/guides/storage/buckets/fundamentals). The changelog markdown fetch was attempted but unavailable through the web reader; no Supabase API/schema implementation was changed in this audit.

## Work started in this audit

1. **BE-01 / FE-07:** Booking POST validation now uses existing date/time validators, rejects past/malformed dates and times outside configured hours, and normalizes accepted time. The form now receives date minimum and opening/closing bounds.
2. Added `handlers/booking_request_test.go`: exercises rejected booking POSTs through the real handler/templates, verifies no bookings are stored and rejected time values remain available; checks unpadded time normalization.
3. **BE-24:** Removed the duplicate review route from `handlers/customer_router.go`.

This is a bounded start on confirmed backend defects. Checkout, availability, migrations, payment, live storage and navigation redesign remain open; no claim is made that those gaps are fixed.

## Prioritized implementation plan and acceptance gates

### Stage 1 — protect core records

- Complete atomic/idempotent checkout and deliberate cart stock accounting (BE-04/05/08).
- Implement guarded booking transitions and duration/timezone availability (BE-02/03).
- Propagate database failures correctly and wire persistent order history (BE-06/09).
- Gate: invalid or overlapping bookings fail; concurrent actions preserve lifecycle; retried checkout produces one order; failures preserve cart/stock; database outages do not render false empty success states.

### Stage 2 — repair the discovery-to-booking experience

- Fix image navigation and summary hooks; preserve location/service/login/signup intent (FE-04/06/08/09/18).
- Standardize prices, appointment statuses and recoverable errors (FE-12/19).
- Gate: a user chooses locality and service, signs in or registers, books, sees Pending confirmation, manages the same appointment and returns to the same result context. Repeat with JavaScript disabled and rejected form values.

### Stage 3 — complete the shared frontend foundation

- Build Go partials for brand/role navigation and CSS primitives; migrate recovery/messages/admin views first (FE-01/02/03/13/14/15).
- Standardize identity/status/empty/error/field feedback and accessible media (FE-05/11/16/20/21/22).
- Gate: consistent desktop/phone destinations, labeled mobile data, keyboard-operable controls/dialogs, valid/error/empty/loading/long-content states and screenshots for every distinct layout family.

### Stage 4 — make production behavior dependable

- Deliver recovery emails, persist shared session/token/media/settings data and configure cookies/timeouts/migrations/readiness (BE-10/11/12/13/14/21/22).
- Add scoped queries/pagination, historical snapshots, CI and PostgreSQL/browser coverage (BE-15/17/18/20/23).
- Gate: data/media survive restart; recovery reaches the account owner; production refuses volatile configuration; status/stock/history behave identically against the database; monitoring distinguishes empty data from errors.

## Verification record

- Baseline `go test ./...`: passed.
- After initial booking fix, `go test ./...`: passed, including added regression tests.
- `go vet ./...`: passed.
- TypeScript dependency was initially absent; installed the pinned project dependency before frontend verification.
- `npm run typecheck` and `npm run build`: passed after dependency installation.
- Generated `static/app.js` matches the committed output; `git diff --exit-code -- static/app.js` passed.
- `git diff --check`: passed.
- Live PostgreSQL, Render settings, Supabase bucket/grants/policies, payment/email delivery, and browser visual/accessibility/performance tests: not executed.

## Release decision

The public design direction is worth keeping, but the repository should not be described as a fully consistent or production-verified platform. Prioritize integrity and interrupted journeys, then consolidate components. Close a gap only after its stated acceptance behavior is demonstrated; a palette change, checked planning checkbox or passing helper test is insufficient.


## Seventh implementation: durable account writes

Password replacement now writes PostgreSQL for both account roles. Customer profile edits and barber status changes also persist, and their production callers pass the request context. Profile writes explicitly exclude passwords and barber account status, so stale copies cannot undo a concurrent reset or suspension. SQL errors remain errors; missing rows and duplicate emails retain their separate meanings.

Acceptance: shared memory and live PostgreSQL cases check stale-profile protection, password replacement, suspension, duplicate emails, missing accounts, and canceled requests. The PostgreSQL CI suite added in PR #54 passed before that PR merged. Durable reset tokens, sessions, delivery, broader profile validation, and account archival remain separate gaps.


## Eighth implementation: review integrity (BE-20)

Review eligibility and insertion are atomic in memory; PostgreSQL uses an eligible owned completed-booking insert and its unique booking key with a controlled conflict result. Concurrent duplicate submissions cannot create multiple reviews or expose raw uniqueness failures. Customer names come from the owned account, and comment limits preserve complete Unicode characters.

Acceptance: shared memory/PostgreSQL cases exercise pending and unowned booking rejection, two simultaneous submissions, one stored review, trusted name snapshots, Unicode limits, and request cancellation. Full styled error presentation remains FE-19.


## Ninth implementation: cart editing and serialized checkout (BE-05 / part of BE-04/08)

Customer cart lines now have quantity and remove forms with CSRF protection, ownership checks, integer/stock validation, and stock-conflict feedback rendered within the cart at the correct HTTP status. Unavailable or suspended-seller items can always be removed. Stock is reserved only at checkout. Product-stock failures have their own error, rather than a booking-slot error.

All PostgreSQL cart writes and checkout lock the customer row before touching cart rows, including empty carts and newly added product lines. Product/seller eligibility reads hold locks until transaction completion. Checkout and cart changes accept request contexts. A PostgreSQL fault-delay test overlaps a new product addition with checkout and checks that the addition survives; shared memory/PostgreSQL editing tests verify ownership, stock, removal, and cancellation. HTTP tests cover rendered controls, CSRF, validation, stock recovery, and removal with an invalid quantity field. Explicit checkout replay keys, payment/fulfillment policy, and browser acceptance remain open.


## Tenth implementation: versioned database migrations (BE-21)

A CLI-created immutable baseline and embedded Go migration runner replace unrecorded startup DDL. The runner records checksums in a private RLS-protected ledger, serializes concurrent commands, and commits schema/history atomically. App startup verifies applied versions and checksums without changing the schema. Render builds the runner and executes it before starting the app; other deployments have an explicit documented migration command. No production database is directly changed by the development tools.

Acceptance: PostgreSQL CI tests fresh and simultaneous migration commands, existing-record/location-column upgrades, checksum rejection, RLS presence, and full rollback against an incompatible baseline. Pending or mismatched migrations prevent application startup. Live production deployment configuration, backups, and restore drills remain operational acceptance items; this does not claim that production has already run the migration.


## Eleventh implementation: replayable checkout (BE-04)

Cart checkout forms carry a cryptographically random request key. The customer-scoped key is saved with the order in the same transaction as items, stock, and cart removal. A duplicate request returns its original order before reading the current cart, including after storage reconnect; later cart additions remain untouched. Failed requests do not claim keys. Missing/malformed keys return a recoverable cart form, and the database enforces key format and per-customer uniqueness through a new versioned migration. Legacy pre-key orders remain valid.

Acceptance: shared memory/PostgreSQL cases cover simultaneous identical requests, one order/stock change, replay with a new cart, customer isolation, invalid keys, and retry after stock failure. PostgreSQL verifies replay after reconnect. HTTP coverage extracts the real form key, checks missing-key recovery, repeats the checkout, verifies identical confirmation destinations, and proves client-supplied totals are ignored. Payment/fulfillment policy and money representation remain BE-07/16.


## Twelfth implementation: preserved audit repairs (FE-07 / BE-24)

The booking form now renders its supplied minimum date and opening/closing time bounds. Server-side date, interval, timezone, and duration validation remains authoritative. The duplicate customer review route and duplicate review-reset assignments are removed. These changes carry the remaining applicable local audit repairs into the remote implementation while leaving the original uncommitted checkout untouched.

Acceptance: the existing real-template/application suite verifies the affected pages and flows; available-slot selection and per-barber schedule policy remain BE-02.


## Thirteenth implementation: discovery/auth continuation (FE-08/09 / part of FE-17/18)

Both public and customer discovery require a selected state before listing shops, sort available state choices, and provide a broader state search. Shop links and profile back links retain state/local-area filters. Public service actions preserve the selected service through login, failed credentials, account creation, and the booking form; rejected booking forms also retain location. Customer return URLs allow only workspace GET destinations and reject external URLs, traversal/encoded paths, authentication loops, and mutation endpoints.

Public profile navigation now shows the dashboard for a signed-in customer. Portfolio shopping continues to a selected-seller catalogue, including after auth, and adding a product retains that seller filter. Tests render the real pages and exercise failed login/signup, successful signup/login continuation, selected service, initial location policy, signed-in public navigation, seller filtering, and unsafe return URLs. Canonical geography (FE-10), landing-page account navigation, media/enlargement, shared navigation, and browser verification remain separately tracked.


## Fourteenth implementation: immutable booking metadata (BE-02 / part of BE-15)

New bookings snapshot service name, price, duration, barber name, shop name, and address inside the same atomic decision as availability/overlap checks. Service and seller reads hold row locks until insertion, and creation accepts the request context. Overlap checks use existing appointments' recorded durations, so editing a service cannot shorten occupied intervals retroactively. Confirmation/history rendering uses these immutable text/price/duration fields rather than depending on current service/shop records.

The versioned migration backfills existing appointments from retained metadata; it cannot reconstruct changes made before snapshotting existed. Shared memory/PostgreSQL cases verify server-owned fields, edits that leave original history/intervals intact, new appointments using new metadata, and cancellation. Real-template tests verify original confirmation/history details after edits. Archival/retention and immutable order seller details remain BE-15; per-barber schedules/available-slot selection remain BE-02.


## Fifteenth implementation: per-barber calendars and available times (BE-02)

Barbers can edit a persistent weekly calendar with closed days, a daily break, dated time-off periods, an IANA timezone, minimum notice, and an advance-booking horizon. Defaults use configured business hours, Africa/Lagos, 30 minutes notice, and a 365-day horizon. Customers select a service/date and receive server-rendered available start times on a 15-minute grid; the flow works without JavaScript. Occupied intervals use booked duration snapshots. All policy checks run again during atomic booking creation. Calendar edits share the barber's booking lock and cannot invalidate or shift an existing future appointment. Booking timezone is recorded for new appointments.

Acceptance: shared memory/PostgreSQL cases cover breaks/time off, closing overruns, occupied intervals, horizon, cancellation, timezone changes, and a race between time off and a new appointment. Unit checks cover lead time, closed weekdays, and nonexistent DST times. HTTP/template tests cover owner editing, CSRF, malformed-date recovery, and available-time selection. Browser interaction/accessibility verification remains part of FE-26/BE-23; live provider configuration remains operational acceptance.


## Sixteenth implementation: storage-owned appointment lifecycle (BE-03)

Booking storage now enforces the domain graph for every caller: barbers accept/reject Pending requests and complete Accepted appointments; customers cancel their own Pending or Accepted appointments. Terminal states cannot reopen, caller-supplied source lists cannot widen transitions, and the legacy setter uses the same enforcement. Exactly one positive owner identity is required; zero IDs do not imply an administrator bypass. New appointments default to Pending and reject any other supplied initial state. HTTP mutation callers propagate their request context.

Acceptance: shared memory/PostgreSQL tests exercise every source/target combination for both roles, unknown states, forged initial states, absent/ambiguous/wrong owners, legacy setter bypass attempts, and cancelled contexts. Existing PostgreSQL concurrent transition tests remain required in CI. Browser recovery and styled conflict feedback remain FE-19/BE-23.


## Seventeenth implementation: collection orders and seller fulfillment (BE-07)

Checkout now uses “Place order” and explicitly explains stock reservation, collection, and direct payment to each shop; no online payment is claimed. Each item has a persistent Pending → Ready → Collected lifecycle, with cancellation allowed from Pending/Ready. Sellers can change only their own items through CSRF-protected POST actions on their dashboard. Cancellation restores reserved stock exactly once. Whole-order status is recomputed from every seller's items under an order lock: Pending, Ready, Completed (all closed with at least one collected), or Cancelled (all cancelled). Original totals remain historical and cancelled items are explicitly not payable. Customer confirmation shows item fulfillment states. A CLI-created versioned migration backfills existing cancelled/completed records.

Acceptance: shared memory/PostgreSQL tests verify initial states, actor/transition restrictions, multi-seller aggregates, concurrent actions across sellers, duplicate cancellation races, stock accounting, terminal-state guards, and seller reads. PostgreSQL fault injection after stock/item updates proves complete rollback and safe retry. HTTP/template checks verify method/CSRF/ownership guards, seller actions, collection wording, customer state display, and conflict responses. Online payments are outside the chosen collection model; browser acceptance and styling remain FE-19/26 and BE-23.

## Eighteenth implementation: durable authentication sessions (BE-11/12, partial)

Configured PostgreSQL now persists authentication sessions in the private server schema, storing only SHA-256 bearer-token digests. Cookies retain role isolation, HttpOnly, SameSite=Lax, and the 12-hour lifetime; new bearer tokens use 32 random bytes. Indexed owner revocation and bounded expired-record cleanup support restart-safe logout and account-wide revocation. Every HTTP caller now uses error-aware session operations with request contexts and a five-second bound. Session query/write failures do not trigger memory fallback, false signed-out redirects, or cookie deletion on failed logout. The CLI-created migration keeps authentication data outside the public API schema.

Acceptance: memory tests verify cookie flags, role isolation, owner revocation, expiry, malformed tokens, identity checks, and cancelled creation. PostgreSQL tests verify token digests, database reconnect persistence, expiry cleanup, role-scoped revocation, and lookup/create/revoke failures. Real HTTP database tests verify session outages preserve cookies and recover, failed logout remains recoverable, reconnect preserves authentication, and successful logout revokes persistent state. CI includes the new session package. Password-reset token persistence/atomic consumption/delivery, production configuration guards, and abuse throttling remain BE-11/12; these are not claimed complete by this session change.


## Nineteenth implementation: durable atomic password recovery (BE-11, partial)

Reset-token digests now persist in the private, RLS-enabled server schema, with one expiring token per role/account and bounded expiry cleanup. Issuance/replacement and completion share an owner lock. Completion rechecks and locks the token, writes the password, revokes existing sessions, and consumes the token in one PostgreSQL transaction. Any failed write rolls all three back; concurrent submissions cannot both succeed. Memory completion also preserves a failed-write token and serializes single use. Every HTTP token operation preserves SQL errors rather than presenting an expired link. Raw reset links are no longer logged; reset pages use no-store/no-referrer headers and bounded bodies.

Acceptance: token tests cover reconnect persistence, digest storage, replacement, role isolation, expiry, query/issuance outages, and concurrent completion. PostgreSQL fault injection fails password updates, session revocation, and token deletion separately and verifies full rollback plus retry. Real HTTP tests for both customers and barbers verify token-read outages, failed password writes preserving token/session state, successful retry, old-password rejection, old-session revocation, and single use. Local tests/vet/reset-session-storage race checks pass; dedicated PostgreSQL CI is required before merge. Mail delivery with canonical links remains open, as do production guards, abuse throttling, and preventing an in-flight login verified against an old password from opening a session after reset (BE-11/12).


## Twentieth implementation: verified session creation (BE-11/12)

Customer/barber logins and customer registration now recheck the password hash that was verified and current barber eligibility inside session creation. PostgreSQL holds an account row share lock until the session commits, serializing against password changes/suspension. A reset that runs after login revokes that session; a login that follows reset rejects the stale password snapshot before issuing a cookie. Development checks run under the session lock so memory recovery cannot miss a concurrently inserted session. Changed credentials receive a recoverable sign-in response, while operational errors remain server errors. Registration no longer deletes profile media referenced by an already-committed account when automatic login fails.

Acceptance: shared memory/PostgreSQL checks verify old-password snapshots and suspended barbers cannot create sessions/cookies, while current credentials work. PostgreSQL race tests use controlled transaction locks to prove both login-before-reset and reset-before-login outcomes leave no usable stale session. Existing recovery failure/revocation tests remain enabled. Production cookie/configuration guards, throttling, durable mail delivery, and media lifecycle acceptance remain open.

## Twenty-first implementation: production authentication configuration (BE-12/21)

Shared runtime configuration now treats explicit production mode and Render runtime as production. Startup rejects missing persistent storage, missing administrator identity, administrator passwords outside 12–72 bytes, insecure-cookie overrides, demo reset-link exposure, and malformed mode/boolean settings. Administrator initialization cannot generate a production password. Database initialization independently refuses production memory fallback. Session creation/logout enforce Secure cookies in production, and reset-link rendering is independently disabled there. The Blueprint explicitly sets production mode and secure cookies; local development behavior remains available without production markers.

Acceptance: configuration matrices verify required settings, supported modes/flags, Render detection, local demo compatibility, and secret-free errors. A subprocess test proves actual production startup exits before serving when persistence is absent. Authentication/storage tests verify independent generated-password/link/fallback guards and Secure creation/logout cookies. Local tests, vet, and configuration/session/storage race checks pass. CI validates the deployment build and Blueprint before merge. Live deployed secret values/settings are not claimed verified; rate limiting, delivery, media durability, and readiness/operational acceptance remain open.
