package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"sort"
)

type CatalogueProduct struct {
	Product models.Product
	Barber  models.Barber
}

// CataloguePage applies availability, archive, seller and product filters in
// one bounded joined query. Only public seller name fields are selected.
func CataloguePage(ctx context.Context, sellerID, productID int, cursor PageCursor) (Page[CatalogueProduct], error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Page[CatalogueProduct]{}, err
	}
	var err error
	cursor, err = cursor.Normalize()
	if err != nil || sellerID < 0 || productID < 0 {
		return Page[CatalogueProduct]{}, ErrInvalidPage
	}
	var items []CatalogueProduct
	if databaseEnabled() {
		query := `select p.id,p.barber_id,p.name,p.category,p.description,p.price,p.quantity,p.image,p.video,p.is_available,p.is_archived,b.id,b.full_name,b.shop_name
   from public.products p join public.barbers b on b.id=p.barber_id where p.is_available and not p.is_archived and b.status='Active'`
		args := []any{}
		if sellerID > 0 {
			args = append(args, sellerID)
			query += " and p.barber_id=$1"
		}
		if productID > 0 {
			args = append(args, productID)
			if sellerID > 0 {
				query += " and p.id=$2"
			} else {
				query += " and p.id=$1"
			}
		}
		query += pageSQL("p.id", cursor, true, &args)
		items, err = queryRecords(ctx, query, func(rows *sql.Rows) (CatalogueProduct, error) {
			var row CatalogueProduct
			err := rows.Scan(&row.Product.ID, &row.Product.BarberID, &row.Product.Name, &row.Product.Category, &row.Product.Description, &row.Product.Price, &row.Product.Quantity, &row.Product.Image, &row.Product.Video, &row.Product.IsAvailable, &row.Product.Archived, &row.Barber.ID, &row.Barber.FullName, &row.Barber.ShopName)
			return row, err
		}, args...)
		if err != nil {
			return Page[CatalogueProduct]{}, err
		}
	} else {
		mu.RLock()
		defer mu.RUnlock()
		for _, product := range products {
			if !product.IsAvailable || product.Archived || sellerID > 0 && product.BarberID != sellerID || productID > 0 && product.ID != productID || !cursorIncludes(product.ID, cursor) {
				continue
			}
			barber, found := findBarberByID(product.BarberID)
			if !found || !barber.IsActive() {
				continue
			}
			items = append(items, CatalogueProduct{Product: product, Barber: models.Barber{ID: barber.ID, FullName: barber.FullName, ShopName: barber.ShopName}})
		}
		sort.Slice(items, func(i, j int) bool {
			if pageReverse(cursor, true) {
				return items[i].Product.ID > items[j].Product.ID
			}
			return items[i].Product.ID < items[j].Product.ID
		})
		if len(items) > cursor.Limit+1 {
			items = items[:cursor.Limit+1]
		}
	}
	return finishPage(items, cursor, true), nil
}
