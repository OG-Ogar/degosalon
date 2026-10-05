# De Go Salon Frontend Modernization Plan

## Purpose

This document is the implementation blueprint for bringing the entire De Go Salon frontend into one professional, clean, responsive visual system before page-by-page implementation begins.

The reference direction is the uploaded Nativity Guard **Dawn Canopy** system: deep forest backgrounds, layered green surfaces, cream typography, restrained warm-gold accents, consistent cards and controls, strong responsive navigation, and clear loading/empty/error states. De Go Salon will adapt that design language to a barber marketplace rather than copy Nativity Guard branding, imagery, policing/SOS concepts, or unrelated product features.

The existing Go server-rendered architecture remains authoritative. The TypeScript foundation introduced in PR #29 remains progressive enhancement. This project is a frontend modernization, not a SPA rewrite.

## Product identity

- Product: **De Go Salon**
- Ecosystem language: **Barber Multiverse**
- Tagline: **Your complete barber ecosystem.**
- Primary roles: customer, barber, administrator.
- Core customer actions: discover nearby barbers, inspect work/services/products, book, manage bookings, message barbers, manage cart/orders and profile.
- Core barber actions: manage profile and portfolio, hairstyles/services, products, bookings, customer conversations and account.
- Core admin actions: monitor the platform, manage barbers/customers/bookings and perform moderation/administrative actions.

## Design goals

The finished interface should feel:

- professional and trustworthy;
- premium without looking expensive or overdecorated;
- warm and personal rather than corporate;
- consistent across public, authentication, customer, barber and admin screens;
- easy to scan and operate on phones;
- clearly De Go Salon, not a visual clone of the reference product.

The reference system's strongest transferable principle is the combination of personal identity and operational clarity: names, barber imagery and human context live beside statuses, bookings, prices, timestamps and actions.

## 1. De Go Salon design tokens

Create one shared semantic token layer rather than continuing to scatter page-specific color values.

Default Forest palette:

```css
:root {
  --void: #091613;
  --base: #10221d;
  --surface: #193129;
  --surface-hi: #254137;
  --border: #355348;
  --border-hi: #527466;
  --ink: #f8f5e9;
  --ink-muted: #c2d4c7;
  --ink-faint: #9db9a8;
  --signal: #f4cb78;
  --signal-ink: #19251c;
  --danger: #ef4444;
  --warning: #f59e0b;
  --success: #22c55e;
  --panel-radius: .875rem;
  --control-radius: .5rem;
}
```

Gold is reserved for the primary action, selected navigation, important links and controlled emphasis. Red is only destructive/error styling. Statuses must never depend on color alone.

### Surface language

Primary cards should use restrained translucent forest surfaces: a 145-degree gradient from approximately `rgba(37,65,55,.55)` to `rgba(25,49,41,.45)`, a quiet border, 14px radius and restrained shadow/blur. Supply opaque fallbacks for reduced transparency and avoid expensive blur on every list row.

Clickable cards may lift approximately 2px on hover/focus. Static cards must not imply clickability.

### Typography

Prefer Inter when it can be served efficiently, otherwise a deliberate system sans-serif stack.

- body: 14–16px;
- operational titles: 20–24px;
- marketing/personal headings: 30–36px;
- card titles: 14–16px semibold;
- metadata: 12–14px;
- identifiers/comparable numbers: tabular or monospace where useful.

Avoid essential 10–11px text.

### Spacing

Use a 4px-based rhythm: 4, 8, 12, 16, 24, 32, 48.

- mobile page padding: 16px;
- wider page padding: 24px;
- card padding: 16–24px;
- reading/form content: normally <=768px;
- marketing content: normally <=1024px;
- dashboards: normally <=1152px.

## 2. Shared application shell

### Desktop

Authenticated customer, barber and admin areas should converge on a shared shell:

- approximately 240px sidebar;
- sticky top bar;
- flexible content region;
- role/workspace label;
- grouped navigation;
- profile/settings/account identity at the bottom;
- gold-tinted active destination;
- muted inactive destinations with clear hover/focus states.

