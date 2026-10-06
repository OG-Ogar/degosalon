create table degosalon_internal.site_media (
    name text primary key check (name in ('landing_image','customer_landing_image')),
    image_url text not null check (length(image_url) between 1 and 2048)
);
revoke all on degosalon_internal.site_media from public;
alter table degosalon_internal.site_media enable row level security;
