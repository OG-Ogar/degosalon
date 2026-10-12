package storage

import (
	"barber-booking/internal/geography"
	"barber-booking/models"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const DiscoveryPageSize = 24
const maxDiscoveryPageSize = 48

var ErrInvalidDiscoveryFilter = errors.New("invalid discovery filter")

type DiscoveryFilter struct {
	StateID, AreaID, Query string
	After, Before, Limit   int
}
type DiscoveryBarber struct {
	models.Barber
	Hairstyles        int
	FromPrice, Rating float64
	ReviewCount       int
}
type DiscoveryPage struct {
	Items                []DiscoveryBarber
	Total                int
	HasPrevious, HasNext bool
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func discoveryStatements(filter DiscoveryFilter) (string, []any, string, []any) {
	where := "b.status='Active'"
	args := []any{}
	if filter.StateID != "" {
		args = append(args, filter.StateID)
		where += fmt.Sprintf(" and b.state_id=$%d", len(args))
	}
	if filter.AreaID != "" {
		args = append(args, filter.AreaID)
		where += fmt.Sprintf(" and b.local_area_id=$%d", len(args))
	}
	if filter.Query != "" {
		args = append(args, "%"+likeEscaper.Replace(filter.Query)+"%")
		where += fmt.Sprintf(" and (b.full_name ilike $%d escape '\\' or b.shop_name ilike $%d escape '\\')", len(args), len(args))
	}
	count := "select count(*) from public.barbers b where " + where
	countArgs := append([]any(nil), args...)
	direction := "asc"
	if filter.After > 0 {
		args = append(args, filter.After)
		where += fmt.Sprintf(" and b.id>$%d", len(args))
	}
	if filter.Before > 0 {
		args = append(args, filter.Before)
		where += fmt.Sprintf(" and b.id<$%d", len(args))
		direction = "desc"
	}
	args = append(args, filter.Limit+1)
	query := fmt.Sprintf(`with page as materialized (
 select b.id,b.full_name,b.shop_name,b.shop_address,b.state,b.local_area,b.profile_image,b.personal_image,b.status
 from public.barbers b where %s order by b.id %s limit $%d
 ), services as (
 select h.barber_id,count(*) as total,min(h.price) as from_price from public.hairstyles h
 join page p on p.id=h.barber_id where h.is_available and not h.is_archived group by h.barber_id
 ), ratings as (
 select r.barber_id,avg(r.rating) as rating,count(*) as total from public.reviews r
 join page p on p.id=r.barber_id group by r.barber_id
 )
 select p.id,p.full_name,p.shop_name,p.shop_address,p.state,p.local_area,p.profile_image,p.personal_image,p.status,
 coalesce(s.total,0),coalesce(s.from_price,0),coalesce(r.rating,0),coalesce(r.total,0)
 from page p left join services s on s.barber_id=p.id left join ratings r on r.barber_id=p.id
 order by p.id %s`, where, direction, len(args), direction)
	return count, countArgs, query, args
}

// DiscoveryPageResult performs one count and one bounded card/summary query.
// Passwords and account contact fields are never selected for these public cards.
func DiscoveryPageResult(ctx context.Context, filter DiscoveryFilter) (DiscoveryPage, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return DiscoveryPage{}, err
	}
	if filter.Limit == 0 {
		filter.Limit = DiscoveryPageSize
	}
	if filter.Limit < 1 || filter.Limit > maxDiscoveryPageSize || filter.After < 0 || filter.Before < 0 || (filter.After > 0 && filter.Before > 0) {
		return DiscoveryPage{}, ErrInvalidDiscoveryFilter
	}
	filter.Query = strings.TrimSpace(filter.Query)
	if len(filter.Query) > 80 {
		filter.Query = filter.Query[:80]
	}
	if filter.StateID == "" {
		if filter.AreaID != "" {
			return DiscoveryPage{}, ErrInvalidDiscoveryFilter
		}
		if filter.Query == "" {
			return DiscoveryPage{}, nil
		}
	} else {
		state, ok := geography.FindState(filter.StateID)
		if !ok {
			return DiscoveryPage{}, ErrInvalidDiscoveryFilter
		}
		filter.StateID = state.ID
		if filter.AreaID != "" {
			area, ok := geography.FindArea(state, filter.AreaID)
			if !ok {
				return DiscoveryPage{}, ErrInvalidDiscoveryFilter
			}
			filter.AreaID = area.ID
		}
	}
	var page DiscoveryPage
	if databaseEnabled() {
		count, countArgs, query, args := discoveryStatements(filter)
		if err := db.QueryRowContext(ctx, count, countArgs...).Scan(&page.Total); err != nil {
			return DiscoveryPage{}, err
		}
		items, err := queryRecords(ctx, query, func(rows *sql.Rows) (DiscoveryBarber, error) {
			var b DiscoveryBarber
			err := rows.Scan(&b.ID, &b.FullName, &b.ShopName, &b.ShopAddress, &b.State, &b.LocalArea, &b.ProfileImage, &b.PersonalImage, &b.Status, &b.Hairstyles, &b.FromPrice, &b.Rating, &b.ReviewCount)
			return b, err
		}, args...)
		if err != nil {
			return DiscoveryPage{}, err
		}
		page.Items = items
	} else {
		mu.RLock()
		defer mu.RUnlock()
		for _, b := range barbers {
			s, ok := geography.FindState(b.State)
			if !b.IsActive() {
				continue
			}
			if filter.StateID != "" && (!ok || s.ID != filter.StateID) {
				continue
			}
			if filter.Query != "" {
				needle := strings.ToLower(filter.Query)
				if !strings.Contains(strings.ToLower(b.FullName), needle) &&
					!strings.Contains(strings.ToLower(b.ShopName), needle) {
					continue
				}
			}
			if filter.AreaID != "" {
				a, ok := geography.FindArea(s, b.LocalArea)
				if !ok || a.ID != filter.AreaID {
					continue
				}
			}
			page.Total++
			if filter.After > 0 && b.ID <= filter.After || filter.Before > 0 && b.ID >= filter.Before {
				continue
			}
			b.State, b.LocalArea = geography.NormalizeLegacy(b.State, b.LocalArea)
			b.PasswordHash = ""
			b.Email = ""
			b.PhoneNumber = ""
			page.Items = append(page.Items, DiscoveryBarber{Barber: b})
		}
		sort.Slice(page.Items, func(i, j int) bool {
			if filter.Before > 0 {
				return page.Items[i].ID > page.Items[j].ID
			}
			return page.Items[i].ID < page.Items[j].ID
		})
		if len(page.Items) > filter.Limit+1 {
			page.Items = page.Items[:filter.Limit+1]
		}
		selected := map[int]*DiscoveryBarber{}
		for i := range page.Items {
			selected[page.Items[i].ID] = &page.Items[i]
		}
		for _, h := range hairstyles {
			if b := selected[h.BarberID]; b != nil && h.IsAvailable && !h.Archived {
				b.Hairstyles++
				if b.Hairstyles == 1 || h.Price < b.FromPrice {
					b.FromPrice = h.Price
				}
			}
		}
		for _, r := range reviews {
			if b := selected[r.BarberID]; b != nil {
				b.ReviewCount++
				b.Rating += float64(r.Rating)
			}
		}
		for i := range page.Items {
			if page.Items[i].ReviewCount > 0 {
				page.Items[i].Rating /= float64(page.Items[i].ReviewCount)
			}
		}
		if err := ctx.Err(); err != nil {
			return DiscoveryPage{}, err
		}
	}
	extra := len(page.Items) > filter.Limit
	if extra {
		page.Items = page.Items[:filter.Limit]
	}
	if len(page.Items) > 0 {
		if filter.Before > 0 {
			page.HasPrevious = extra
			page.HasNext = true
			for i, j := 0, len(page.Items)-1; i < j; i, j = i+1, j-1 {
				page.Items[i], page.Items[j] = page.Items[j], page.Items[i]
			}
		} else {
			page.HasPrevious = filter.After > 0
			page.HasNext = extra
		}
	}
	return page, nil
}