Role navigation must remain role-specific. A customer must not inherit barber/admin destinations simply because the shell is shared.

### Mobile

Below the medium breakpoint:

- sidebar becomes an accessible drawer;
- dim backdrop;
- visible close action;
- approximately 200ms transition;
- close on route navigation;
- Escape support and sensible focus restoration.

Use a bottom navigation only for the four/five most important destinations. Overflow belongs under More rather than a long horizontal menu. Fixed navigation must respect safe areas and never cover forms, toasts or actions.

## 3. Shared frontend primitives

Before broad page migration, establish reusable CSS/HTML conventions and TypeScript behavior for:

1. cards with header/body/footer spacing;
2. primary, secondary, ghost and danger buttons;
3. input/select/textarea/file controls with labels, hints and inline errors;
4. status and priority chips;
5. avatars with initials/image fallback;
6. image/portfolio cards;
7. skeleton, empty, error and unavailable states;
8. toast/flash feedback;
9. responsive dialogs/confirmation surfaces;
10. tabs/segmented controls;
11. KPI/stat cards;
12. breadcrumbs/back context;
13. list/table patterns that become cards on narrow screens.

Default interactive control height should be around 44px; large primary controls 48px. All controls require visible keyboard focus.

## 4. Public experience

### Landing page

Keep the current De Go Salon identity and purpose while aligning it with the new system.

Target composition:

brand header -> atmospheric barber-focused hero -> primary Explore Barbers CTA + secondary barber CTA -> concise value/capability cards -> how it works -> trust/accountability explanation -> final CTA.

Do not add fabricated testimonials, barber counts or booking statistics.

The hero should use the forest/gold atmosphere but barber imagery/content should remain the emotional anchor. Decorative layers must be subtle enough that the page stays fast and readable.

### Explore / public barber discovery

Unify `templates/explore.html`, `templates/explore-barber.html` and authenticated customer discovery visually.

Location selection should feel like the start of discovery rather than a generic form. State/LGA/local-area controls need strong hierarchy, clear selection state and mobile ergonomics.

Results should prioritize:

- shop/barber image;
- shop name and barber identity;
- locality;
- key services/starting price where available;
- rating/review evidence only where backed by real data;
- availability where real;
- clear View Profile / Book action.

## 5. Authentication

Customer, barber and admin authentication pages should share one visual family while clearly naming the role.

Desktop target: calm branded panel + focused authentication panel.

Mobile: one readable column with brand context reduced rather than removed.

Cover:

- customer login/register/forgot/reset;
- barber login/register/forgot/reset;
- admin login.

Requirements:

- preserve existing auth contracts;
- inline validation;
- password visibility from the TypeScript layer;
- useful error copy;
- retain safe entered values after validation failure where the existing server flow permits;
- no fake loading/success states.

## 6. Customer frontend

### Customer dashboard

Turn the dashboard into a personalized home base:

- greeting/identity;
- dominant **Find a Barber** action;
- quick actions;
- upcoming/recent booking context;
- messages/activity;
- relevant cart/order context when present.

Do not overload it with every available feature.

### Find Barbers / barber list

This is a primary product surface and receives high priority.

- professional location filter;
- responsive result cards;
- strong barber/shop imagery;
- location context;
- services/pricing preview where data exists;
- honest empty state;
- easy filter revision;
- visible primary profile/booking action.

### Barber detail

Preserve the corrected contract:

- shop `ProfileImage` is the shop/brand image;
- `PersonalImage` identifies the barber beside their name.

Structure:

hero/shop image -> barber identity/location/status -> primary booking action -> portfolio/hairstyles/services and prices -> products -> availability -> reviews/comments where genuinely supported -> contact/message action.

The page must work cleanly even when optional images or sections are absent.

### Booking flow

Modernize booking selection, confirmation and booking history around a clear step hierarchy.

Selected hairstyle/service, price, barber, date/time and final action should remain visible and understandable. The TypeScript booking summary is enhancement only; server validation remains authoritative.

