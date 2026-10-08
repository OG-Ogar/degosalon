create index messages_pair_id_cursor on public.messages(barber_id,customer_id,id desc);
create index messages_customer_activity on public.messages(customer_id,barber_id,id desc);
