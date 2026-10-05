package storage

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"sort"
	"time"

	"barber-booking/models"

	"github.com/jackc/pgx/v5"
)

// Product order storage.
var (
	productOrders     []models.ProductOrder
	productOrderItems []models.ProductOrderItem
)

// IDs for product orders and order items.
var (
	lastProductOrderID     int
	lastProductOrderItemID int
)

type CheckoutError struct{ Message string }

func (e CheckoutError) Error() string { return e.Message }

// CheckoutCart atomically converts a customer's cart into one order. Stock is
// reserved only here, at checkout, so abandoned carts never consume inventory.
func CheckoutCart(customer models.Customer) (models.ProductOrder, error) {
	return CheckoutCartContext(context.Background(), customer)
}

func CheckoutCartContext(ctx context.Context, customer models.Customer) (models.ProductOrder, error) {
	return checkoutCart(ctx, customer, "")
}

var ErrInvalidCheckoutKey = errors.New("invalid checkout retry key")

func ValidCheckoutKey(key string) bool {
	if len(key) != 32 {
		return false
	}
	decoded, err := hex.DecodeString(key)
	return err == nil && len(decoded) == 16 && hex.EncodeToString(decoded) == key
}

// CheckoutCartWithKey replays a successful customer-scoped request before
// inspecting the current cart. Replays never consume a subsequently added cart.
func CheckoutCartWithKey(ctx context.Context, customer models.Customer, key string) (models.ProductOrder, error) {
	if !ValidCheckoutKey(key) {
		return models.ProductOrder{}, ErrInvalidCheckoutKey
	}
	return checkoutCart(ctx, customer, key)
}

