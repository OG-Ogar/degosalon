-- Accounts remembered on a browser and WebAuthn passkeys.
--
-- A remembered-device row only tells the login page which account to show. It
-- cannot create a session: the visitor still has to prove ownership with their
-- password or a registered passkey.
create table degosalon_internal.remembered_devices(
 token_digest text primary key check(length(token_digest)=64),
 role text not null check(role in ('barber','customer')),
 user_id bigint not null check(user_id>0),
 expires_at timestamptz not null
);
create index remembered_devices_expiry_idx on degosalon_internal.remembered_devices(expires_at);
revoke all on degosalon_internal.remembered_devices from public;
alter table degosalon_internal.remembered_devices enable row level security;

-- WebAuthn credential records. The public key is safe to store; private keys
-- never leave the authenticator on the visitor's phone or laptop.
create table degosalon_internal.passkeys(
 credential_id text primary key,
 role text not null check(role in ('barber','customer')),
 user_id bigint not null check(user_id>0),
 email text not null,
 display_name text not null,
 credential jsonb not null,
 created_at timestamptz not null default now(),
 last_used_at timestamptz
);
create index passkeys_owner_idx on degosalon_internal.passkeys(role,user_id);
revoke all on degosalon_internal.passkeys from public;
alter table degosalon_internal.passkeys enable row level security;
