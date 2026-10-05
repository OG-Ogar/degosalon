-- Server-only authentication state; never exposed through the public API schema.
create table degosalon_internal.auth_sessions(
 id_digest text primary key check(length(id_digest)=64),
 role text not null check(role in ('admin','barber','customer')),
 user_id bigint not null,
 csrf_token text not null,
 expires_at timestamptz not null,
 check((role='admin' and user_id=0) or (role in ('barber','customer') and user_id>0))
);
create index auth_sessions_owner_idx on degosalon_internal.auth_sessions(role,user_id);
create index auth_sessions_expiry_idx on degosalon_internal.auth_sessions(expires_at);
revoke all on degosalon_internal.auth_sessions from public;
