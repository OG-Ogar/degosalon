package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"sort"
)

// MessagePage returns newest-first storage rows. Views reverse each bounded
// page for chronological reading while retaining ID-based history cursors.
func MessagePage(ctx context.Context, barberID, customerID int, cursor PageCursor) (Page[models.Message], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[models.Message]{}, err
	}
	var err error
	cursor, err = cursor.Normalize()
	if err != nil || barberID < 1 || customerID < 1 {
		return Page[models.Message]{}, ErrInvalidPage
	}
	var items []models.Message
	if databaseEnabled() {
		args := []any{barberID, customerID}
		query := `select id,barber_id,customer_id,sender_role,sender_name,text,sent_at from public.messages where barber_id=$1 and customer_id=$2` + pageSQL("id", cursor, false, &args)
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (models.Message, error) {
			var m models.Message
			err := rows.Scan(&m.ID, &m.BarberID, &m.CustomerID, &m.SenderRole, &m.SenderName, &m.Text, &m.SentAt)
			return m, err
		}, args...)
		if err != nil {
			return Page[models.Message]{}, err
		}
	} else {
		mu.RLock()
		defer mu.RUnlock()
		for _, m := range messages {
			if m.BarberID == barberID && m.CustomerID == customerID && cursorIncludes(m.ID, cursor) {
				items = append(items, m)
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
	}
	return finishPage(items, cursor, false), nil
}

type ConversationCard struct {
	Conversation
	LastID              int
	Name, Detail, Image string
}

func ConversationPageForCustomer(ctx context.Context, id int, cursor PageCursor) (Page[ConversationCard], error) {
	return conversationPage(ctx, "customer", id, cursor)
}
func ConversationPageForBarber(ctx context.Context, id int, cursor PageCursor) (Page[ConversationCard], error) {
	return conversationPage(ctx, "barber", id, cursor)
}

func conversationPage(ctx context.Context, role string, id int, cursor PageCursor) (Page[ConversationCard], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[ConversationCard]{}, err
	}
	var err error
	cursor, err = cursor.Normalize()
	if err != nil || id < 1 {
		return Page[ConversationCard]{}, ErrInvalidPage
	}
	var items []ConversationCard
	if databaseEnabled() {
		args := []any{id}
		participant := "public.barbers"
		join := "p.barber_id"
		detail := "b.shop_name"
		image := "b.personal_image"
		fallback := "Barber"
		if role == "barber" {
			participant = "public.customers"
			join = "p.customer_id"
			detail = "b.phone_number"
			image = "b.profile_image"
			fallback = "Customer"
		}
		query := `with latest as (
   select m.barber_id,m.customer_id,max(m.id) as last_id from public.messages m where m.` + role + `_id=$1 group by m.barber_id,m.customer_id
  ), page as materialized (
   select * from latest where true` + pageSQL("last_id", cursor, false, &args) + `
  ), counts as (
   select m.barber_id,m.customer_id,count(*) as total from public.messages m join page p on p.barber_id=m.barber_id and p.customer_id=m.customer_id group by m.barber_id,m.customer_id
  ) select p.barber_id,p.customer_id,m.text,m.sent_at,c.total,p.last_id,coalesce(b.full_name,'` + fallback + `'),coalesce(` + detail + `,''),coalesce(` + image + `,'')
  from page p join public.messages m on m.id=p.last_id join counts c on c.barber_id=p.barber_id and c.customer_id=p.customer_id
  left join ` + participant + ` b on b.id=` + join
		direction := "desc"
		if pageReverse(cursor, false) {
			direction = "asc"
		}
		query += " order by p.last_id " + direction
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (ConversationCard, error) {
			var c ConversationCard
			err := rows.Scan(&c.BarberID, &c.CustomerID, &c.LastText, &c.LastAt, &c.Count, &c.LastID, &c.Name, &c.Detail, &c.Image)
			return c, err
		}, args...)
		if err != nil {
			return Page[ConversationCard]{}, err
		}
	} else {
		mu.RLock()
		defer mu.RUnlock()
		grouped := map[[2]int]*ConversationCard{}
		for _, m := range messages {
			if role == "customer" && m.CustomerID != id || role == "barber" && m.BarberID != id {
				continue
			}
			key := [2]int{m.BarberID, m.CustomerID}
			card := grouped[key]
			if card == nil {
				card = &ConversationCard{Conversation: Conversation{BarberID: m.BarberID, CustomerID: m.CustomerID}}
				grouped[key] = card
			}
			card.Count++
			if m.ID > card.LastID {
				card.LastID = m.ID
				card.LastText = m.Text
				card.LastAt = m.SentAt
			}
		}
		for _, card := range grouped {
			if !cursorIncludes(card.LastID, cursor) {
				continue
			}
			if role == "customer" {
				card.Name = "Barber"
				if b, ok := findBarberByID(card.BarberID); ok {
					card.Name = b.FullName
					card.Detail = b.ShopName
					card.Image = b.PersonalImage
				}
			} else {
				card.Name = "Customer"
				if c, ok := findCustomerByID(card.CustomerID); ok {
					card.Name = c.FullName
					card.Detail = c.PhoneNumber
					card.Image = c.ProfileImage
				}
			}
			items = append(items, *card)
		}
		sort.Slice(items, func(i, j int) bool {
			if pageReverse(cursor, false) {
				return items[i].LastID < items[j].LastID
			}
			return items[i].LastID > items[j].LastID
		})
		if len(items) > cursor.Limit+1 {
			items = items[:cursor.Limit+1]
		}
	}
	return finishPage(items, cursor, false), nil
}
func ConversationCountForCustomer(ctx context.Context, id int) (int, error) {
	return conversationCount(ctx, "customer_id", id)
}
func ConversationCountForBarber(ctx context.Context, id int) (int, error) {
	return conversationCount(ctx, "barber_id", id)
}
func conversationCount(ctx context.Context, field string, id int) (int, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if databaseEnabled() {
		other := "barber_id"
		if field == "barber_id" {
			other = "customer_id"
		}
		var count int
		err := db.QueryRowContext(ctx, "select count(distinct "+other+") from public.messages where "+field+"=$1", id).Scan(&count)
		return count, err
	}
	mu.RLock()
	defer mu.RUnlock()
	seen := map[int]bool{}
	for _, m := range messages {
		if field == "customer_id" && m.CustomerID == id {
			seen[m.BarberID] = true
		} else if field == "barber_id" && m.BarberID == id {
			seen[m.CustomerID] = true
		}
	}
	return len(seen), nil
}