### Messages

Conversation list and thread should use a consistent messaging pattern:

- clear counterpart identity;
- readable message grouping;
- timestamps as supporting information;
- composer fixed only when it does not obscure content;
- TypeScript Enter-to-send behavior must remain accessible;
- empty/error states.

### Cart/products/orders

Use image-forward product cards, clear quantities/prices, restrained totals hierarchy and obvious checkout/order actions. Never make destructive remove actions visually compete with checkout.

### Customer profile

Use identity/avatar treatment, clearly separated account/profile sections and strong save feedback. File preview behavior should reuse the TypeScript foundation.

## 7. Barber frontend

### Barber dashboard

Treat this as an operations dashboard with personal/shop context.

Recommended order:

- shop/barber identity and greeting;
- today's/upcoming bookings;
- key counts/operational indicators backed by real data;
- quick actions;
- recent booking/customer/message activity;
- profile completeness or actionable setup gaps where they can be calculated truthfully.

### Barber profile/edit profile

Separate identity, shop information, location, shop image and personal barber image clearly. Explain image roles so barbers understand what customers will see.

Keep image preview/resizing feedback and server-side upload validation.

### Hairstyles/services

List cards should emphasize style image, name, price and edit/delete actions. Forms should use the shared field system. Delete confirmation uses the shared danger pattern.

### Products

Apply the same catalog language used on the customer side while exposing management actions appropriate to the barber.

### Bookings

Provide strong status semantics and scan-friendly cards/table behavior. Primary actions depend on current booking state. Completed/cancelled/no-show states should be visually distinct but accessible.

### Messages

Match the customer messaging language so the same conversation feels like one product from both sides.

## 8. Admin frontend

Admin should use the same De Go Salon design system with higher information density, not a separate legacy-looking interface.

### Dashboard

- page title/context;
- real KPI cards;
- recent activity;
- platform health/attention items only when real data exists;
- clear management shortcuts.

### Barber management

Barber list/profile/edit/delete flows need:

- scan-friendly identity/status/location;
- clear moderation/management actions;
- consistent status chips;
- safe destructive confirmation.

### Customers and bookings

Use responsive data presentation: structured tables where space allows, stacked cards on small screens. Preserve real server functionality and pagination/filter behavior where present.

## 9. Feedback and states

Every major page family must deliberately support:

- loading;
- empty;
- validation error;
- server error;
- success;
- disabled;
- unavailable/unsupported;
- destructive confirmation.

Loading should be local to the waiting region. Never introduce a timed full-screen loading overlay.

Flash/toast feedback must be truthful. Consequential errors remain visible inline even if a toast is used.

## 10. Accessibility

Frontend modernization is not complete without:

- semantic heading order;
- keyboard-reachable actions;
- visible focus;
- associated labels/errors/hints;
- alt text for meaningful barber/style/product images;
- decorative layers hidden from assistive technology;
- 4.5:1 normal-text contrast target;
- 3:1 relevant large-text/control target;
- reduced-motion support;
- reduced-transparency/solid fallback;
- dialogs with Escape, focus trap and focus restoration;
- layouts that survive enlarged text.

## 11. Responsive acceptance widths

Explicitly verify representative pages at:

- 360px;
- 390px;
- 768px;
- 1024px;
- 1440px.

No unintended horizontal scrolling. Primary actions must remain reachable. Images need stable aspect ratios and `object-fit` behavior.

## 12. Performance rules

- preserve server-rendered first content;
- keep TypeScript progressive;
- no frontend framework migration merely for styling;
- appropriately size images;
- lazy-load noncritical images where appropriate;
- avoid blur on hundreds of rows;
- keep generated JS deterministic/current;
- do not block navigation behind decorative loading;
- preserve Render's TypeScript + Go build pipeline.

## 13. Implementation phases

### Phase 0 — inventory and regression baseline

Before restyling:

- inventory every template and stylesheet;
- map each route to role, template and CSS;
- record shared patterns and duplication;
- run TypeScript and Go validation;
- identify tests coupled to exact markup/class names;
- preserve screenshots/previews of representative current states when possible.

