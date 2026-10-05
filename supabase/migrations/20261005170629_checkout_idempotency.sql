-- Null keys preserve orders created before replayable checkout.
alter table public.product_orders add column checkout_key text;
alter table public.product_orders add constraint product_orders_checkout_key_format check(checkout_key is null or checkout_key ~ '^[a-f0-9]{32}$');
create unique index product_orders_customer_checkout_key on public.product_orders(customer_id,checkout_key) where checkout_key is not null;
