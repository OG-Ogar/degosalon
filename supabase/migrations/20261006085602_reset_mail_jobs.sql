-- Ciphertext is server-only. Token expiry/consumption removes the queued link.
create table degosalon_internal.reset_mail_jobs (
    token_digest text primary key references degosalon_internal.password_resets(token_digest) on delete cascade,
    payload bytea not null check (octet_length(payload) between 29 and 4096),
    attempts integer not null default 0 check (attempts between 0 and 8),
    next_attempt_at timestamptz not null default now(),
    lease_key text not null default '',
    lease_until timestamptz,
    check ((lease_key = '' and lease_until is null) or (length(lease_key) = 32 and lease_until is not null))
);
create index reset_mail_jobs_due on degosalon_internal.reset_mail_jobs(next_attempt_at) where attempts < 8;
revoke all on degosalon_internal.reset_mail_jobs from public;
alter table degosalon_internal.reset_mail_jobs enable row level security;
