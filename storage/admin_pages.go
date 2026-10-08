package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"sort"
	"strings"
)

type AdminCounts struct{ Barbers, ActiveBarbers, Customers, Bookings, PendingBookings int }

func AdminDashboardCounts(ctx context.Context) (AdminCounts, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return AdminCounts{}, err
	}
	if databaseEnabled() {
		var counts AdminCounts
		err := db.QueryRowContext(ctx, `select (select count(*) from public.barbers),(select count(*) from public.barbers where status='Active'),(select count(*) from public.customers),(select count(*) from public.bookings),(select count(*) from public.bookings where status='Pending')`).Scan(&counts.Barbers, &counts.ActiveBarbers, &counts.Customers, &counts.Bookings, &counts.PendingBookings)
		return counts, err
	}
	mu.RLock()
	defer mu.RUnlock()
	counts := AdminCounts{Barbers: len(barbers), Customers: len(customers), Bookings: len(bookings)}
	for _, b := range barbers {
		if b.IsActive() {
			counts.ActiveBarbers++
		}
	}
	for _, b := range bookings {
		if b.Status == models.StatusPending {
			counts.PendingBookings++
		}
	}
	return counts, nil
}

type AdminBarberCard struct {
	models.Barber
	Hairstyles, Bookings int
}
type AdminCustomerCard struct {
	models.Customer
	Bookings int
}

func AdminBarberPage(ctx context.Context, search string, cursor PageCursor) (Page[AdminBarberCard], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[AdminBarberCard]{}, err
	}
	var err error
	cursor, err = cursor.Normalize()
	if err != nil || len(search) > 200 {
		return Page[AdminBarberCard]{}, ErrInvalidPage
	}
	search = strings.ToLower(strings.TrimSpace(search))
	var items []AdminBarberCard
	if databaseEnabled() {
		args := []any{search}
		query := `with page as materialized (
   select id,full_name,phone_number,email,shop_name,status from public.barbers where strpos(lower(full_name||' '||shop_name||' '||email||' '||status),$1)>0` + pageSQL("id", cursor, false, &args) + `
  ), services as (select h.barber_id,count(*) as total from public.hairstyles h join page p on p.id=h.barber_id where not h.is_archived group by h.barber_id),
  appointments as (select b.barber_id,count(*) as total from public.bookings b join page p on p.id=b.barber_id group by b.barber_id)
  select p.id,p.full_name,p.phone_number,p.email,p.shop_name,p.status,coalesce(s.total,0),coalesce(a.total,0) from page p left join services s on s.barber_id=p.id left join appointments a on a.barber_id=p.id`
		direction := "desc"
		if pageReverse(cursor, false) {
			direction = "asc"
		}
		query += " order by p.id " + direction
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (AdminBarberCard, error) {
			var card AdminBarberCard
			err := rows.Scan(&card.ID, &card.FullName, &card.PhoneNumber, &card.Email, &card.ShopName, &card.Status, &card.Hairstyles, &card.Bookings)
			return card, err
		}, args...)
		if err != nil {
			return Page[AdminBarberCard]{}, err
		}
	} else {
		mu.RLock()
		defer mu.RUnlock()
		selected := map[int]*AdminBarberCard{}
		for _, b := range barbers {
			if !cursorIncludes(b.ID, cursor) || !strings.Contains(strings.ToLower(b.FullName+" "+b.ShopName+" "+b.Email+" "+b.Status), search) {
				continue
			}
			items = append(items, AdminBarberCard{Barber: models.Barber{ID: b.ID, FullName: b.FullName, PhoneNumber: b.PhoneNumber, Email: b.Email, ShopName: b.ShopName, Status: b.Status}})
		}
		sort.Slice(items, func(i, j int) bool {
			if pageReverse(cursor, false) {
				return items[i].ID < items[j].ID
			}
			return items[i].ID > items[j].ID
		})
		if len(items) > cursor.Limit+1 {
			items = items[:cursor.Limit+1]
		}
		for i := range items {
			selected[items[i].ID] = &items[i]
		}
		for _, h := range hairstyles {
			if c := selected[h.BarberID]; c != nil && !h.Archived {
				c.Hairstyles++
			}
		}
		for _, b := range bookings {
			if c := selected[b.BarberID]; c != nil {
				c.Bookings++
			}
		}
	}
	return finishPage(items, cursor, false), nil
}
func AdminCustomerPage(ctx context.Context, search string, cursor PageCursor) (Page[AdminCustomerCard], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[AdminCustomerCard]{}, err
	}
	var err error
	cursor, err = cursor.Normalize()
	if err != nil || len(search) > 200 {
		return Page[AdminCustomerCard]{}, ErrInvalidPage
	}
	search = strings.ToLower(strings.TrimSpace(search))
	var items []AdminCustomerCard
	if databaseEnabled() {
		args := []any{search}
		query := `with page as materialized (select id,full_name,phone_number,email from public.customers where strpos(lower(full_name||' '||email||' '||phone_number),$1)>0` + pageSQL("id", cursor, false, &args) + `
  ), appointments as (select b.customer_id,count(*) as total from public.bookings b join page p on p.id=b.customer_id group by b.customer_id)
  select p.id,p.full_name,p.phone_number,p.email,coalesce(a.total,0) from page p left join appointments a on a.customer_id=p.id`
		direction := "desc"
		if pageReverse(cursor, false) {
			direction = "asc"
		}
		query += " order by p.id " + direction
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (AdminCustomerCard, error) {
			var card AdminCustomerCard
			err := rows.Scan(&card.ID, &card.FullName, &card.PhoneNumber, &card.Email, &card.Bookings)
			return card, err
		}, args...)
		if err != nil {
			return Page[AdminCustomerCard]{}, err
		}
	} else {
		mu.RLock()
		defer mu.RUnlock()
		selected := map[int]*AdminCustomerCard{}
		for _, c := range customers {
			if cursorIncludes(c.ID, cursor) && strings.Contains(strings.ToLower(c.FullName+" "+c.Email+" "+c.PhoneNumber), search) {
				items = append(items, AdminCustomerCard{Customer: models.Customer{ID: c.ID, FullName: c.FullName, PhoneNumber: c.PhoneNumber, Email: c.Email}})
			}
		}
		sort.Slice(items, func(i, j int) bool {
			if pageReverse(cursor, false) {
				return items[i].ID < items[j].ID
			}
			return items[i].ID > items[j].ID
		})
		if len(items) > cursor.Limit+1 {
			items = items[:cursor.Limit+1]
		}
		for i := range items {
			selected[items[i].ID] = &items[i]
		}
		for _, b := range bookings {
			if c := selected[b.CustomerID]; c != nil {
				c.Bookings++
			}
		}
	}
	return finishPage(items, cursor, false), nil
}
