package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

type BookingCard struct {
	models.Booking
	BarberName, ShopName, HairstyleName, HairstyleImage string
	Price                                               float64
	Duration                                            int
}
type BookingFilter struct {
	BarberID, CustomerID int
	Group                string
	Now                  time.Time
	LegacyTimezone       string
}

func (f BookingFilter) valid() bool {
	if f.BarberID < 0 || f.CustomerID < 0 {
		return false
	}
	switch f.Group {
	case "", "all", "pending", "upcoming", "followup", "finished":
		return true
	}
	return false
}
func bookingScope(f BookingFilter, args *[]any) string {
	query := "true"
	if f.BarberID > 0 {
		*args = append(*args, f.BarberID)
		query += fmt.Sprintf(" and b.barber_id=$%d", len(*args))
	}
	if f.CustomerID > 0 {
		*args = append(*args, f.CustomerID)
		query += fmt.Sprintf(" and b.customer_id=$%d", len(*args))
	}
	return query
}

// Validate the timezone before AT TIME ZONE so malformed legacy snapshots
// remain follow-up work rather than aborting the entire list.
func bookingClassificationSQL(f BookingFilter, args *[]any) string {
	*args = append(*args, f.LegacyTimezone)
	zone := fmt.Sprintf("coalesce(nullif(b.booking_timezone,''),$%d)", len(*args))
	*args = append(*args, f.Now)
	now := fmt.Sprintf("$%d", len(*args))
	return "case when b.status not in ('Pending','Accepted') then 'finished' when (case when exists(select 1 from pg_timezone_names tz where tz.name=" + zone + ") then (b.booking_date+b.booking_time) at time zone " + zone + " end)>" + now + " then 'upcoming' else 'followup' end"
}
func bookingMatches(b models.Booking, f BookingFilter) bool {
	if f.BarberID > 0 && b.BarberID != f.BarberID || f.CustomerID > 0 && b.CustomerID != f.CustomerID {
		return false
	}
	group := ClassifyAppointment(b, f.Now, f.LegacyTimezone)
	switch f.Group {
	case "pending":
		return b.Status == models.StatusPending
	case "upcoming":
		return group == AppointmentUpcoming
	case "followup":
		return group == AppointmentFollowUp
	case "finished":
		return group == AppointmentFinished
	}
	return true
}
func bookingCard(b models.Booking) BookingCard {
	card := BookingCard{Booking: b, BarberName: b.BarberName, ShopName: b.ShopName, HairstyleName: b.ServiceName, Price: b.ServicePrice, Duration: b.ServiceDuration}
	if b.ServiceDuration > 0 {
		return card
	}
	if barber, ok := findBarberByID(b.BarberID); ok {
		card.BarberName, card.ShopName = barber.FullName, barber.ShopName
	}
	for _, style := range hairstyles {
		if style.ID == b.HairstyleID {
			card.HairstyleName, card.HairstyleImage, card.Price, card.Duration = style.Name, style.Image, style.Price, style.Duration
			break
		}
	}
	return card
}
func BookingPage(ctx context.Context, f BookingFilter, cursor PageCursor) (Page[BookingCard], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[BookingCard]{}, err
	}
	cursor, err := cursor.Normalize()
	if err != nil || !f.valid() {
		return Page[BookingCard]{}, ErrInvalidPage
	}
	var items []BookingCard
	if databaseEnabled() {
		args := []any{}
		predicate := bookingScope(f, &args)
		if f.Group == "pending" {
			predicate += " and b.status='Pending'"
		} else if f.Group != "" && f.Group != "all" {
			predicate += " and (" + bookingClassificationSQL(f, &args) + ")='" + f.Group + "'"
		}
		query := `select b.id,b.barber_id,b.hairstyle_id,b.customer_id,b.customer_name,b.customer_phone,b.customer_email,b.booking_date::text,b.booking_time::text,b.notes,b.status,b.created_at,b.barber_name,b.shop_name,b.shop_address,b.service_name,b.service_price,b.service_duration,coalesce(b.booking_timezone,''),case when b.service_duration>0 then b.barber_name else coalesce(s.full_name,'') end,case when b.service_duration>0 then b.shop_name else coalesce(s.shop_name,'') end,case when b.service_duration>0 then b.service_name else coalesce(h.name,'') end,case when b.service_duration>0 then '' else coalesce(h.image,'') end,case when b.service_duration>0 then b.service_price else coalesce(h.price,0) end,case when b.service_duration>0 then b.service_duration else coalesce(h.duration,0) end from public.bookings b left join public.barbers s on s.id=b.barber_id left join public.hairstyles h on h.id=b.hairstyle_id where ` + predicate + pageSQL("b.id", cursor, false, &args)
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (BookingCard, error) {
			var v BookingCard
			err := rows.Scan(&v.ID, &v.BarberID, &v.HairstyleID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.Date, &v.Time, &v.Notes, &v.Status, &v.CreatedAt, &v.Booking.BarberName, &v.Booking.ShopName, &v.ShopAddress, &v.ServiceName, &v.ServicePrice, &v.ServiceDuration, &v.BookingTimezone, &v.BarberName, &v.ShopName, &v.HairstyleName, &v.HairstyleImage, &v.Price, &v.Duration)
			return v, err
		}, args...)
		if err != nil {
			return Page[BookingCard]{}, err
		}
	} else {
		mu.RLock()
		defer mu.RUnlock()
		for _, b := range bookings {
			if cursorIncludes(b.ID, cursor) && bookingMatches(b, f) {
				items = append(items, bookingCard(b))
			}
		}
		sort.Slice(items, func(i, j int) bool {
			if pageReverse(cursor, false) {
				return items[i].ID < items[j].ID
			}
			return items[i].ID > items[j].ID
		})
	}
	return finishPage(items, cursor, false), nil
}

