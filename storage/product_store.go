package storage

import "barber-booking/models"

func ListProductsByBarber(barberID int) []models.Product {
	if databaseEnabled(){rows,err:=db.Query("select id,barber_id,name,category,description,price,quantity,image,is_available from public.products where barber_id=$1 order by id",barberID);if err==nil{defer rows.Close();var out []models.Product;for rows.Next(){var v models.Product;if rows.Scan(&v.ID,&v.BarberID,&v.Name,&v.Category,&v.Description,&v.Price,&v.Quantity,&v.Image,&v.IsAvailable)==nil{out=append(out,v)}};return out}}
	mu.RLock()
	defer mu.RUnlock()

	var owned []models.Product

	for _, product := range products {
		if product.BarberID == barberID {
			owned = append(owned, product)
		}
	}

	return owned
}

func ListAvailableProducts() []models.Product {
	if databaseEnabled(){rows,err:=db.Query("select id,barber_id,name,category,description,price,quantity,image,is_available from public.products where is_available=true order by id");if err==nil{defer rows.Close();var out []models.Product;for rows.Next(){var v models.Product;if rows.Scan(&v.ID,&v.BarberID,&v.Name,&v.Category,&v.Description,&v.Price,&v.Quantity,&v.Image,&v.IsAvailable)==nil{out=append(out,v)}};return out}}
	mu.RLock()
	defer mu.RUnlock()

	var available []models.Product

	for _, product := range products {
		if product.IsAvailable {
			available = append(available, product)
		}
	}

	return available
}

func FindProductByID(id int) (models.Product, bool) {
	if databaseEnabled(){var v models.Product;err:=db.QueryRow("select id,barber_id,name,category,description,price,quantity,image,is_available from public.products where id=$1",id).Scan(&v.ID,&v.BarberID,&v.Name,&v.Category,&v.Description,&v.Price,&v.Quantity,&v.Image,&v.IsAvailable);return v,err==nil}
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

func FindProductOwnedBy(id, barberID int) (models.Product, error) {
	if databaseEnabled() {
		var v models.Product
		err:=db.QueryRow("select id,barber_id,name,category,description,price,quantity,image,is_available from public.products where id=$1",id).Scan(&v.ID,&v.BarberID,&v.Name,&v.Category,&v.Description,&v.Price,&v.Quantity,&v.Image,&v.IsAvailable)
		if err!=nil{return models.Product{},ErrNotFound}; if v.BarberID!=barberID{return models.Product{},ErrForbidden}; return v,nil
	}
	mu.RLock()
	defer mu.RUnlock()

	product, found := findProductByID(id)

	if !found {
		return models.Product{}, ErrNotFound
	}

	if product.BarberID != barberID {
		return models.Product{}, ErrForbidden
	}

	return product, nil
}

func CreateProduct(product models.Product) (models.Product, error) {
	if databaseEnabled(){err:=db.QueryRow("insert into public.products(barber_id,name,category,description,price,quantity,image,is_available) values($1,$2,$3,$4,$5,$6,$7,$8) returning id",product.BarberID,product.Name,product.Category,product.Description,product.Price,product.Quantity,product.Image,product.IsAvailable).Scan(&product.ID);return product,err}
	mu.Lock()
	defer mu.Unlock()

	product.ID = nextID(&lastProductID)

	products = append(products, product)

	return product, nil
}

func UpdateProduct(product models.Product) error {
	if databaseEnabled() {
		result,err:=db.Exec("update public.products set name=$1,category=$2,description=$3,price=$4,quantity=$5,image=$6,is_available=$7 where id=$8 and barber_id=$9",product.Name,product.Category,product.Description,product.Price,product.Quantity,product.Image,product.IsAvailable,product.ID,product.BarberID)
		if err!=nil{return err}; n,_:=result.RowsAffected(); if n==0{return ErrNotFound}; return nil
	}
	mu.Lock()
	defer mu.Unlock()

	for i := range products {
		if products[i].ID != product.ID {
			continue
		}

		if products[i].BarberID != product.BarberID {
			return ErrForbidden
		}

		products[i] = product

		return nil
	}

	return ErrNotFound
}

func DeleteProduct(id, barberID int) (string, error) {
	if databaseEnabled() {
		var image string
		err:=db.QueryRow("delete from public.products where id=$1 and barber_id=$2 returning image",id,barberID).Scan(&image)
		if err!=nil{return "",ErrNotFound}; return image,nil
	}
	mu.Lock()
	defer mu.Unlock()

	for i := range products {
		if products[i].ID != id {
			continue
		}

		if products[i].BarberID != barberID {
			return "", ErrForbidden
		}

		image := products[i].Image

		products = append(products[:i], products[i+1:]...)

		return image, nil
	}

	return "", ErrNotFound
}
