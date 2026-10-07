# Form feedback audit (FE-21)

All template-rendered `.Error` summaries use the shared `form-error` partial with a stable ID, alert semantics and readable error styling. Their POST forms associate the summary server-side through `aria-describedby`; no JavaScript is required. Customer cart `.Problem` summaries follow the same contract for checkout and quantity changes. Administrator archive conflicts pass their error into the shared confirmation view. Barber catalogue confirmations have no error state and do not reference a missing summary.

Covered families: all role sign-ins, customer/barber registration, both recovery flows, customer/barber profile editors, product creation/editing, both service editors, schedule, booking/reviews, both messaging composers, administrator account creation/editing, both site-image forms, account archival and cart/checkout. Generic request failures retain their HTML recovery pages and alert text.

Server messages describe rejected submissions at form level. They do not identify a field reliably, so the presentation does not mark arbitrary fields invalid. Native browser validation supplies field-specific `aria-invalid` and associated live notes. Both input and change events clear resolved native errors; existing hint IDs remain intact when notes are added. No script attempts to infer errors from another form's parent or replace an existing description.

Associated remaining help: registration photo instructions, recovery email instructions, product stock/photo instructions and customer profile-photo instructions. Existing password, geography, service category/media, barber profile media and schedule time-off associations are preserved. Message character counters remain associated with composers.

Verification: real rejected registration and sign-in submissions with JavaScript on/off, retained registration values, referenced help IDs and corrected native-select feedback. Existing browser journeys cover the migrated forms, including accessibility scans. Production inbox/provider acceptance and comprehensive FE-26 visual/keyboard/zoom acceptance remain separate.
