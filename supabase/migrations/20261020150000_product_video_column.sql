-- A product can carry a short clip, exactly like a hairstyle. The column is
-- added idempotently so a database built from the reference schema and a
-- database built only from this ledger converge on the same shape.
alter table public.products
  add column if not exists video text not null default '';

-- The media reference guard now watches the clip as well as the photo, so a
-- retired object cannot be reintroduced through the new column.
drop trigger if exists guard_product_media on public.products;
create trigger guard_product_media before insert or update of image,video on public.products
    for each row execute function degosalon_internal.guard_media_reference('image','video');

create index if not exists products_video_media on public.products(video) where video<>'';

-- Clip references that already exist become known media objects.
insert into degosalon_internal.media_objects(uri)
select distinct video from public.products
 where video ~ '^https://[^/?#]+/storage/v1/object/public/degosalon-media/product-videos/([a-f0-9]{32}|[0-9]{10,20})\.(mp4|webm|mov|m4v|3gp)$'
on conflict do nothing;
