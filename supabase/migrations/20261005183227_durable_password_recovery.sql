create table degosalon_internal.password_resets(
 role text not null check(role in ('barber','customer')),
 user_id bigint not null check(user_id>0),
 token_digest text not null unique check(length(token_digest)=64),
 expires_at timestamptz not null,
 primary key(role,user_id)
);
create index password_resets_expiry_idx on degosalon_internal.password_resets(expires_at);
revoke all on degosalon_internal.password_resets from public;
alter table degosalon_internal.password_resets enable row level security;
