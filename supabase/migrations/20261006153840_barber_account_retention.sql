-- Retain accounts and their historical references after administrator archival.
alter table public.barbers drop constraint barbers_status_check;
alter table public.barbers add constraint barbers_status_check
 check (status in ('Active','Suspended','Archived'));

create function degosalon_internal.guard_archived_barber() returns trigger
 language plpgsql set search_path = '' as $$
begin
 if old.status = 'Archived' then
  raise exception 'Archived barber accounts cannot be changed' using errcode = '55000';
 end if;
 return new;
end;
$$;
revoke all on function degosalon_internal.guard_archived_barber() from public;
create trigger guard_archived_barber before update on public.barbers
 for each row execute function degosalon_internal.guard_archived_barber();
