-- Private, append-only audit trail. No request bodies, secrets or contact data.
create table degosalon_internal.admin_audit_events (
 id bigint generated always as identity primary key,
 request_id text not null check (request_id ~ '^[0-9a-f]{32}$'),
 actor text not null check (length(actor) between 1 and 200),
 action text not null check (action in ('barber.create','barber.update','barber.archive','barber.suspend','barber.activate','site.landing_image','site.customer_landing_image')),
 target_id integer not null check (target_id >= 0),
 phase text not null check (phase in ('started','finished')),
 http_status integer not null,
 created_at timestamptz not null default clock_timestamp(),
 unique (request_id,phase),
 check ((phase='started' and http_status=0) or (phase='finished' and http_status between 200 and 599))
);
alter table degosalon_internal.admin_audit_events enable row level security;
revoke all on degosalon_internal.admin_audit_events from public;
create function degosalon_internal.guard_admin_audit_events() returns trigger
 language plpgsql set search_path = '' as $$
begin
 raise exception 'Administrator audit events are append only' using errcode = '55000';
end;
$$;
revoke all on function degosalon_internal.guard_admin_audit_events() from public;
create trigger guard_admin_audit_events before update or delete on degosalon_internal.admin_audit_events
 for each row execute function degosalon_internal.guard_admin_audit_events();
