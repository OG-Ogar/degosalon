# Remaining implementation gaps

Updated: 7 October 2026. Baseline: main at `a84e418016c87511b94e13b96959a23b163a3cbc` (including merged PR #91). This list reconciles the original gap IDs with merged source. Implementation and production acceptance are tracked separately.

## Deployment checkpoint

Render deployment investigation is in progress. No workspace is selected in the Render connection; the available workspace is **My Workspace** (`tea-dav5lgvpn0mc73a8ki50`). Workspace confirmation is required before inspecting its service or changing deployment settings. Deployment health is not yet verified.

The repository Blueprint builds the frontend, app, migration CLI and media CLI, then starts `./migrate && ./media -create-bucket && ./app`. Check actual service commands against this configuration, then inspect build/runtime logs and verify PostgreSQL connectivity, migrations, Supabase bucket access, production admin configuration, secure cookies and SMTP configuration. Do not record a successful deployment until the deployed commit and live HTTP responses have been checked.

## Features and repairs still required

| Gap IDs | Remaining work | Completion target |
| --- | --- | --- |
| FE-10, BE-19 | Canonical geography and broader discovery | Choose canonical state/area IDs, dependent choices, normalization/backfill and broader-search behavior. Sorted free-text options are insufficient. |
| FE-02 | Complete shared layout/control primitives | Shared role navigation is merged. Finish reusable form, confirmation and layout primitives across remaining page families. |
| FE-16 | Consistent domain status presentation | The admin Rejected badge is repaired; centralize actual domain states, labels and treatments across all views. |
| FE-21 | Complete accessible field feedback | Shared hidden-text utilities, password note associations, invalid-field feedback and message character-count associations are merged. Audit server errors and remaining forms; shared feedback does not close the cross-page audit. |
| FE-24 | Responsive media delivery | Add suitable image variants, srcset/sizes, reserved dimensions and deliberate video/hero loading; measure performance and set budgets. |
| FE-25 | Support/legal content and metadata | Approve real support, help, privacy and terms destinations. Public profile descriptions/canonical markup have started; complete canonical/share metadata across public pages with the deployed origin. |
| BE-18 | Scoped queries and stable pagination | Messages are scoped and review aggregates use SQL. Push discovery filters/counts into SQL, avoid per-shop loading where appropriate and bound lists/threads. |
| BE-22 | Operational lifecycle and audit tooling | Add dedicated liveness/readiness, signal-driven graceful shutdown including workers, correlated structured logs and durable admin audit events. |
| BE-23 | Browser and direct security coverage | Go tests/vet/race, frontend checks, PostgreSQL CI and Playwright browser CI exist. PR #91 passed all three CI jobs before merging. Dedicated security-package coverage and broader browser journeys remain. |
| FE-26 | Browser accessibility/responsiveness acceptance | Finish contrast, keyboard/focus, 200% zoom, 320px width, touch targets, long content and broken-media checks across all page families. Fix findings and retain evidence. |

## Reconciled messaging and browser implementation

[PR #91](https://github.com/OG-Ogar/degosalon/pull/91), `feat/frontend-messaging-browser-verification`, merged on 7 October 2026. Final checked head: `8e53fb81095ca6e81ab66301bc900242311b3851`. The [final PR CI run](https://github.com/OG-Ogar/degosalon/actions/runs/37597022175) passed `verify`, `postgres` and `browser`; CodeRabbit also reported success. Earlier failed runs are superseded by this final head.

FE-13's shared messaging stylesheet and FE-23's explicit manual-refresh contract are implemented on main. Both roles load `/static/messages.css`; conversation pages explain that incoming messages appear after refresh/send and expose named refresh links. Polling, streaming and unread state remain optional product work. Expanded field feedback is merged, but FE-21's audit of remaining forms and server errors stays open. Browser CI is implemented; BE-23's dedicated security coverage and FE-26's full browser acceptance remain open. Passing this scoped suite does not certify production or every page family.

## Production acceptance still outstanding

These are verification tasks for existing implementations, not necessarily missing features:

- Fresh and upgrade PostgreSQL migrations; concurrent booking overlap and lifecycle transitions; checkout replay/rollback; archival and retained history.
- Actual reset-email inbox delivery, sender configuration, shared keys, secure deployed cookies and throttle/proxy behavior.
- Durable media upload/read/cleanup, restart survival, bucket configuration and any legacy-media migration.
- Database backup restoration, outage recovery, deployed startup/readiness and graceful lifecycle behavior.
- Render build and runtime logs, live application responses and deployed commit identity.

## Merged repairs excluded from the backlog

BE-09/10's [caller audit](storage-caller-audit.md) confirms active request paths use checked context-aware storage APIs. Five remaining affected-row error omissions are repaired, with driver fault-injection coverage of updates, transitions and checkout rollback. Bounded detached media reconciliation remains intentional. Provider outage/pooler cancellation still needs production acceptance; worker shutdown is tracked under BE-22.

BE-16 uses an explicit NGN two-decimal contract matching existing PostgreSQL numeric(12,2) columns: nonnegative amounts up to ₦9,999,999,999.99, with excess submitted precision rejected rather than silently rounded. All cart/checkout multiplication and addition use bounded integer kobo. Existing float64 model fields remain compatibility adapters for SQL scans and templates, not business arithmetic. Product/service/order writes validate the contract; order-item subtotals must equal exact unit-price multiplication. Oversized carts remain editable with checkout disabled; checkout independently rejects them before stock/cart/order mutation. PostgreSQL CI exercises fractional totals, overflow rollback and retry-key reuse after correction.

BE-17 compares open appointments against one request-time instant using the recorded timezone, accepting both legacy HH:MM and PostgreSQL HH:MM:SS values. Elapsed or unverifiable open appointments appear in a separate Needs follow-up section with message/outcome actions; page reads never infer no-shows or mutate lifecycle status. Customer upcoming lists and both dashboards exclude elapsed appointments; pending counters still count all pending requests. Legacy rows without timezone snapshots use the configured legacy booking timezone.

BE-19's shared customer/barber identity parsers validate bounded international names, email syntax and phone digit/format limits; barber forms also require bounded shop/address/state/area fields. These parsers cover customer/barber registration and profile edits plus administrator barber creation/update. Invalid submissions are rejected before account/media mutation, with values retained. Canonical geography remains separate under FE-10/BE-19; existing records are not normalized or backfilled by these input checks.

PRs #86–90 implemented separate image enlargement controls and a native accessible viewer (FE-04/05), submission locks and pending announcements (FE-20), IME-safe chat and deliberate mobile focus (FE-22), recovery layouts (FE-03), labeled mobile tables (FE-15), shared role navigation/mobile wrapping (FE-01/14), grouped currency display (FE-12), brand fallbacks (FE-11), customer-aware public navigation (FE-17), public portfolio media and exact-product continuation (FE-18), and shared HTML error recovery (FE-19). Remaining form-state/context preservation should be assessed during browser acceptance.

Earlier main already contains booking/schedule/lifecycle safeguards (BE-01/02/03, FE-06/07), transactional checkout/cart/history/fulfillment and eligibility checks (BE-04–08), durable recovery and secure configuration (BE-11/12), durable media and bounded uploads (BE-13/14), historical snapshots and archival (BE-15), review integrity (BE-20), migrations/persistence guards (BE-21), duplicate cleanup (BE-24), and location-first discovery/auth return context (FE-08/09).

## Decisions requiring product input

Online payments are optional under the current collection policy. Email/barber verification, review moderation, remembered locality and realtime unread chat are not automatically missing requirements. Canonical geography and approved legal/support content need explicit definitions.

## Execution order

1. Restore and verify the Render deployment.
2. Reconcile merged messaging/browser status and verify the manual-refresh conversation contract; retain full browser acceptance as separate work.
3. Implement validation, exact money and appointment-time semantics in independently reviewable branches.
4. Finish storage/query/lifecycle/security work.
5. Complete media, content and cross-page browser acceptance.

For each batch: branch from current main, implement and verify, push, wait for CI to pass, merge, then begin the next batch.
