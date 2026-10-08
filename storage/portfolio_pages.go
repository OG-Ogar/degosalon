package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"sort"
)

func HairstylePage(ctx context.Context, barberID int, public bool, cursor PageCursor) (Page[models.Hairstyle], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[models.Hairstyle]{}, err
	}
	cursor, err := cursor.Normalize()
	if err != nil || barberID <= 0 {
		return Page[models.Hairstyle]{}, ErrInvalidPage
	}
	var items []models.Hairstyle
	if databaseEnabled() {
		args := []any{barberID}
		predicate := "not is_archived"
		if public {
			predicate += " and is_available"
		}
		query := "select id,barber_id,name,category,description,price,duration,image,video,is_available,is_archived from public.hairstyles where barber_id=$1 and " + predicate + pageSQL("id", cursor, true, &args)
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (models.Hairstyle, error) {
			var v models.Hairstyle
			err := rows.Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable, &v.Archived)
			return v, err
		}, args...)
		if err != nil {
			return Page[models.Hairstyle]{}, err
		}
	} else {
		mu.RLock()
		defer mu.RUnlock()
		for _, v := range hairstyles {
			if v.BarberID == barberID && cursorIncludes(v.ID, cursor) && !v.Archived && (!public || v.IsAvailable) {
				items = append(items, v)
			}
		}
		sort.Slice(items, func(i, j int) bool {
			if !pageReverse(cursor, true) {
				return items[i].ID < items[j].ID
			}
			return items[i].ID > items[j].ID
		})
	}
	return finishPage(items, cursor, true), nil
}
func ProductPage(ctx context.Context, barberID int, public bool, cursor PageCursor) (Page[models.Product], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[models.Product]{}, err
	}
	cursor, err := cursor.Normalize()
	if err != nil || barberID <= 0 {
		return Page[models.Product]{}, ErrInvalidPage
	}
	var items []models.Product
	if databaseEnabled() {
		args := []any{barberID}
		predicate := "not is_archived"
		if public {
			predicate += " and is_available and quantity>0"
		}
		query := "select id,barber_id,name,category,description,price,quantity,image,is_available,is_archived from public.products where barber_id=$1 and " + predicate + pageSQL("id", cursor, true, &args)
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (models.Product, error) {
			var v models.Product
			err := rows.Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Quantity, &v.Image, &v.IsAvailable, &v.Archived)
			return v, err
		}, args...)
		if err != nil {
			return Page[models.Product]{}, err
		}
	} else {
		mu.RLock()
		defer mu.RUnlock()
		for _, v := range products {
			if v.BarberID == barberID && cursorIncludes(v.ID, cursor) && !v.Archived && (!public || v.IsAvailable && v.Quantity > 0) {
				items = append(items, v)
			}
		}
		sort.Slice(items, func(i, j int) bool {
			if !pageReverse(cursor, true) {
				return items[i].ID < items[j].ID
			}
			return items[i].ID > items[j].ID
		})
	}
	return finishPage(items, cursor, true), nil
}
func ReviewPage(ctx context.Context, barberID int, cursor PageCursor) (Page[models.Review], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[models.Review]{}, err
	}
	cursor, err := cursor.Normalize()
	if err != nil || barberID <= 0 {
		return Page[models.Review]{}, ErrInvalidPage
	}
	var items []models.Review
	if databaseEnabled() {
		args := []any{barberID}
		predicate := "true"
		query := "select id,barber_id,customer_id,booking_id,customer_name,rating,comment,created_at from public.reviews where barber_id=$1 and " + predicate + pageSQL("id", cursor, false, &args)
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (models.Review, error) {
			var v models.Review
			err := rows.Scan(&v.ID, &v.BarberID, &v.CustomerID, &v.BookingID, &v.CustomerName, &v.Rating, &v.Comment, &v.CreatedAt)
			return v, err
		}, args...)
		if err != nil {
			return Page[models.Review]{}, err
		}
	} else {
		mu.RLock()
		defer mu.RUnlock()
		for _, v := range reviews {
			if v.BarberID == barberID && cursorIncludes(v.ID, cursor) {
				items = append(items, v)
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
