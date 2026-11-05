alter table public.social_posts
    add column background_color text not null default '#fff0c2'
    check (background_color ~ '^#[0-9a-fA-F]{6}$');
