-- Retain seller/product descriptions independently of future catalogue edits.
alter table public.product_order_items
 add column seller_name text not null default '',
 add column seller_shop_name text not null default '',
 add column seller_shop_address text not null default '',
 add column seller_phone text not null default '',
 add column product_category text not null default '',
 add column product_description text not null default '';

-- Existing orders can only recover metadata still retained at migration time.
update public.product_order_items i set
 seller_name=b.full_name,
 seller_shop_name=b.shop_name,
 seller_shop_address=b.shop_address,
 seller_phone=b.phone_number,
 product_category=p.category,
 product_description=p.description
from public.barbers b, public.products p
where b.id=i.barber_id and p.id=i.product_id;

-- New writes must supply snapshots rather than silently omitting them.
alter table public.product_order_items
 alter column seller_name drop default,
 alter column seller_shop_name drop default,
 alter column seller_shop_address drop default,
 alter column seller_phone drop default,
 alter column product_category drop default,
 alter column product_description drop default;

create function degosalon_internal.protect_order_details() returns trigger
language plpgsql set search_path=pg_catalog as $$
begin
 if row(new.order_id,new.product_id,new.barber_id,new.product_name,new.price,new.quantity,new.subtotal,
        new.seller_name,new.seller_shop_name,new.seller_shop_address,new.seller_phone,new.product_category,new.product_description)
    is distinct from
    row(old.order_id,old.product_id,old.barber_id,old.product_name,old.price,old.quantity,old.subtotal,
        old.seller_name,old.seller_shop_name,old.seller_shop_address,old.seller_phone,old.product_category,old.product_description) then
   raise exception using errcode='55000', message='Order details are immutable';
 end if;
 return new;
end $$;
revoke all on function degosalon_internal.protect_order_details() from public;
create trigger protect_order_details before update on public.product_order_items
for each row execute function degosalon_internal.protect_order_details();
