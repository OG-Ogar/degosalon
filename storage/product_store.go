package storage

import (
	"barber-booking/internal/money"
	"barber-booking/models"
	"context"
)

func ListProductsByBarber(ctx context.Context, barberID int) ([]models.Product, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if databaseEnabled() {
		return queryRecords(ctx, "select "+productColumns+" from public.products where barber_id=$1 and not is_archived order by id", scanProduct, barberID)
	}
	mu.RLock()
	defer mu.RUnlock()
	out := make([]models.Product, 0)
	for _, p := range products {
		if p.BarberID == barberID && !p.Archived {
			out = append(out, p)
		}
	}
	return out, nil
}

func ListAvailableProducts(ctx context.Context) ([]models.Product, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if databaseEnabled() {
		return queryRecords(ctx, "select "+productColumns+" from public.products where is_available=true and not is_archived order by id", scanProduct)
	}
	mu.RLock()
	defer mu.RUnlock()
	out := make([]models.Product, 0)
	for _, p := range products {
		if p.IsAvailable && !p.Archived {
			out = append(out, p)
		}
	}
	return out, nil
}

func FindProductByID(id int) (models.Product, bool) {
	ctx, cancel := databaseContext(context.Background())
	defer cancel()

	if databaseEnabled() {
		var v models.Product
		err := db.QueryRowContext(ctx, "select "+productColumns+" from public.products where id=$1", id).Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Quantity, &v.Image, &v.Video, &v.IsAvailable, &v.Archived)
		return v, err == nil
	}
	mu.RLock()
	defer mu.RUnlock()

	return findProductByID(id)
}

func findProductByID(id int) (models.Product, bool) {
	for _, product := range products {
		if product.ID == id {
			return product, true
		}
	}

	return models.Product{}, false
}

func FindProductOwnedBy(ctx context.Context, id, barberID int) (models.Product, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	product, err := FindProductByIDResult(ctx, id)
	if err != nil {
		return models.Product{}, err
	}
	if product.Archived {
		return models.Product{}, ErrNotFound
	}
	if product.BarberID != barberID {
		return models.Product{}, ErrForbidden
	}
	return product, nil
}

func CreateProduct(product models.Product) (models.Product, error) {
	return CreateProductContext(context.Background(), product)
}

func CreateProductContext(ctx context.Context, product models.Product) (models.Product, error) {
	product.Archived = false
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return models.Product{}, err
	}

	if _, err := money.FromFloat(product.Price); err != nil {
		return models.Product{}, err
	}

	if databaseEnabled() {
		err := db.QueryRowContext(ctx, "insert into public.products(barber_id,name,category,description,price,quantity,image,video,is_available) values($1,$2,$3,$4,$5,$6,$7,$8,$9) returning id", product.BarberID, product.Name, product.Category, product.Description, product.Price, product.Quantity, product.Image, product.Video, product.IsAvailable).Scan(&product.ID)
		return product, err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return models.Product{}, err
	}
	if err := guardMediaLocked(product.Image, product.Video); err != nil {
		return models.Product{}, err
	}

	product.ID = nextID(&lastProductID)

	products = append(products, product)

	return product, nil
}

func UpdateProduct(product models.Product) error {
	return UpdateProductContext(context.Background(), product)
}

func UpdateProductContext(ctx context.Context, product models.Product) error {
	product.Archived = false
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}

	if _, err := money.FromFloat(product.Price); err != nil {
		return err
	}

	if databaseEnabled() {
		result, err := db.ExecContext(ctx, "update public.products set name=$1,category=$2,description=$3,price=$4,quantity=$5,image=$6,video=$7,is_available=$8 where id=$9 and barber_id=$10 and not is_archived", product.Name, product.Category, product.Description, product.Price, product.Quantity, product.Image, product.Video, product.IsAvailable, product.ID, product.BarberID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	for i := range products {
		if products[i].ID != product.ID {
			continue
		}

		if products[i].Archived {
			return ErrNotFound
		}
		if products[i].BarberID != product.BarberID {
			return ErrForbidden
		}

		if err := guardMediaLocked(product.Image, product.Video); err != nil {
			return err
		}
		products[i] = product

		return nil
	}

	return ErrNotFound
}

// DeleteProduct archives an owned product and retains its row and media.
// Its compatibility cleanup path is empty.
func DeleteProduct(id, barberID int) (string, error) {
	return DeleteProductContext(context.Background(), id, barberID)
}

func DeleteProductContext(ctx context.Context, id, barberID int) (string, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if databaseEnabled() {
		var archivedID int
		err := db.QueryRowContext(ctx, "update public.products set is_archived=true,is_available=false where id=$1 and barber_id=$2 and not is_archived returning id", id, barberID).Scan(&archivedID)
		if err != nil {
			return "", recordError(err)
		}
		return "", nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}

	for i := range products {
		if products[i].ID != id {
			continue
		}

		if products[i].Archived {
			return "", ErrNotFound
		}
		if products[i].BarberID != barberID {
			return "", ErrForbidden
		}

		products[i].Archived = true
		products[i].IsAvailable = false
		return "", nil
	}

	return "", ErrNotFound
}

// FindProductByIDResult is the error-aware database form used by request
// handlers that must distinguish a missing product from a database outage.
func FindProductByIDResult(ctx context.Context, id int) (models.Product, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return models.Product{}, err
	}
	if databaseEnabled() {
		var v models.Product
		err := db.QueryRowContext(ctx, "select "+productColumns+" from public.products where id=$1", id).Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Quantity, &v.Image, &v.Video, &v.IsAvailable, &v.Archived)
		return v, recordError(err)
	}
	mu.RLock()
	defer mu.RUnlock()
	v, ok := findProductByID(id)
	if !ok {
		return models.Product{}, ErrNotFound
	}
	return v, nil
}
