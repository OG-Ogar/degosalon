create table public.barber_calendars(
 barber_id bigint primary key references public.barbers(id),
 settings jsonb not null check(jsonb_typeof(settings)='object')
);
alter table public.barber_calendars enable row level security;

alter table public.bookings add column booking_timezone text;
