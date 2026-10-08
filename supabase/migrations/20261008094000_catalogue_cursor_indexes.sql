-- Scope available customer catalogue pages before loading descriptions/media.
create index products_catalogue_cursor on public.products(id) where is_available and not is_archived;
create index products_catalogue_seller_cursor on public.products(barber_id,id) where is_available and not is_archived;
