# Shared UI primitives

`static/ui.css` loads after page styles across all template-rendered page families. Existing classes retain page artwork; shared classes define layout, spacing and control behavior:

- `ui-page`: bounded, centered content with narrow-screen gutters and long-text wrapping. Landing artwork and auth shells keep their specialized outer layouts.
- `ui-panel`: shared surface, border, padding and radius.
- `ui-form` and `ui-field`: bounded forms and consistently spaced labeled fields, native inputs, selects, textareas and checkboxes.
- `ui-control` and `ui-actions`: 44px minimum controls and wrapping action groups, with explicit primary and danger treatments.
- `ui-confirmation`: bounded archive content. Both barber catalogue and administrator account confirmations use `confirmation-actions` from `templates/partials/confirmation.html`; their handlers retain action routes, CSRF tokens and domain decisions.

Use these classes when adding pages instead of introducing another role-specific sizing system. Keep labels, required constraints, field help and form error associations in the server template; JavaScript is an enhancement. The FE-21 audit covers remaining feedback associations separately.

Browser coverage exercises product/service creation forms, schedule and profile editors, admin creation and both archive confirmations at 320px, including cancel navigation, a real account archive, accessibility scans and retained screenshots. Full browser acceptance remains FE-26 work.
