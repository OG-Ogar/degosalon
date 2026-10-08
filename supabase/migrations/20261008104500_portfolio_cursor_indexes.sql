create index hairstyles_owner_cursor on public.hairstyles(barber_id,id) where not is_archived;
create index products_owner_cursor on public.products(barber_id,id) where not is_archived;
create index reviews_owner_cursor on public.reviews(barber_id,id desc);
