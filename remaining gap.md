# Customer journey improvements — 9 October 2026

Sequential workflow: branch, CI on the final head, merge, then verify Render before the next batch.

1. **Separate galleries:** implemented in this batch. Public Handwork and Products use separate server-rendered views; booking pages contain hairstyle cards and a link to the product catalogue. Browser coverage checks switching and independent pagination.
2. **Booking hairstyle photo:** pending. Persist the customer's selected hairstyle photo with the booking and show it in customer history and barber acceptance cards. Retain historical media references.
3. **Cart stock reservations:** pending. The owner confirmed Buy adds to cart, not checkout. Reserve one unit on each accepted addition, release on removal or expiry, and consume reservations at checkout without double deduction. Include concurrent stock protection, clear expiry feedback and database migration.
4. **Chat header photos:** pending. Show the conversation partner's profile picture above their name for both roles, with a fallback.

Email configuration was completed on 9 October: Render deployment `dep-db4b1t59fdbs73bda4i0` was live and logged `password-recovery email worker configured`. Real-inbox acceptance remains unconfirmed.

---

# Remaining implementation gaps

Updated: 8 October 2026. Baseline: main at `dfd5bfb3a6cdc6ae2a02897b7c74ca4b8c725e83`, including merged PR #115. The original scoped implementation gaps are merged. PR #115 passed verify, PostgreSQL and browser CI at final head `bcf7934dd2a6efd84604582b51919c2dbee6f493`. Production acceptance remains separate.

## Deployment checkpoint

PR #115's merge `dfd5bfb3a6cdc6ae2a02897b7c74ca4b8c725e83` deployed successfully as `dep-db3uo68jo6nc73bhlhq0` at 19:19 UTC on 8 October 2026. Render reports **live**, and the new instance's logs record HTTP 200 responses for `/` after startup. Public URL: https://degosalon.onrender.com.

Production acceptance is partially verified through the connected Render and Supabase tools. Render service `srv-db0v17vavr4c739fir6g` in **My Workspace** (`tea-dav5lgvpn0mc73a8ki50`) deploys `main`. Supabase project `myllzicsekynuvopurly` is reachable and reported ACTIVE_HEALTHY. The media bucket is public with a 50 MiB limit and the expected JPEG/PNG/GIF/WebP/MP4/WebM/QuickTime allowlist. This does not certify backup restoration, real-inbox delivery or provider outage recovery.

The repository Blueprint builds the frontend, app, migration CLI, media CLI and audit export CLI, then starts `./migrate && ./media -create-bucket && ./app`. Check actual service commands against this configuration, then inspect build/runtime logs and verify PostgreSQL connectivity, migrations, Supabase bucket access, production admin configuration, secure cookies and SMTP configuration. Do not record a successful deployment until the deployed commit and live HTTP responses have been checked.

### Confirmed production follow-ups

- **Recovery email:** Brevo HTTPS transport is implemented alongside SMTP, with bounded requests/receipts, redirect refusal, sanitized errors and fake-provider tests. The owner selected Brevo and reports adding `BREVO_API_KEY` in Render. Configure `BREVO_FROM` with the exact verified sender and confirm a persistent `RESET_MAIL_KEY` (at least 32 random characters) without rotating an existing key. Save/redeploy, then verify delivery to an authorized real inbox and one-time password reset. Provider credentials, sender configuration and inbox delivery have not been independently verified. Do not commit credentials.
- **Service configuration drift:** the actual Render health check is `/`, while the Blueprint specifies `/health/ready`. The actual build command omits the audit CLI present in the Blueprint. Align the existing service's settings; editing `render.yaml` alone does not update a manually configured service.
- **Verification limits:** direct HTTP checks from the review environment were blocked by a proxy connection timeout. Render deployment status and server-side HTTP logs are provider evidence, not a direct browser acceptance test.
- **Manual/provider acceptance:** real-device/screen-reader checks, durable upload/restart/cleanup, canonical-origin inspection and backup/restore/outage drills remain open. Use a disposable restored database for recovery drills.

## Reconciled implementation status

