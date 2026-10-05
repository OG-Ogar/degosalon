alter table public.bookings
 add column barber_name text,
 add column shop_name text,
 add column shop_address text,
 add column service_name text,
 add column service_price numeric(12,2),
 add column service_duration integer;
-- Existing appointments adopt the currently retained service/shop metadata.
-- Earlier edits cannot be reconstructed; new bookings preserve their originals.
update public.bookings b set barber_name=a.full_name,shop_name=a.shop_name,shop_address=a.shop_address,
 service_name=h.name,service_price=h.price,service_duration=h.duration
 from public.barbers a,public.hairstyles h where a.id=b.barber_id and h.id=b.hairstyle_id;
alter table public.bookings
 alter column barber_name set not null,
 alter column shop_name set not null,
 alter column shop_address set not null,
 alter column service_name set not null,
 alter column service_price set not null,
 alter column service_duration set not null,
 add constraint bookings_snapshot_price check(service_price >= 0),
 add constraint bookings_snapshot_duration check(service_duration > 0);
