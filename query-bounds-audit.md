# Request query bounds

The BE-18 audit replaces growing request lists with SQL cursor pages. Default pages hold 24 records; message histories hold at most 100. Queries fetch one extra record for navigation. IDs define stable boundaries, and deleting the boundary record does not shift the cursor. Empty stale pages return to the first page. Filters apply before SQL ordering and limits.

| Request family | Storage boundary |
| --- | --- |
| Discovery | Location-scoped count and one 24-shop query with page-batched summaries |
| Customer catalogue | One 24-product query with eligible seller join; exact-product continuation |
| Seller inventory | One 24-service/product query scoped to the authenticated barber |
| Shop profiles | Independent 24-service/product/review queries; full SQL rating aggregate; selected service resolved separately when outside the page |
| Conversation lists | One 24-conversation query with participant summaries and counts |
| Message history | One owner-pair query with 100 messages; displayed chronologically |
| Admin accounts | One 24-account query with full-list search and page-batched counts; no password hashes |
| Appointment lists | One 24-booking query scoped by authenticated owner, with joined legacy display metadata and group filter |
| Admin shop details | One shop lookup and independent 24-service/booking pages |
| Customer dashboard | SQL totals and at most six upcoming appointment cards; 24 orders per page |
| Barber dashboard | SQL totals and 24 seller order items, with a fulfillment filter |
| Cart | One joined 24-line query and a separate exact whole-cart aggregate; no product/seller queries per line |

Dashboard totals and preview classification use the same request-time instant. Open appointments with an elapsed start or an invalid timezone stay in Needs follow-up. Pending totals include all pending appointments. Reads do not infer outcomes or mutate lifecycle state. PostgreSQL validates the recorded timezone against its timezone catalog before conversion; memory mode uses Go's timezone database, with the configured legacy timezone for missing snapshots.

Complete-entity operations intentionally remain scoped to the whole entity: transactional checkout reads the entire owned cart, and an authorized order receipt reads every item of that one order. Pagination never changes checkout totals or silently omits purchase lines. Availability reads are scoped to one barber and date. Legacy unbounded compatibility APIs remain for tests and internal operations; active growing-list render paths use the bounded APIs above.

Memory/PostgreSQL tests cover forward/backward navigation, ownership, aggregates beyond page boundaries, timezone follow-up, historical metadata, hidden inventory and monetary overflow. Driver tests check SQL limits and preserve outages. Native browser journeys retain filter/navigation, whole-cart totals and checkout across pages. PostgreSQL and full browser CI are required before merge.