type DashboardCounts struct{ Barbers, Hairstyles, Pending, Upcoming, AcceptedUpcoming int }

func DashboardTotals(ctx context.Context, f BookingFilter) (DashboardCounts, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return DashboardCounts{}, err
	}
	if !f.valid() || f.BarberID == 0 && f.CustomerID == 0 {
		return DashboardCounts{}, ErrInvalidPage
	}
	var result DashboardCounts
	if databaseEnabled() {
		args := []any{}
		predicate := bookingScope(f, &args)
		group := bookingClassificationSQL(f, &args)
		args = append(args, f.BarberID)
		owner := fmt.Sprintf("$%d", len(args))
		query := `with appointments as (select b.status,` + group + ` as classification from public.bookings b where ` + predicate + `) select (select count(*) from public.barbers where status='Active'),(select count(*) from public.hairstyles where barber_id=` + owner + ` and not is_archived),count(*) filter(where status='Pending'),count(*) filter(where classification='upcoming'),count(*) filter(where classification='upcoming' and status='Accepted') from appointments`
		err := db.QueryRowContext(ctx, query, args...).Scan(&result.Barbers, &result.Hairstyles, &result.Pending, &result.Upcoming, &result.AcceptedUpcoming)
		return result, err
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, b := range barbers {
		if b.IsActive() {
			result.Barbers++
		}
	}
	for _, h := range hairstyles {
		if h.BarberID == f.BarberID && !h.Archived {
			result.Hairstyles++
		}
	}
	f.Group = "all"
	for _, b := range bookings {
		if !bookingMatches(b, f) {
			continue
		}
		if b.Status == models.StatusPending {
			result.Pending++
		}
		if ClassifyAppointment(b, f.Now, f.LegacyTimezone) == AppointmentUpcoming {
			result.Upcoming++
			if b.Status == models.StatusAccepted {
				result.AcceptedUpcoming++
			}
		}
	}
	return result, nil
}
