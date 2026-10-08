-- Equality filters followed by the stable ID cursor; no skipped-row OFFSET scans.
create index barbers_discovery_state_id on public.barbers(state_id,id) where status='Active';
create index barbers_discovery_area_id on public.barbers(state_id,local_area_id,id) where status='Active';
drop index public.barbers_canonical_location_idx;
-- Page-scoped aggregates can read compact indexed values rather than media/text rows.
create index hairstyles_discovery_summary on public.hairstyles(barber_id,price) where is_available and not is_archived;
create index reviews_discovery_summary on public.reviews(barber_id) include(rating);
