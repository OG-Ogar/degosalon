# Browser acceptance coverage

The acceptance suite exercises all 46 full-page HTML templates through 52 route variants, including the shared HTML error response. It uses real registrations, uploads, bookings, purchases and conversations with deliberately long names, addresses, descriptions and messages. Existing focused tests retain native navigation without JavaScript, IME handling, image-dialog keyboard isolation, pagination and form feedback.

## Page matrix

| Family | Covered views |
| --- | --- |
| Public | Home, discovery, populated shop, Help/Support/Privacy/Terms, missing-page recovery |
| Authentication | Customer/barber login, registration, recovery, invalid reset link; admin login |
| Customer | Dashboard, discovery, populated shop/booking form, appointments, confirmation, catalogue, cart, receipt, profile/edit, conversations/thread |
| Barber | Dashboard, appointments, inventory, service create/detail/edit/archive confirmation, product create/edit, profile/edit, schedule, conversations/thread |
| Admin | Dashboard, barber search/list/create/detail/edit/archive confirmation, customers and appointments |

Every matrix route runs at 1280px desktop width, 320px width, and a 200% zoom-equivalent reflow condition: 640×450 CSS pixels at device scale factor 2, representing a 1280×900 physical viewport. This checks enlarged text/reflow through Chromium device metrics; it does not operate the browser's zoom menu or certify a physical device.

Desktop and 320px checks run axe WCAG 2.0/2.1 A/AA rules, horizontal-overflow checks, keyboard focus and visible form-control outlines. Zoom-equivalent checks retain layout, focus and screenshots. Visible enabled buttons, selects, text inputs and textareas must be at least 24×24 CSS pixels; primary shared actions use the existing 44px control height. Native checkbox/radio targets are excluded from the numeric control check and retain associated labels. This is scoped automated acceptance, not blanket WCAG certification.

## Stateful journeys

- Booking acceptance/completion, rejected schedule changes that conflict with appointments, saved breaks reflected in availability, completed-booking review eligibility and duplicate review refusal.
- Checkout retry-key replay, seller ready/collected transitions, cancellation restoring stock, receipt ownership and retained snapshots after shop archival.
- Both roles change passwords, lose previous sessions, reject old credentials and cannot reuse reset links. Browser recovery uses the explicit development-link fixture; Go/PostgreSQL tests separately cover queued delivery and relay behavior. Actual inbox delivery remains production acceptance.
- Administrator suspension invalidates seller access and hides the shop; reactivation restores visibility; archival keeps completed appointment and purchase history.
- Touch-enabled 320px contexts abort image requests across populated public/customer/barber/admin views. Alt text, reserved layout, accessible controls and image-dialog open/close remain usable.
- Existing tests cover 25-shop/catalogue/account pages, 105-message histories, 25 bookings/orders/cart lines, exact selection outside the current service page, and checkout of the whole cart from its second page.

## Repairs retained from acceptance

Populated booking cards, table wrappers, headers, hints, prices and destructive actions now use compatible dark surfaces and readable text. Barber product-form labels retain the role's text color. Form controls, including discovery, use opaque keyboard focus outlines. Public portfolio headings use the page foreground; enlargement controls stay within reserved image containers, and product prices retain a readable layout.

The suite saves screenshots in `test-results/` and a Playwright HTML report. CI uploads both in `frontend-browser-evidence` for 14 days and keeps traces/screenshots on failure. Synthetic test accounts and an isolated upload directory prevent changes to development media. The isolated server trusts only its loopback proxy for synthetic client addresses. Acceptance fixtures use a distinct client from the legacy suite; a browser check still enforces the real registration budget and verifies client isolation. Production throttle settings are unchanged. Browser CI has a 25-minute deadline for the expanded suite, with one worker and no retries.

Real-device, cross-browser, screen-reader and live-provider verification remain separate production acceptance. Owner-approved support contacts and requested legal placeholders remain the product's current content.