| Gap IDs | Remaining work | Completion target |
| --- | --- | --- |
| FE-02 | Implemented and merged in PR #103 | Shared page/panel/form/field/control/action primitives now serve all template families; barber and admin archive confirmations share a CSRF-preserving partial. See `ui-primitives.md`. Browser acceptance is covered by FE-26 below. |
| FE-21 | Implemented and merged in PR #104 | Audited all template-rendered error forms, shared server summaries and no-script form associations; associated remaining field/upload help and repaired native select feedback clearing. See `form-feedback-audit.md`. Browser acceptance is covered by FE-26 below. |
| FE-24 | Implemented and merged in PR #105 | Owned responsive thumbnails, srcset/sizes, reserved slots, eager responsive heroes, original enlargement and deferred videos; measured fixture/browser budgets are recorded in `media-performance.md`. Real deployed-device performance remains production acceptance. |
| FE-25 | Implemented and merged in PR #106; deployed-origin acceptance pending | Support publishes owner-approved 08128162561 and 09164226427. Privacy/terms remain explicit placeholders by owner instruction; they no longer require replacement for this scope. Trusted-origin metadata is covered locally; verify the actual deployed canonical origin during live HTTP acceptance. |
| BE-18 | Implemented and merged through #114 | Discovery is scoped and paginated with page-batched summaries; messages are scoped and review aggregates use SQL. Customer catalogue paging merged in #110. Conversation lists now use joined participant summaries and cursor pages; threads load up to 100 messages, with explicit older/newer navigation. Conversation dashboard counts use SQL aggregates. Conversation paging merged in #111. Admin account pagination merged in #112. Admin account pages use bounded SQL search and page-batched counts; admin dashboard totals use SQL aggregates. Inventory and portfolio pages merged in #113. PR #114 adds joined booking/cart reads, booking and fulfillment filters, paged order/admin profile lists, bounded upcoming previews, and SQL dashboard/cart aggregates. Full-cart transactional checkout and single-order detail remain deliberately scoped to that complete entity. See `query-bounds-audit.md`. |
| BE-22 | Implemented and merged in #108/#109 | Structured JSON request logs use generated correlation IDs and registered route patterns. Private append-only admin audit attempts precede authenticated mutations; completion events record HTTP outcomes. Bounded operator export is available via cmd/audit. Live backup/outage/shutdown acceptance remains separate. |
| BE-23 | Implemented and merged in PR #115 | Browser journeys now cover recovery/session invalidation, booking outcomes, schedule conflicts, reviews, checkout replay/ownership, fulfillment/restocking and account lifecycle. Go/PostgreSQL/security coverage remains in place. See `browser-acceptance.md`. |
| FE-26 | Implemented and merged in PR #115 | All 46 full-page templates are exercised at desktop/320px/200% zoom-equivalent reflow, with axe, keyboard/focus, target-size, long-content and broken-media/touch checks. Populated-view findings are repaired and screenshots retained. Automated acceptance limits and live-device follow-up are explicit in `browser-acceptance.md`. |

## PR #85 reconciliation

PR #85's image navigation and native dialog implementation is superseded by merged PRs #86 and #91. The current browser suite retains its keyboard isolation, Escape/focus return and selection checks. Reconciliation extends that suite with close/backdrop dismissal, image-click persistence, and a real uploaded-service journey verifying separate selection/enlargement plus server-rendered summaries and selection without JavaScript. The older duplicate fixture/configuration should not be merged. Reconciliation PR #107 passed verify/PostgreSQL/browser CI and merged; #85 is closed as superseded.

## Reconciled messaging and browser implementation