### Phase 1 — design-system foundation

Implement semantic tokens and shared primitives first. Do not begin by independently rewriting every page stylesheet.

Deliver:

- global tokens;
- typography/spacing;
- cards;
- buttons;
- form fields;
- chips;
- state components;
- responsive shell conventions;
- accessibility/motion rules.

### Phase 2 — public + authentication

Modernize:

- home/landing;
- public exploration/location flow;
- customer auth;
- barber auth;
- admin auth.

Verify before moving on.

### Phase 3 — customer application

Modernize dashboard, discovery, barber detail, booking, booking history, messaging, cart/products/orders and profile.

Discovery + barber detail + booking are highest priority because they form the core customer conversion path.

### Phase 4 — barber application

Modernize dashboard, profile, hairstyles/services, products, bookings and messages.

### Phase 5 — admin application

Modernize dashboard and management surfaces with the same system and denser responsive patterns.

### Phase 6 — consistency/accessibility/performance pass

Audit the whole application for:

- stray legacy colors;
- inconsistent radii/spacing;
- duplicate CSS;
- missing states;
- inaccessible focus/contrast;
- mobile overflow;
- stale class-dependent tests;
- unnecessary JavaScript;
- image sizing;
- role/navigation consistency.

## 14. Engineering constraints

Throughout implementation:

- preserve Go routes, auth, CSRF, database contracts and business behavior;
- do not convert De Go Salon to React/Vue/etc.;
- use `frontend/app.ts` for progressive interaction and compile to `static/app.js`;
- do not hand-edit generated JS when the source belongs in TypeScript;
- avoid broad backend changes in visual PRs;
- keep tests meaningful rather than weakening assertions;
- no fabricated production data;
- no unrelated feature development disguised as UI work;
- no `Co-authored-by` trailers unless explicitly requested.

## 15. Test gates for every implementation batch

Before merging each frontend batch:

```bash
npm install --no-audit --no-fund
npm run typecheck
npm run build
git diff --exit-code -- static/app.js
GOTOOLCHAIN=go1.23.0 go test ./...
```

Also inspect the PR diff for accidental backend changes and verify the final GitHub Actions run on the final head SHA.

## 16. Definition of complete

The frontend modernization is complete only when:

- public, customer, barber and admin pages visibly belong to one De Go Salon system;
- the Forest/cream/gold language is consistent without overwhelming content;
- primary customer discovery and booking flows are especially polished;
- shared components replace avoidable one-off styling;
- mobile layouts are intentional rather than compressed desktop layouts;
- every real navigation destination remains reachable;
- loading/empty/error/success states are coherent;
- accessibility and reduced-motion/transparency behavior are verified;
- TypeScript remains progressive enhancement;
- Go/server behavior remains intact;
- all automated tests and CI are green;
- representative desktop/mobile pages have been manually reviewed.

## Tracking convention

When implementation starts, convert the phase/page items above into checkboxes and update this file in every frontend PR. A page is checked off only after its implementation, responsive review and relevant automated tests pass.


## 17. Phase 0 inventory — completed source audit

This inventory is based on the repository tree on `main` immediately after PR #29. It is the implementation checklist; entries remain unchecked until the page has been restyled, responsively reviewed, and relevant tests pass.

### Public

- [x] `templates/home.html` — landing/home. Forest/cream/gold visual foundation implemented; responsive landing structure complete.
- [ ] `templates/explore.html` — public location/discovery entry.
- [ ] `templates/explore-barber.html` — public barber exploration/result surface.

### Customer authentication

- [ ] `templates/customer/login.html`
- [ ] `templates/customer/register.html`
- [ ] `templates/customer/forgot-password.html`
- [ ] `templates/customer/reset-password.html`

### Customer application

