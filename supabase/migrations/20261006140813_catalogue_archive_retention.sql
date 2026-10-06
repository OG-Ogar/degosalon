-- Catalogue removal retains rows and their media for history and fulfillment.
alter table public.products add column is_archived boolean not null default false,
 add constraint products_archive_unavailable check(not is_archived or not is_available);
alter table public.hairstyles add column is_archived boolean not null default false,
 add constraint hairstyles_archive_unavailable check(not is_archived or not is_available);

create function degosalon_internal.protect_catalogue_archive() returns trigger
language plpgsql set search_path=pg_catalog as $$
begin
 if old.is_archived and not new.is_archived then
   raise exception using errcode='55000',message='Archived catalogue entries cannot be reopened';
 end if;
 return new;
end $$;
revoke all on function degosalon_internal.protect_catalogue_archive() from public;
create trigger protect_product_archive before update on public.products
for each row execute function degosalon_internal.protect_catalogue_archive();
create trigger protect_hairstyle_archive before update on public.hairstyles
for each row execute function degosalon_internal.protect_catalogue_archive();
