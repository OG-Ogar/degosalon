-- Per-user display preferences (colour theme + accessibility text size), the
-- server source of truth for frontreadme.md §2.3A. Values are allowlisted in
-- Go before they reach here; the check constraints are the second line of
-- defence so a crafted value can never break the layout or a token set.
create table degosalon_internal.user_preferences (
    user_role text not null check (user_role in ('admin','barber','customer')),
    user_id bigint not null check (user_id >= 0),
    theme text not null default 'forest' check (theme in ('forest','midnight','daylight','ocean','stone')),
    text_size text not null default 'standard' check (text_size in ('standard','large','xlarge','xxlarge')),
    notify_alerts boolean not null default false,
    updated_at timestamptz not null default now(),
    primary key (user_role, user_id),
    check ((user_role = 'admin' and user_id = 0) or (user_role <> 'admin' and user_id > 0))
);
revoke all on degosalon_internal.user_preferences from public;
alter table degosalon_internal.user_preferences enable row level security;
