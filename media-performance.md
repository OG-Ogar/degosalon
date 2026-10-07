# Responsive media delivery (FE-24)

Owned JPEG/PNG uploads use `/media/preview` candidates at 320, 640, 960 and 1600 pixels. Previews are fixed 4:3 center crops with truthful width descriptors and JPEG quality 78. Templates reserve dimensions and declare sizes; avatar slots declare 96px, cards use responsive column sizes, and decorative home/customer-dashboard imagery uses 100vw with eager/high-priority loading. Other images load lazily. Enlargement opens the original source, preserving full framing and detail. GIF/WebP references retain original delivery to preserve animation. Portfolio videos require playback and use `preload="none"`.

The preview endpoint accepts only known local upload paths or owned public Supabase images. Foreign URLs, traversal, arbitrary widths and video URLs are rejected before reading. Provider reads send no credentials, refuse redirects, and cap duration and bytes. Source headers are bounded to 10,000px per edge and 16 million pixels before decoding; payloads cap at 5 MiB. Two transforms run at once with bounded waiting; a 16 MiB/128-entry cache expires previews after five minutes. Previews need no new persistent objects, schema or cleanup ledger. Original archival/media-reference rules remain unchanged.

## Budgets and measurements

The deterministic 2400×1800 representative JPEG fixture is 201,093 bytes. Measured previews: 4,572 bytes (320px), 12,985 (640px), 25,044 (960px), 48,389 (1600px). Tests enforce respective byte ceilings of 30, 120, 240 and 500 KiB, correct dimensions and improvement over the original.

Browser CI measures a real uploaded hero at 320px, checks one selected candidate, a 30 KiB image budget, CLS ≤0.1 and a generous 5-second local-CI LCP ceiling. It retains performance JSON and a screenshot. Desktop verification checks selection of the 1600px candidate; naturalWidth is density-corrected by the browser and is not the encoded resource width. Browser uploads use a temporary server directory to preserve development media.

These fixture/CI measurements establish regression budgets. Production acceptance must sample real media and devices (including cold provider/cache paths), targeting p75 LCP ≤2.5 seconds, CLS ≤0.1 and INP ≤200ms. Production performance is not certified by a local fixture. Increase budgets only with recorded measurements and a justified visual requirement.

Browser-server environment explicitly disables database, object-storage and SMTP connections, and uses `https://example.test` as its metadata origin. This prevents inherited developer/provider settings from making test uploads or email attempts against external services.
