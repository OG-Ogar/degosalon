package storage

import (
	"barber-booking/models"
	"database/sql"
)

const productColumns = "id,barber_id,name,category,description,price,quantity,image,is_available"

func scanProduct(rows *sql.Rows) (models.Product, error) {
	var v models.Product
	err := rows.Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Quantity, &v.Image, &v.IsAvailable)
	return v, err
}
