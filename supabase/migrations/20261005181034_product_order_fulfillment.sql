alter table public.product_order_items add column fulfillment_status text not null default 'Pending'
 check(fulfillment_status in ('Pending','Ready','Collected','Cancelled'));
update public.product_order_items i set fulfillment_status=case o.status when 'Cancelled' then 'Cancelled' when 'Completed' then 'Collected' else 'Pending' end
 from public.product_orders o where o.id=i.order_id;

alter table public.product_orders drop constraint product_orders_status_check;
alter table public.product_orders add constraint product_orders_status_check
 check(status in ('Pending','Ready','Paid','Cancelled','Completed'));
