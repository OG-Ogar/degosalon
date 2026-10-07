# Storage caller audit — 7 October 2026

Scope: reachable request handlers, storage compatibility wrappers, account/session/recovery mutations and detached media reconciliation.

All live handler storage reads and mutations use checked context-aware APIs. Registration/profile/admin account writes, catalogue mutations, booking transitions, cart/checkout, discovery, reviews and conversations pass the request context. Legacy background-context/error-discarding wrappers remain compatibility APIs; their existence is not a live request defect. Context-aware list APIs only call memory helpers after confirming database mode is disabled.

Five mutations discarded `RowsAffected()` errors: product/service/barber updates, booking transitions and checkout stock decrements. These now propagate the original operational error before interpreting a row count. Driver fault-injection tests cover all five public APIs; checkout does not commit on that failure.

Detached contexts remain intentional for media retirement scheduling after a committed replacement or request failure, and lease acknowledgments after provider activity. These operations need to persist reconciliation despite request cancellation and still use the five-second database operation bound. Initial upload intents use the request context. Workers inherit their worker context; signal-driven worker cancellation/join belongs to BE-22.

Existing cancellation/deadline tests and PostgreSQL integration cover cancelled requests, pool acquisition, account writes, lifecycle transitions and checkout rollback/replay. Live provider outage and pooler cancellation acceptance remain production verification tasks.
