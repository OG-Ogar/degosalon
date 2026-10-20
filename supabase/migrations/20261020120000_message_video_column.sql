-- Chat attachments live in their own columns. The baseline fixture created
-- image/audio by hand, but no migration ever added them, so a database built
-- only from the ledger could not store a photo or voice note. Video is new.
-- Each statement is idempotent so both kinds of database converge.
alter table public.messages
  add column if not exists image text not null default '';

alter table public.messages
  add column if not exists audio text not null default '';

alter table public.messages
  add column if not exists video text not null default '';