- [ ] `templates/customer/dashboard.html`
- [ ] `templates/customer/barber-list.html`
- [ ] `templates/customer/barber-detail.html`
- [ ] `templates/customer/booking-confirmation.html`
- [ ] `templates/customer/bookings.html`
- [ ] `templates/customer/chats.html`
- [ ] `templates/customer/chat.html`
- [ ] `templates/customer/products.html`
- [ ] `templates/customer/cart.html`
- [ ] `templates/customer/order-confirmation.html`
- [ ] `templates/customer/profile.html`
- [ ] `templates/customer/edit-profile.html`

### Barber authentication

- [ ] `templates/barber/login.html`
- [ ] `templates/barber/register.html`
- [ ] `templates/barber/forgot-password.html`
- [ ] `templates/barber/reset-password.html`

### Barber application

- [ ] `templates/barber/dashboard.html`
- [ ] `templates/barber/profile.html`
- [ ] `templates/barber/edit-profile.html`
- [ ] `templates/barber/hairstyle-list.html`
- [ ] `templates/barber/hairstyle-profile.html`
- [ ] `templates/barber/hairstyle-form.html`
- [ ] `templates/barber/edit-hairstyle.html`
- [ ] `templates/barber/confirm-delete-hairstyle.html`
- [ ] `templates/barber/products.html`
- [ ] `templates/barber/product-form.html`
- [ ] `templates/barber/bookings.html`
- [ ] `templates/barber/chats.html`
- [ ] `templates/barber/chat.html`

### Admin

- [ ] `templates/admin/login.html`
- [ ] `templates/admin/dashboard.html`
- [ ] `templates/admin/barber-list.html`
- [ ] `templates/admin/barber-profile.html`
- [ ] `templates/admin/edit-barber.html`
- [ ] `templates/admin/confirm-delete.html`
- [ ] `templates/admin/customers.html`
- [ ] `templates/admin/bookings.html`

### Existing style/interaction assets to consolidate

- `static/app.css` — global/shared styling; target home for design tokens and reusable primitives.
- `static/app.js` — generated TypeScript output; do not hand-edit.
- `frontend/app.ts` — progressive interaction source.
- `static/login.css` — authentication styling candidate for consolidation.
- `static/barber.css` and `static/barber-profile.css` — barber application styles.
- `static/customer-dashboard.css`, `static/customer-barbers.css`, `static/customer-barber-profile.css`, `static/customer-bookings.css`, `static/customer-booking-confirmation.css`, `static/customer-messages.css`, `static/customer-profile.css` — customer page styles.
- `static/products.css` — catalog/product styling shared across product surfaces.
- `static/booking-images.css` — booking image-specific rules.
- `static/image-viewer.css` and `static/image-viewer.js` — legacy/specialized image-viewer behavior; audit before deciding whether to move behavior into TypeScript.

### Duplication/consolidation targets

The repository currently uses a global stylesheet plus many role/page-specific stylesheets. The modernization should not blindly delete them. First move universal tokens and primitives into `static/app.css`; then reduce page styles to genuinely page-specific layout rules. Customer and barber messaging, profile imagery, product cards, authentication forms, buttons, fields and status treatments should converge on shared patterns.

### Implementation order locked after inventory

1. Design tokens and shared primitives in `static/app.css`.
2. Shared responsive shell conventions.
3. Public landing/explore.
4. Customer/barber/admin authentication.
5. Customer core journey: dashboard -> discovery -> barber detail -> booking.
6. Remaining customer pages.
7. Barber operational pages.
8. Admin pages.
9. Full consistency/accessibility/performance cleanup.

### Phase 0 baseline status

- [x] Repository frontend tree inventoried.
- [x] Public/customer/barber/admin template families identified.
- [x] Existing CSS and JavaScript/TypeScript assets inventoried.
- [x] Consolidation targets documented.
- [ ] Baseline TypeScript build/typecheck verified on this branch.
- [ ] Baseline Go test suite verified on this branch.
- [ ] Representative pre-restyle desktop/mobile screenshots captured where environment permits.

The unchecked baseline verification items must be completed before the first visual implementation PR is merged. They do not block drafting the Phase 1 design-system changes, but they must not be reported as passing without execution evidence.
