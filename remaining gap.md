# Remaining implementation gaps

Updated: 7 October 2026. Baseline: main at `a84e418016c87511b94e13b96959a23b163a3cbc` (including merged PR #91). This list reconciles the original gap IDs with merged source. Implementation and production acceptance are tracked separately.

## Deployment checkpoint

Render deployment investigation is in progress. No workspace is selected in the Render connection; the available workspace is **My Workspace** (`tea-dav5lgvpn0mc73a8ki50`). Workspace confirmation is required before inspecting its service or changing deployment settings. Deployment health is not yet verified.

The repository Blueprint builds the frontend, app, migration CLI and media CLI, then starts `./migrate && ./media -create-bucket && ./app`. Check actual service commands against this configuration, then inspect build/runtime logs and verify PostgreSQL connectivity, migrations, Supabase bucket access, production admin configuration, secure cookies and SMTP configuration. Do not record a successful deployment until the deployed commit and live HTTP responses have been checked.

## Features and repairs still required

| Gap IDs | Remaining work | Completion target |
| --- | --- | --- |
| FE-02 | Implemented and merged in PR #103 | Shared page/panel/form/field/control/action primitives now serve all template families; barber and admin archive confirmations share a CSRF-preserving partial. See `ui-primitives.md`. Full browser acceptance remains FE-26. |
| FE-21 | Implemented and merged in PR #104 | Audited all template-rendered error forms, shared server summaries and no-script form associations; associated remaining field/upload help and repaired native select feedback clearing. See `form-feedback-audit.md`. Full visual/browser acceptance stays FE-26. |
| FE-24 | Implemented on this branch; CI/merge pending | Owned responsive thumbnails, srcset/sizes, reserved slots, eager responsive heroes, original enlargement and deferred videos; measured fixture/browser budgets are recorded in `media-performance.md`. Real deployed-device performance remains production acceptance. |
| FE-25 | Final support/legal content and deployment metadata acceptance | Shared destinations and explicitly approved placeholders are implemented, with configured-origin canonical/share metadata across public pages. Replace placeholders with approved contacts/policies and verify the deployed canonical origin. |
| BE-18 | Scoped queries and stable pagination | Messages are scoped and review aggregates use SQL. Push discovery filters/counts into SQL, avoid per-shop loading where appropriate and bound lists/threads. |
| BE-22 | Operational lifecycle and audit tooling | Add dedicated liveness/readiness, signal-driven graceful shutdown including workers, correlated structured logs and durable admin audit events. |
| BE-23 | Broader browser journey coverage | Go tests/vet/race, frontend checks, PostgreSQL CI, Playwright browser CI and dedicated security-package tests exist. Broader browser journey coverage remains aligned with FE-26. |
| FE-26 | Browser accessibility/responsiveness acceptance | Finish contrast, keyboard/focus, 200% zoom, 320px width, touch targets, long content and broken-media checks across all page families. Fix findings and retain evidence. |

## Reconciled messaging and browser implementation

[PR #91](https://github.com/OG-Ogar/degosalon/pull/91), `feat/frontend-messaging-browser-verification`, merged on 7 October 2026. Final checked head: `8e53fb81095ca6e81ab66301bc900242311b3851`. The [final PR CI run](https://github.com/OG-Ogar/degosalon/actions/runs/37597022175) passed `verify`, `postgres` and `browser`; CodeRabbit also reported success. Earlier failed runs are superseded by this final head.

FE-13's shared messaging stylesheet and FE-23's explicit manual-refresh contract are implemented on main. Both roles load `/static/messages.css`; conversation pages explain that incoming messages appear after refresh/send and expose named refresh links. Polling, streaming and unread state remain optional product work. Expanded field feedback is merged, but FE-21's audit of remaining forms and server errors stays open. Browser CI is implemented; BE-23's broader browser journeys and FE-26's full browser acceptance remain open; dedicated security coverage is merged. Passing this scoped suite does not certify production or every page family.

## Production acceptance still outstanding

These are verification tasks for existing implementations, not necessarily missing features:

- Fresh and upgrade PostgreSQL migrations; concurrent booking overlap and lifecycle transitions; checkout replay/rollback; archival and retained history.
- Actual reset-email inbox delivery, sender configuration, shared keys, secure deployed cookies and throttle/proxy behavior.
- Durable media upload/read/cleanup, restart survival, bucket configuration and any legacy-media migration.
- Database backup restoration, outage recovery, deployed startup/readiness and graceful lifecycle behavior.
- Render build and runtime logs, live application responses and deployed commit identity.

## Merged repairs excluded from the backlog

FE-16's shared presentation catalog and badge partial cover appointment, barber-account, order, item fulfillment and catalogue availability states. Labels, descriptions and five visual treatments are centralized; unknown or cross-domain values render a neutral Unknown status and never become CSS classes. All current status views use the shared component. Booking details now describe the stored current state instead of always promising pending approval, including when revisiting rejected/cancelled/completed appointments. A real booking/cancellation browser journey retains the revisited status check and fixes the booking-details theme contrast it exposed. Persisted lifecycle values and mutation rules remain unchanged.

FE-25's implementation adds shared Help/Support/Privacy/Terms destinations and footer links across template-rendered page families. Content uses the requested, explicit placeholders, with no fabricated contact or legal agreement; placeholder pages send noindex/follow. Help contains current app instructions. Home, discovery, public shop profiles and content pages share descriptions, Open Graph/Twitter metadata and absolute canonical URLs from the configured HTTPS SITE_URL/Render origin; request headers and discovery/session query parameters never determine the canonical URL. Final owner-approved content and actual deployed-origin verification remain acceptance work.

FE-10/BE-19 uses Nigeria's 36 states, FCT and 774 LGAs/area councils, with stable application IDs, parent-bound choices and explicit legacy aliases. Barber registration/self-edit/admin forms require a valid pair; public/customer discovery accepts both IDs and existing name-based links, offers all locations even where no shops exist, and retains whole-state broadening. Native grouped selects work without JavaScript; the enhanced controls limit LGAs to the selected state and clear an incompatible choice. The migration normalizes/backfills recognized active/suspended records, preserves unknown legacy areas without guessing, and leaves archived historical records untouched. Private RLS-enabled catalogs and foreign keys retain matching parent relationships. [Data sources and adaptation notes](internal/geography/README.md) document naming changes and licensing. SQL discovery scoping and pagination remain under BE-18.

BE-23's direct password-security coverage checks independent PBKDF2-SHA256 vectors, multiblock derivation, fresh salts, exact password verification and malformed digest handling. Digest verification rejects oversized encodings, excessive iteration counts and incorrect salt/key lengths before hashing; existing application-generated digests retain their format and iteration count. A fuzz target covers malformed input robustness. Broader browser journeys remain open.

BE-09/10's [caller audit](storage-caller-audit.md) confirms active request paths use checked context-aware storage APIs. Five remaining affected-row error omissions are repaired, with driver fault-injection coverage of updates, transitions and checkout rollback. Bounded detached media reconciliation remains intentional. Provider outage/pooler cancellation still needs production acceptance; worker shutdown is tracked under BE-22.

BE-16 uses an explicit NGN two-decimal contract matching existing PostgreSQL numeric(12,2) columns: nonnegative amounts up to ₦9,999,999,999.99, with excess submitted precision rejected rather than silently rounded. All cart/checkout multiplication and addition use bounded integer kobo. Existing float64 model fields remain compatibility adapters for SQL scans and templates, not business arithmetic. Product/service/order writes validate the contract; order-item subtotals must equal exact unit-price multiplication. Oversized carts remain editable with checkout disabled; checkout independently rejects them before stock/cart/order mutation. PostgreSQL CI exercises fractional totals, overflow rollback and retry-key reuse after correction.

BE-17 compares open appointments against one request-time instant using the recorded timezone, accepting both legacy HH:MM and PostgreSQL HH:MM:SS values. Elapsed or unverifiable open appointments appear in a separate Needs follow-up section with message/outcome actions; page reads never infer no-shows or mutate lifecycle status. Customer upcoming lists and both dashboards exclude elapsed appointments; pending counters still count all pending requests. Legacy rows without timezone snapshots use the configured legacy booking timezone.

BE-19's shared customer/barber identity parsers validate bounded international names, email syntax and phone digit/format limits; barber forms also require bounded shop/address/state/area fields. These parsers cover customer/barber registration and profile edits plus administrator barber creation/update. Invalid submissions are rejected before account/media mutation, with values retained. Canonical geography is now handled by the separate FE-10/BE-19 catalog and migration described above.

PRs #86–90 implemented separate image enlargement controls and a native accessible viewer (FE-04/05), submission locks and pending announcements (FE-20), IME-safe chat and deliberate mobile focus (FE-22), recovery layouts (FE-03), labeled mobile tables (FE-15), shared role navigation/mobile wrapping (FE-01/14), grouped currency display (FE-12), brand fallbacks (FE-11), customer-aware public navigation (FE-17), public portfolio media and exact-product continuation (FE-18), and shared HTML error recovery (FE-19). Remaining form-state/context preservation should be assessed during browser acceptance.

Earlier main already contains booking/schedule/lifecycle safeguards (BE-01/02/03, FE-06/07), transactional checkout/cart/history/fulfillment and eligibility checks (BE-04–08), durable recovery and secure configuration (BE-11/12), durable media and bounded uploads (BE-13/14), historical snapshots and archival (BE-15), review integrity (BE-20), migrations/persistence guards (BE-21), duplicate cleanup (BE-24), and location-first discovery/auth return context (FE-08/09).

## Decisions requiring product input

Online payments are optional under the current collection policy. Email/barber verification, review moderation, remembered locality and realtime unread chat are not automatically missing requirements. Product decisions: use Nigeria states/FCT and official LGAs/area councils for geography; use clearly labeled placeholders for support/help/privacy/terms until final content is approved.

## Execution order

1. Restore and verify the Render deployment.
2. Reconcile merged messaging/browser status and verify the manual-refresh conversation contract; retain full browser acceptance as separate work.
3. Implement validation, exact money and appointment-time semantics in independently reviewable branches.
4. Finish storage/query/lifecycle/security work.
5. Complete media, content and cross-page browser acceptance.

For each batch: branch from current main, implement and verify, push, wait for CI to pass, merge, then begin the next batch.
