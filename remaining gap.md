# Remaining implementation gaps

Updated: 6 October 2026. Baseline: main at `55d485be8a6e5bc4dd8ea6e9dd6f1a09e73541a6` (merged PRs #86–90). This list reconciles the original gap IDs with merged source. Implementation and production acceptance are tracked separately.

## Deployment checkpoint

Render deployment investigation is in progress. No workspace is selected in the Render connection; the available workspace is **My Workspace** (`tea-dav5lgvpn0mc73a8ki50`). Workspace confirmation is required before inspecting its service or changing deployment settings. Deployment health is not yet verified.

The repository Blueprint builds the frontend, app, migration CLI and media CLI, then starts `./migrate && ./media -create-bucket && ./app`. Check actual service commands against this configuration, then inspect build/runtime logs and verify PostgreSQL connectivity, migrations, Supabase bucket access, production admin configuration, secure cookies and SMTP configuration. Do not record a successful deployment until the deployed commit and live HTTP responses have been checked.

## Features and repairs still required

| Gap IDs | Remaining work | Completion target |
| --- | --- | --- |
| BE-19 | Server location and identity validation | Validate required state/area and bounded, sensible name/email/phone inputs on every relevant mutation. |
| FE-10, BE-19 | Canonical geography and broader discovery | Choose canonical state/area IDs, dependent choices, normalization/backfill and broader-search behavior. Sorted free-text options are insufficient. |
| BE-16 | Exact, bounded money arithmetic | Replace floating-point business amounts with minor units or a defined decimal contract; enforce rounding and upper bounds across storage, cart and checkout. FE-12 display formatting is merged, but does not close this backend gap. |
| BE-17 | Truthful upcoming appointment classification | Compare appointment instants using the recorded timezone; define overdue/no-show behavior instead of grouping by status alone. |
| FE-02 | Complete shared layout/control primitives | Shared role navigation is merged. Finish reusable form, confirmation and layout primitives across remaining page families. |
| FE-13 | Consolidated messaging presentation | Shared message styles are proposed in PR #91, but are not merged. Complete browser validation first. |
| FE-16 | Consistent domain status presentation | The admin Rejected badge is repaired; centralize actual domain states, labels and treatments across all views. |
| FE-21 | Complete accessible field feedback | Shared hidden-text utilities and password note associations are merged. Broader invalid-field/character-count associations are proposed in PR #91 and require validation. Audit server errors and remaining forms. |
| FE-23 | Explicit incoming-message contract | PR #91 proposes named manual-refresh links. Merge and verify that contract, or deliberately choose polling/streaming with unread state and stable scroll. |
| FE-24 | Responsive media delivery | Add suitable image variants, srcset/sizes, reserved dimensions and deliberate video/hero loading; measure performance and set budgets. |
| FE-25 | Support/legal content and metadata | Approve real support, help, privacy and terms destinations. Public profile descriptions/canonical markup have started; complete canonical/share metadata across public pages with the deployed origin. |
| BE-09/10 | Storage error and cancellation caller audit | Trace reachable callers of legacy helpers; migrate affected request paths to checked context-aware APIs and handle affected-row errors. Wrapper presence alone is not proof of a live defect. |
| BE-18 | Scoped queries and stable pagination | Messages are scoped and review aggregates use SQL. Push discovery filters/counts into SQL, avoid per-shop loading where appropriate and bound lists/threads. |
| BE-22 | Operational lifecycle and audit tooling | Add dedicated liveness/readiness, signal-driven graceful shutdown including workers, correlated structured logs and durable admin audit events. |
| BE-23 | Browser and direct security coverage | Go tests/vet/race, frontend checks and PostgreSQL CI exist. Browser CI in PR #91 is failing and unmerged; dedicated security-package coverage remains. |
| FE-26 | Browser accessibility/responsiveness acceptance | Finish contrast, keyboard/focus, 200% zoom, 320px width, touch targets, long content and broken-media checks across all page families. Fix findings and retain evidence. |

## Open implementation branch

[PR #91](https://github.com/OG-Ogar/degosalon/pull/91), `feat/frontend-messaging-browser-verification`, is not merged. Latest checked head: `1a85629337855c6936496eddc4416eb34e2d9d49`. Go verification and PostgreSQL checks passed; browser verification failed. Its proposed messaging styles, explicit refresh behavior, expanded field feedback and browser CI must not be counted as completed on main. An earlier browser run exposed customer empty-state contrast and an ambiguous test locator; follow-up changes still require a successful browser run.

## Production acceptance still outstanding

These are verification tasks for existing implementations, not necessarily missing features:

- Fresh and upgrade PostgreSQL migrations; concurrent booking overlap and lifecycle transitions; checkout replay/rollback; archival and retained history.
- Actual reset-email inbox delivery, sender configuration, shared keys, secure deployed cookies and throttle/proxy behavior.
- Durable media upload/read/cleanup, restart survival, bucket configuration and any legacy-media migration.
- Database backup restoration, outage recovery, deployed startup/readiness and graceful lifecycle behavior.
- Render build and runtime logs, live application responses and deployed commit identity.

## Merged repairs excluded from the backlog

PRs #86–90 implemented separate image enlargement controls and a native accessible viewer (FE-04/05), submission locks and pending announcements (FE-20), IME-safe chat and deliberate mobile focus (FE-22), recovery layouts (FE-03), labeled mobile tables (FE-15), shared role navigation/mobile wrapping (FE-01/14), grouped currency display (FE-12), brand fallbacks (FE-11), customer-aware public navigation (FE-17), public portfolio media and exact-product continuation (FE-18), and shared HTML error recovery (FE-19). Remaining form-state/context preservation should be assessed during browser acceptance.

Earlier main already contains booking/schedule/lifecycle safeguards (BE-01/02/03, FE-06/07), transactional checkout/cart/history/fulfillment and eligibility checks (BE-04–08), durable recovery and secure configuration (BE-11/12), durable media and bounded uploads (BE-13/14), historical snapshots and archival (BE-15), review integrity (BE-20), migrations/persistence guards (BE-21), duplicate cleanup (BE-24), and location-first discovery/auth return context (FE-08/09).

## Decisions requiring product input

Online payments are optional under the current collection policy. Email/barber verification, review moderation, remembered locality and realtime unread chat are not automatically missing requirements. Canonical geography and approved legal/support content need explicit definitions.

## Execution order

1. Restore and verify the Render deployment.
2. Resolve PR #91 browser failures; merge only after all required CI is green.
3. Implement validation, exact money and appointment-time semantics in independently reviewable branches.
4. Finish storage/query/lifecycle/security work.
5. Complete media, content and cross-page browser acceptance.

For each batch: branch from current main, implement and verify, push, wait for CI to pass, merge, then begin the next batch.
