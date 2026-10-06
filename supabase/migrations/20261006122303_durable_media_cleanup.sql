-- Intent is committed before upload. Tombstones prevent resurrection during a
-- remote delete and remain available to retry late/uncertain provider outcomes.
create table degosalon_internal.media_objects (
    uri text primary key check (length(uri) between 1 and 2048),
    state text not null default 'pending' check (state in ('pending','active','deleting','deleted')),
    next_check_at timestamptz not null default now() + interval '1 hour',
    lease_key text not null default '',
    attempts integer not null default 0 check (attempts between 0 and 30),
    check (lease_key = '' or (state = 'deleting' and lease_key ~ '^[a-f0-9]{32}$'))
);
alter table degosalon_internal.media_objects enable row level security;
revoke all on degosalon_internal.media_objects from public;
create index media_objects_due on degosalon_internal.media_objects(next_check_at,uri);

create function degosalon_internal.guard_media_reference() returns trigger
language plpgsql set search_path = pg_catalog, degosalon_internal as $$
declare reference text; object_state text;
begin
    -- Consistent lock ordering for records with multiple images/videos.
    for reference in select distinct to_jsonb(new)->>column_name
        from unnest(tg_argv) as columns_to_guard(column_name)
        where coalesce(to_jsonb(new)->>column_name,'') <> '' order by 1
    loop
        select state into object_state from degosalon_internal.media_objects
            where uri=reference for update;
        if found and object_state in ('deleting','deleted') then
            raise exception 'media object is retired' using errcode='55000';
        end if;
    end loop;
    return new;
end;
$$;
revoke all on function degosalon_internal.guard_media_reference() from public;
create trigger guard_barber_media before insert or update of profile_image,personal_image on public.barbers
    for each row execute function degosalon_internal.guard_media_reference('profile_image','personal_image');
create trigger guard_customer_media before insert or update of profile_image on public.customers
    for each row execute function degosalon_internal.guard_media_reference('profile_image');
create trigger guard_hairstyle_media before insert or update of image,video on public.hairstyles
    for each row execute function degosalon_internal.guard_media_reference('image','video');
create trigger guard_product_media before insert or update of image on public.products
    for each row execute function degosalon_internal.guard_media_reference('image');
create trigger guard_site_media before insert or update of image_url on degosalon_internal.site_media
    for each row execute function degosalon_internal.guard_media_reference('image_url');

-- Seed previously referenced application objects. The worker additionally
-- requires the exact configured project origin and supported filename type.
insert into degosalon_internal.media_objects(uri)
select distinct uri from (
 select profile_image as uri from public.barbers union all
 select personal_image from public.barbers union all
 select profile_image from public.customers union all
 select image from public.hairstyles union all select video from public.hairstyles union all
 select image from public.products union all
 select image_url from degosalon_internal.site_media
) as references_to_media
where uri ~ '^https://[^/?#]+/storage/v1/object/public/degosalon-media/(hairstyles|hairstyle-videos|products|barbers|customers|site)/([a-f0-9]{32}|[0-9]{10,20})\.(jpg|jpeg|png|gif|webp|mp4|webm|mov)$'
on conflict do nothing;

create index barbers_profile_media on public.barbers(profile_image) where profile_image<>'';
create index barbers_personal_media on public.barbers(personal_image) where personal_image<>'';
create index customers_profile_media on public.customers(profile_image) where profile_image<>'';
create index hairstyles_image_media on public.hairstyles(image) where image<>'';
create index hairstyles_video_media on public.hairstyles(video) where video<>'';
create index products_image_media on public.products(image) where image<>'';
create index site_image_media on degosalon_internal.site_media(image_url);
