create table degosalon_internal.auth_throttles(
 key_digest text primary key check(length(key_digest)=64),
 attempts integer not null check(attempts>0),
 expires_at timestamptz not null
);
create index auth_throttles_expiry_idx on degosalon_internal.auth_throttles(expires_at);
revoke all on degosalon_internal.auth_throttles from public;
alter table degosalon_internal.auth_throttles enable row level security;
