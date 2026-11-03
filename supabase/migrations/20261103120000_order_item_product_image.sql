-- Snapshot the product photo with each order line so historical orders keep
-- showing the photo the customer bought, even after the catalogue changes.
alter table public.product_order_items
 add column product_image text not null default '';

-- Existing orders can only recover the photo still retained at migration time.
update public.product_order_items i set product_image=coalesce(p.image,'')
from public.products p
where p.id=i.product_id;

alter table public.product_order_items alter column product_image drop default;

-- Keep the photo immutable once the order line exists, like the other snapshots.
create or replace function degosalon_internal.protect_order_details() returns trigger
language plpgsql set search_path=pg_catalog as $$
begin
 if row(new.order_id,new.product_id,new.barber_id,new.product_name,new.price,new.quantity,new.subtotal,
        new.seller_name,new.seller_shop_name,new.seller_shop_address,new.seller_phone,new.product_category,new.product_description,new.product_image)
    is distinct from
    row(old.order_id,old.product_id,old.barber_id,old.product_name,old.price,old.quantity,old.subtotal,
        old.seller_name,old.seller_shop_name,old.seller_shop_address,old.seller_phone,old.product_category,old.product_description,old.product_image) then
   raise exception using errcode='55000', message='Order details are immutable';
 end if;
 return new;
end $$;
revoke all on function degosalon_internal.protect_order_details() from public;
