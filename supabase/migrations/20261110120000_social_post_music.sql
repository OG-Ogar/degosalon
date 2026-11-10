-- Attach an optional bundled soundtrack to a photo/video post.
alter table public.social_posts
    add column music text not null default ''
    check (music = '' or music ~ '^[a-z0-9-]{1,40}$');