func checkoutCart(ctx context.Context, customer models.Customer, key string) (models.ProductOrder, error) {
	if err := ctx.Err(); err != nil {
		return models.ProductOrder{}, err
	}
	if !databaseEnabled() {
		return checkoutCartMemory(customer, key)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return models.ProductOrder{}, err
	}
	defer tx.Rollback()
	if err := lockCustomerCart(ctx, tx, customer.ID); err != nil {
		return models.ProductOrder{}, err
	}

	if key != "" {
		var previous models.ProductOrder
		err := tx.QueryRowContext(ctx, `select id,customer_id,customer_name,customer_phone,customer_email,total_amount,status,created_at from public.product_orders where customer_id=$1 and checkout_key=$2`, customer.ID, key).Scan(&previous.ID, &previous.CustomerID, &previous.CustomerName, &previous.CustomerPhone, &previous.CustomerEmail, &previous.TotalAmount, &previous.Status, &previous.CreatedAt)
		if err == nil {
			previous.CheckoutKey = key
			return previous, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return models.ProductOrder{}, err
		}
	}
	rows, err := tx.QueryContext(ctx, "select product_id,quantity from public.carts where customer_id=$1 order by product_id for update", customer.ID)
	if err != nil {
		return models.ProductOrder{}, err
	}
	var cart []models.CartItem
	for rows.Next() {
		var item models.CartItem
		if err = rows.Scan(&item.ProductID, &item.Quantity); err != nil {
			rows.Close()
			return models.ProductOrder{}, err
		}
		cart = append(cart, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return models.ProductOrder{}, err
	}
	rows.Close()
	if len(cart) == 0 {
		return models.ProductOrder{}, ErrNotFound
	}

	type line struct {
		product  models.Product
		quantity int
		subtotal float64
	}
	lines := make([]line, 0, len(cart))
	total := 0.0
	for _, item := range cart {
		if item.Quantity <= 0 {
			return models.ProductOrder{}, CheckoutError{"Invalid product quantity in cart."}
		}
		var p models.Product
		var barberStatus string
		err = tx.QueryRowContext(ctx, `select p.id,p.barber_id,p.name,p.category,p.description,p.price,p.quantity,p.image,p.is_available,b.status
			from public.products p join public.barbers b on b.id=p.barber_id where p.id=$1 for update of p for share of b`, item.ProductID).
			Scan(&p.ID, &p.BarberID, &p.Name, &p.Category, &p.Description, &p.Price, &p.Quantity, &p.Image, &p.IsAvailable, &barberStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ProductOrder{}, CheckoutError{"A product in your cart is no longer available."}
		}
		if err != nil {
			return models.ProductOrder{}, err
		}
		if !p.IsAvailable || barberStatus != "Active" {
			return models.ProductOrder{}, CheckoutError{"A product in your cart is no longer available for sale."}
		}
		if p.Quantity < item.Quantity {
			return models.ProductOrder{}, CheckoutError{"A product in your cart no longer has enough stock."}
		}
		sub := p.Price * float64(item.Quantity)
		total += sub
		lines = append(lines, line{p, item.Quantity, sub})
	}

	order := models.ProductOrder{CheckoutKey: key, CustomerID: customer.ID, CustomerName: customer.FullName, CustomerPhone: customer.PhoneNumber, CustomerEmail: customer.Email, TotalAmount: total, Status: models.OrderPending, CreatedAt: time.Now()}
	var storedKey any
	if key != "" {
		storedKey = key
	}
	err = tx.QueryRowContext(ctx, "insert into public.product_orders(customer_id,customer_name,customer_phone,customer_email,total_amount,status,created_at,checkout_key) values($1,$2,$3,$4,$5,$6,$7,$8) returning id", order.CustomerID, order.CustomerName, order.CustomerPhone, order.CustomerEmail, order.TotalAmount, order.Status, order.CreatedAt, storedKey).Scan(&order.ID)
	if err != nil {
		return models.ProductOrder{}, err
	}
	for _, item := range lines {
		_, err = tx.ExecContext(ctx, "insert into public.product_order_items(order_id,product_id,barber_id,product_name,price,quantity,subtotal) values($1,$2,$3,$4,$5,$6,$7)", order.ID, item.product.ID, item.product.BarberID, item.product.Name, item.product.Price, item.quantity, item.subtotal)
		if err != nil {
			return models.ProductOrder{}, err
		}
		result, err := tx.ExecContext(ctx, "update public.products set quantity=quantity-$1 where id=$2 and quantity >= $1", item.quantity, item.product.ID)
		if err != nil {
			return models.ProductOrder{}, err
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return models.ProductOrder{}, CheckoutError{"A product in your cart no longer has enough stock."}
		}
	}
	if _, err = tx.ExecContext(ctx, "delete from public.carts where customer_id=$1", customer.ID); err != nil {
		return models.ProductOrder{}, err
	}
	if err = tx.Commit(); err != nil {
		return models.ProductOrder{}, err
	}
	return order, nil
}

func checkoutCartMemory(customer models.Customer, key string) (models.ProductOrder, error) {
	mu.Lock()
	defer mu.Unlock()
	if key != "" {
		for _, previous := range productOrders {
			if previous.CustomerID == customer.ID && previous.CheckoutKey == key {
				return previous, nil
			}
		}
	}
	cart := carts[customer.ID]
	if len(cart) == 0 {
		return models.ProductOrder{}, ErrNotFound
	}
	order := models.ProductOrder{CheckoutKey: key, CustomerID: customer.ID, CustomerName: customer.FullName, CustomerPhone: customer.PhoneNumber, CustomerEmail: customer.Email, Status: models.OrderPending, CreatedAt: time.Now()}
	type line struct {
		index, quantity int
		subtotal        float64
	}
	lines := []line{}
	for productID, quantity := range cart {
		if quantity <= 0 {
			return models.ProductOrder{}, CheckoutError{"Invalid product quantity in cart."}
		}
		idx := -1
		for i := range products {
			if products[i].ID == productID {
				idx = i
				break
			}
		}
		if idx < 0 || !products[idx].IsAvailable {
			return models.ProductOrder{}, CheckoutError{"A product in your cart is no longer available."}
		}
		barber, found := findBarberByID(products[idx].BarberID)
		if !found || !barber.IsActive() {
			return models.ProductOrder{}, CheckoutError{"A product in your cart is no longer available for sale."}
		}
		if products[idx].Quantity < quantity {
			return models.ProductOrder{}, CheckoutError{"A product in your cart no longer has enough stock."}
		}
		sub := products[idx].Price * float64(quantity)
		order.TotalAmount += sub
		lines = append(lines, line{idx, quantity, sub})
	}
	order.ID = nextID(&lastProductOrderID)
	productOrders = append(productOrders, order)
	for _, ln := range lines {
		p := products[ln.index]
		products[ln.index].Quantity -= ln.quantity
		lastProductOrderItemID++
		productOrderItems = append(productOrderItems, models.ProductOrderItem{ID: lastProductOrderItemID, OrderID: order.ID, ProductID: p.ID, BarberID: p.BarberID, ProductName: p.Name, Price: p.Price, Quantity: ln.quantity, Subtotal: ln.subtotal})
	}
	delete(carts, customer.ID)
	return order, nil
}

// CreateProductOrder creates and stores a new product order.
func CreateProductOrder(order models.ProductOrder) (models.ProductOrder, error) {
	if databaseEnabled() {
		err := db.QueryRow("insert into public.product_orders(customer_id,customer_name,customer_phone,customer_email,total_amount,status,created_at) values($1,$2,$3,$4,$5,$6,$7) returning id", order.CustomerID, order.CustomerName, order.CustomerPhone, order.CustomerEmail, order.TotalAmount, order.Status, order.CreatedAt).Scan(&order.ID)
		return order, err
	}
	mu.Lock()
	defer mu.Unlock()

	order.ID = nextID(&lastProductOrderID)

	productOrders = append(productOrders, order)

	return order, nil
}

// CreateProductOrderItem creates and stores a product inside an order.
func CreateProductOrderItem(item models.ProductOrderItem) (models.ProductOrderItem, error) {
	if databaseEnabled() {
		err := db.QueryRow("insert into public.product_order_items(order_id,product_id,barber_id,product_name,price,quantity,subtotal) values($1,$2,$3,$4,$5,$6,$7) returning id", item.OrderID, item.ProductID, item.BarberID, item.ProductName, item.Price, item.Quantity, item.Subtotal).Scan(&item.ID)
		return item, err
	}
	mu.Lock()
	defer mu.Unlock()

	item.ID = nextID(&lastProductOrderItemID)

	productOrderItems = append(productOrderItems, item)

	return item, nil
}

// FindProductOrderByID finds an order by its ID.
func FindProductOrderByID(id int) (models.ProductOrder, bool) {
	if databaseEnabled() {
		var v models.ProductOrder
		err := db.QueryRow("select id,customer_id,customer_name,customer_phone,customer_email,total_amount,status,created_at from public.product_orders where id=$1", id).Scan(&v.ID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.TotalAmount, &v.Status, &v.CreatedAt)
		return v, err == nil
	}
	mu.RLock()
	defer mu.RUnlock()

	for _, order := range productOrders {
		if order.ID == id {
			return order, true
		}
	}

	return models.ProductOrder{}, false
}

// ListProductOrderItems returns all items belonging to an order.
func ListProductOrderItems(ctx context.Context, orderID int) ([]models.ProductOrderItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if databaseEnabled() {
		return queryRecords(ctx, "select id,order_id,product_id,barber_id,product_name,price,quantity,subtotal from public.product_order_items where order_id=$1 order by id", func(rows *sql.Rows) (models.ProductOrderItem, error) {
			var v models.ProductOrderItem
			err := rows.Scan(&v.ID, &v.OrderID, &v.ProductID, &v.BarberID, &v.ProductName, &v.Price, &v.Quantity, &v.Subtotal)
			return v, err
		}, orderID)
	}
	mu.RLock()
	defer mu.RUnlock()
	out := make([]models.ProductOrderItem, 0)
	for _, v := range productOrderItems {
		if v.OrderID == orderID {
			out = append(out, v)
		}
	}
	return out, nil
}

// ListProductOrdersByCustomer returns all orders belonging to a customer.
func ListProductOrdersByCustomer(ctx context.Context, customerID int) ([]models.ProductOrder, error) {
	if databaseEnabled() {
		rows, err := db.QueryContext(ctx, "select id,customer_id,customer_name,customer_phone,customer_email,total_amount,status,created_at from public.product_orders where customer_id=$1 order by id desc", customerID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		orders := make([]models.ProductOrder, 0)
		for rows.Next() {
			var order models.ProductOrder
			if err := rows.Scan(&order.ID, &order.CustomerID, &order.CustomerName, &order.CustomerPhone, &order.CustomerEmail, &order.TotalAmount, &order.Status, &order.CreatedAt); err != nil {
				return nil, err
			}
			orders = append(orders, order)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return orders, nil
	}
	mu.RLock()
	defer mu.RUnlock()
	orders := make([]models.ProductOrder, 0)
	for _, order := range productOrders {
		if order.CustomerID == customerID {
			orders = append(orders, order)
		}
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i].ID > orders[j].ID })
	return orders, nil
}

// ListProductOrderItemsByBarber returns all purchased product items
// belonging to a specific barber.
func ListProductOrderItemsByBarber(ctx context.Context, barberID int) ([]models.ProductOrderItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if databaseEnabled() {
		return queryRecords(ctx, `select i.id,i.order_id,i.product_id,i.barber_id,i.product_name,i.price,i.quantity,i.subtotal,o.customer_name,o.customer_phone,o.customer_email,o.status from public.product_order_items i join public.product_orders o on o.id=i.order_id where i.barber_id=$1 order by i.id desc`, func(rows *sql.Rows) (models.ProductOrderItem, error) {
			var v models.ProductOrderItem
			err := rows.Scan(&v.ID, &v.OrderID, &v.ProductID, &v.BarberID, &v.ProductName, &v.Price, &v.Quantity, &v.Subtotal, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.OrderStatus)
			return v, err
		}, barberID)
	}
	mu.RLock()
	defer mu.RUnlock()
	out := make([]models.ProductOrderItem, 0)
	for _, v := range productOrderItems {
		if v.BarberID != barberID {
			continue
		}
		for _, order := range productOrders {
			if order.ID == v.OrderID {
				v.CustomerName = order.CustomerName
				v.CustomerPhone = order.CustomerPhone
				v.CustomerEmail = order.CustomerEmail
				v.OrderStatus = order.Status
				break
			}
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func FindProductOrderByIDResult(ctx context.Context, id int) (models.ProductOrder, error) {
	if err := ctx.Err(); err != nil {
		return models.ProductOrder{}, err
	}
	if databaseEnabled() {
		var v models.ProductOrder
		err := db.QueryRowContext(ctx, "select id,customer_id,customer_name,customer_phone,customer_email,total_amount,status,created_at from public.product_orders where id=$1", id).Scan(&v.ID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.TotalAmount, &v.Status, &v.CreatedAt)
		return v, recordError(err)
	}
	v, ok := FindProductOrderByID(id)
	if !ok {
		return models.ProductOrder{}, ErrNotFound
	}
	return v, nil
}
