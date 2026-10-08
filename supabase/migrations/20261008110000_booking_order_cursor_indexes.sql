create index bookings_customer_cursor on public.bookings(customer_id,id desc);
create index bookings_barber_cursor on public.bookings(barber_id,id desc);
create index product_orders_customer_cursor on public.product_orders(customer_id,id desc);
create index product_order_items_barber_cursor on public.product_order_items(barber_id,id desc);