[PR #91](https://github.com/OG-Ogar/degosalon/pull/91), `feat/frontend-messaging-browser-verification`, merged on 7 October 2026. Final checked head: `8e53fb81095ca6e81ab66301bc900242311b3851`. The [final PR CI run](https://github.com/OG-Ogar/degosalon/actions/runs/37597022175) passed `verify`, `postgres` and `browser`; CodeRabbit also reported success. Earlier failed runs are superseded by this final head.

FE-13's shared messaging stylesheet and FE-23's explicit manual-refresh contract are implemented on main. Both roles load `/static/messages.css`; conversation pages explain that incoming messages appear after refresh/send and expose named refresh links. Polling, streaming and unread state remain optional product work. FE-21's remaining-form/server-error audit is merged in PR #104; browser acceptance is covered under FE-26. Browser CI, broader BE-23 journeys and the FE-26 page matrix are implemented; dedicated security coverage is merged. The merged acceptance matrix covers all 46 full-page templates; it does not certify production, every possible application state or physical-device accessibility.

## Production acceptance still outstanding

These are verification tasks for existing implementations, not necessarily missing features:

- Fresh and upgrade PostgreSQL migrations; concurrent booking overlap and lifecycle transitions; checkout replay/rollback; archival and retained history.
- Actual reset-email inbox delivery, sender configuration, shared keys, secure deployed cookies and throttle/proxy behavior.
- Durable media upload/read/cleanup, restart survival, bucket configuration and any legacy-media migration.
- Database backup restoration, outage recovery, deployed startup/readiness and graceful lifecycle behavior.
- Render build and runtime logs, live application responses and deployed commit identity.

## Merged repairs excluded from the backlog

BE-13/14's media completion checks reject cancellation or response-body read failures even when the provider returned success headers, for both uploads and deletes. Generic retry errors retain provider-detail secrecy; bounded response draining remains in place. Deterministic success-header interruption tests and repeated deadline/race checks cover this boundary. Actual provider durability and outage recovery remain production acceptance work.

BE-18's discovery batch uses canonical SQL location predicates, a separate filtered count and one bounded card query with page-scoped service/review aggregates. Public/customer discovery uses 24-shop ID cursor pages with next/previous/first navigation; aliases and whole-state broadening retain their semantics. Public cards do not select password hashes, account emails or phone numbers. Cursor context survives profile, availability and auth return links; stale empty pages recover to the first page of the selected location. Composite/covering indexes support the filters, cursor and compact aggregate values. Memory/PostgreSQL tests cover paging in both directions, summaries, hidden/archived/suspended exclusion, insert/archive stability, unknown legacy areas and cancellation; driver tests bound query count and preserve operational errors. The remaining list/thread bounds are completed through #110–114.

FE-16's shared presentation catalog and badge partial cover appointment, barber-account, order, item fulfillment and catalogue availability states. Labels, descriptions and five visual treatments are centralized; unknown or cross-domain values render a neutral Unknown status and never become CSS classes. All current status views use the shared component. Booking details now describe the stored current state instead of always promising pending approval, including when revisiting rejected/cancelled/completed appointments. A real booking/cancellation browser journey retains the revisited status check and fixes the booking-details theme contrast it exposed. Persisted lifecycle values and mutation rules remain unchanged.

FE-25's implementation adds shared Help/Support/Privacy/Terms destinations and footer links across template-rendered page families. Content uses the requested, explicit placeholders, with no fabricated contact or legal agreement; placeholder pages send noindex/follow. Help contains current app instructions. Home, discovery, public shop profiles and content pages share descriptions, Open Graph/Twitter metadata and absolute canonical URLs from the configured HTTPS SITE_URL/Render origin; request headers and discovery/session query parameters never determine the canonical URL. The owner has since approved support contacts 08128162561 and 09164226427 and requested that privacy/terms remain placeholders. Final legal replacement is outside this scope; actual deployed-origin verification remains acceptance work.

FE-10/BE-19 uses Nigeria's 36 states, FCT and 774 LGAs/area councils, with stable application IDs, parent-bound choices and explicit legacy aliases. Barber registration/self-edit/admin forms require a valid pair; public/customer discovery accepts both IDs and existing name-based links, offers all locations even where no shops exist, and retains whole-state broadening. Native grouped selects work without JavaScript; the enhanced controls limit LGAs to the selected state and clear an incompatible choice. The migration normalizes/backfills recognized active/suspended records, preserves unknown legacy areas without guessing, and leaves archived historical records untouched. Private RLS-enabled catalogs and foreign keys retain matching parent relationships. [Data sources and adaptation notes](internal/geography/README.md) document naming changes and licensing. SQL discovery scoping and pagination merged in #102.

BE-23's direct password-security coverage checks independent PBKDF2-SHA256 vectors, multiblock derivation, fresh salts, exact password verification and malformed digest handling. Digest verification rejects oversized encodings, excessive iteration counts and incorrect salt/key lengths before hashing; existing application-generated digests retain their format and iteration count. A fuzz target covers malformed input robustness. Broader browser journeys are covered in the acceptance batch.

BE-09/10's [caller audit](storage-caller-audit.md) confirms active request paths use checked context-aware storage APIs. Five remaining affected-row error omissions are repaired, with driver fault-injection coverage of updates, transitions and checkout rollback. Bounded detached media reconciliation remains intentional. Provider outage/pooler cancellation still needs production acceptance; worker shutdown merged in #108.

BE-16 uses an explicit NGN two-decimal contract matching existing PostgreSQL numeric(12,2) columns: nonnegative amounts up to ₦9,999,999,999.99, with excess submitted precision rejected rather than silently rounded. All cart/checkout multiplication and addition use bounded integer kobo. Existing float64 model fields remain compatibility adapters for SQL scans and templates, not business arithmetic. Product/service/order writes validate the contract; order-item subtotals must equal exact unit-price multiplication. Oversized carts remain editable with checkout disabled; checkout independently rejects them before stock/cart/order mutation. PostgreSQL CI exercises fractional totals, overflow rollback and retry-key reuse after correction.

BE-17 compares open appointments against one request-time instant using the recorded timezone, accepting both legacy HH:MM and PostgreSQL HH:MM:SS values. Elapsed or unverifiable open appointments appear in a separate Needs follow-up section with message/outcome actions; page reads never infer no-shows or mutate lifecycle status. Customer upcoming lists and both dashboards exclude elapsed appointments; pending counters still count all pending requests. Legacy rows without timezone snapshots use the configured legacy booking timezone.

BE-19's shared customer/barber identity parsers validate bounded international names, email syntax and phone digit/format limits; barber forms also require bounded shop/address/state/area fields. These parsers cover customer/barber registration and profile edits plus administrator barber creation/update. Invalid submissions are rejected before account/media mutation, with values retained. Canonical geography is now handled by the separate FE-10/BE-19 catalog and migration described above.

PRs #86–90 implemented separate image enlargement controls and a native accessible viewer (FE-04/05), submission locks and pending announcements (FE-20), IME-safe chat and deliberate mobile focus (FE-22), recovery layouts (FE-03), labeled mobile tables (FE-15), shared role navigation/mobile wrapping (FE-01/14), grouped currency display (FE-12), brand fallbacks (FE-11), customer-aware public navigation (FE-17), public portfolio media and exact-product continuation (FE-18), and shared HTML error recovery (FE-19). Browser acceptance retains form-state/context and native navigation checks.

Earlier main already contains booking/schedule/lifecycle safeguards (BE-01/02/03, FE-06/07), transactional checkout/cart/history/fulfillment and eligibility checks (BE-04–08), durable recovery and secure configuration (BE-11/12), durable media and bounded uploads (BE-13/14), historical snapshots and archival (BE-15), review integrity (BE-20), migrations/persistence guards (BE-21), duplicate cleanup (BE-24), and location-first discovery/auth return context (FE-08/09).

## Decisions requiring product input

The owner confirmed on 8 October 2026: finish the agreed gap scope first and defer optional features. Realtime/unread chat, online payments, identity verification, review moderation, remembered locality, customer deletion/retention extensions and administrator recovery/MFA are not part of the current implementation sequence.

Online payments are optional under the current collection policy. Email/barber verification, review moderation, remembered locality and realtime unread chat are not automatically missing requirements. Product decisions: use Nigeria states/FCT and official LGAs/area councils for geography; use clearly labeled placeholders for support/help/privacy/terms until final content is approved.

## Sequential execution record

1. Reconciled #85 with #107 and closed the superseded PR.
2. Completed #102, waited for final-head CI and merged discovery pagination.
3. Merged #108/#109 for lifecycle, health, correlated logging and administrator audit.
4. Merged #110–114 for catalogue, messaging, accounts, portfolio, booking/order/cart bounds and aggregates.
5. Merged #115 after all checks passed. Fixed its throttle test by separating the 20-attempt peer budget from the five-attempt account budget; the browser suite now checks both independently without weakening production protection.
6. Verified PR #115's live deployment through Render status and new-instance HTTP logs. Remaining provider/recovery and direct browser acceptance tasks are listed above.

Every merged batch passed verify, PostgreSQL and browser CI on its final pushed head before merge. Source implementation and live operational acceptance are distinct; no live backup restoration or inbox delivery is claimed here. Deployment evidence is recorded in the checkpoint above.
