alter table public.bookings
  add column if not exists service_image text not null default '';

update public.bookings b
set service_image = coalesce(h.image, '')
from public.hairstyles h
where h.id = b.hairstyle_id
  and b.service_image = '';
