package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
)

func ListBarbersResult(ctx context.Context) ([]models.Barber, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !databaseEnabled() {
		return ListBarbers(), nil
	}
	return queryRecords(ctx, "select id,full_name,phone_number,email,password_hash,shop_name,shop_address,state,local_area,profile_image,personal_image,status from public.barbers order by id", func(rows *sql.Rows) (models.Barber, error) {
		var v models.Barber
		err := rows.Scan(&v.ID, &v.FullName, &v.PhoneNumber, &v.Email, &v.PasswordHash, &v.ShopName, &v.ShopAddress, &v.State, &v.LocalArea, &v.ProfileImage, &v.PersonalImage, &v.Status)
		return v, err
	})
}

func ListActiveBarbersResult(ctx context.Context) ([]models.Barber, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !databaseEnabled() {
		return ListActiveBarbers(), nil
	}
	return queryRecords(ctx, "select id,full_name,phone_number,email,password_hash,shop_name,shop_address,state,local_area,profile_image,personal_image,status from public.barbers where status='Active' order by id", func(rows *sql.Rows) (models.Barber, error) {
		var v models.Barber
		err := rows.Scan(&v.ID, &v.FullName, &v.PhoneNumber, &v.Email, &v.PasswordHash, &v.ShopName, &v.ShopAddress, &v.State, &v.LocalArea, &v.ProfileImage, &v.PersonalImage, &v.Status)
		return v, err
	})
}

func ListCustomersResult(ctx context.Context) ([]models.Customer, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !databaseEnabled() {
		return ListCustomers(), nil
	}
	return queryRecords(ctx, "select id,full_name,phone_number,email,password_hash,profile_image from public.customers order by id", func(rows *sql.Rows) (models.Customer, error) {
		var v models.Customer
		err := rows.Scan(&v.ID, &v.FullName, &v.PhoneNumber, &v.Email, &v.PasswordHash, &v.ProfileImage)
		return v, err
	})
}

func ListHairstylesByBarberResult(ctx context.Context, id int) ([]models.Hairstyle, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !databaseEnabled() {
		return ListHairstylesByBarber(id), nil
	}
	return queryRecords(ctx, "select id,barber_id,name,category,description,price,duration,image,video,is_available from public.hairstyles where barber_id=$1 order by id", func(rows *sql.Rows) (models.Hairstyle, error) {
		var v models.Hairstyle
		err := rows.Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable)
		return v, err
	}, id)
}

func ListAvailableHairstylesByBarberResult(ctx context.Context, id int) ([]models.Hairstyle, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !databaseEnabled() {
		return ListAvailableHairstylesByBarber(id), nil
	}
	return queryRecords(ctx, "select id,barber_id,name,category,description,price,duration,image,video,is_available from public.hairstyles where barber_id=$1 and is_available=true order by id", func(rows *sql.Rows) (models.Hairstyle, error) {
		var v models.Hairstyle
		err := rows.Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable)
		return v, err
	}, id)
}

func ListBookingsResult(ctx context.Context) ([]models.Booking, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !databaseEnabled() {
		return ListBookings(), nil
	}
	return queryRecords(ctx, "select id,barber_id,hairstyle_id,customer_id,customer_name,customer_phone,customer_email,booking_date::text,booking_time::text,notes,status,created_at,barber_name,shop_name,shop_address,service_name,service_price,service_duration,coalesce(booking_timezone,'') from public.bookings order by booking_date,booking_time,id", func(rows *sql.Rows) (models.Booking, error) {
		var v models.Booking
		err := rows.Scan(&v.ID, &v.BarberID, &v.HairstyleID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.Date, &v.Time, &v.Notes, &v.Status, &v.CreatedAt, &v.BarberName, &v.ShopName, &v.ShopAddress, &v.ServiceName, &v.ServicePrice, &v.ServiceDuration, &v.BookingTimezone)
		return v, err
	})
}

func ListBookingsByBarberResult(ctx context.Context, id int) ([]models.Booking, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !databaseEnabled() {
		return ListBookingsByBarber(id), nil
	}
	return queryRecords(ctx, "select id,barber_id,hairstyle_id,customer_id,customer_name,customer_phone,customer_email,booking_date::text,booking_time::text,notes,status,created_at,barber_name,shop_name,shop_address,service_name,service_price,service_duration,coalesce(booking_timezone,'') from public.bookings where barber_id=$1 order by booking_date,booking_time,id", func(rows *sql.Rows) (models.Booking, error) {
		var v models.Booking
		err := rows.Scan(&v.ID, &v.BarberID, &v.HairstyleID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.Date, &v.Time, &v.Notes, &v.Status, &v.CreatedAt, &v.BarberName, &v.ShopName, &v.ShopAddress, &v.ServiceName, &v.ServicePrice, &v.ServiceDuration, &v.BookingTimezone)
		return v, err
	}, id)
}

func ListBookingsByCustomerResult(ctx context.Context, id int) ([]models.Booking, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !databaseEnabled() {
		return ListBookingsByCustomer(id), nil
	}
	return queryRecords(ctx, "select id,barber_id,hairstyle_id,customer_id,customer_name,customer_phone,customer_email,booking_date::text,booking_time::text,notes,status,created_at,barber_name,shop_name,shop_address,service_name,service_price,service_duration,coalesce(booking_timezone,'') from public.bookings where customer_id=$1 order by booking_date,booking_time,id", func(rows *sql.Rows) (models.Booking, error) {
		var v models.Booking
		err := rows.Scan(&v.ID, &v.BarberID, &v.HairstyleID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.Date, &v.Time, &v.Notes, &v.Status, &v.CreatedAt, &v.BarberName, &v.ShopName, &v.ShopAddress, &v.ServiceName, &v.ServicePrice, &v.ServiceDuration, &v.BookingTimezone)
		return v, err
	}, id)
}

func ListReviewsByBarberResult(ctx context.Context, id int) ([]models.Review, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !databaseEnabled() {
		return ListReviewsByBarber(id), nil
	}
	return queryRecords(ctx, "select id,barber_id,customer_id,booking_id,customer_name,rating,comment,created_at from public.reviews where barber_id=$1 order by created_at desc,id desc", func(rows *sql.Rows) (models.Review, error) {
		var v models.Review
		err := rows.Scan(&v.ID, &v.BarberID, &v.CustomerID, &v.BookingID, &v.CustomerName, &v.Rating, &v.Comment, &v.CreatedAt)
		return v, err
	}, id)
}

func FindBookingByIDResult(ctx context.Context, id int) (models.Booking, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return models.Booking{}, err
	}
	if !databaseEnabled() {
		v, ok := FindBookingByID(id)
		if !ok {
			return models.Booking{}, ErrNotFound
		}
		return v, nil
	}
	var v models.Booking
	err := db.QueryRowContext(ctx, "select id,barber_id,hairstyle_id,customer_id,customer_name,customer_phone,customer_email,booking_date::text,booking_time::text,notes,status,created_at,barber_name,shop_name,shop_address,service_name,service_price,service_duration,coalesce(booking_timezone,'') from public.bookings where id=$1", id).Scan(&v.ID, &v.BarberID, &v.HairstyleID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.Date, &v.Time, &v.Notes, &v.Status, &v.CreatedAt, &v.BarberName, &v.ShopName, &v.ShopAddress, &v.ServiceName, &v.ServicePrice, &v.ServiceDuration, &v.BookingTimezone)
	return v, recordError(err)
}

func ReviewSummaryResult(ctx context.Context, id int) (float64, int, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}
	if !databaseEnabled() {
		average, count := ReviewSummary(id)
		return average, count, nil
	}
	var average float64
	var count int
	err := db.QueryRowContext(ctx, "select coalesce(avg(rating),0),count(*) from public.reviews where barber_id=$1", id).Scan(&average, &count)
	return average, count, err
}

func CanCustomerReviewResult(ctx context.Context, customerID, barberID int) (models.Booking, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return models.Booking{}, err
	}
	if !databaseEnabled() {
		booking, ok := CanCustomerReview(customerID, barberID)
		if !ok {
			return models.Booking{}, ErrNotFound
		}
		return booking, nil
	}
	var v models.Booking
	err := db.QueryRowContext(ctx, `select b.id,b.barber_id,b.hairstyle_id,b.customer_id,b.customer_name,b.customer_phone,b.customer_email,b.booking_date::text,b.booking_time::text,b.notes,b.status,b.created_at,b.barber_name,b.shop_name,b.shop_address,b.service_name,b.service_price,b.service_duration,coalesce(b.booking_timezone,'')
 from public.bookings b where b.customer_id=$1 and b.barber_id=$2 and b.status='Completed'
 and not exists(select 1 from public.reviews r where r.booking_id=b.id)
 order by b.booking_date,b.booking_time,b.id limit 1`, customerID, barberID).Scan(&v.ID, &v.BarberID, &v.HairstyleID, &v.CustomerID, &v.CustomerName, &v.CustomerPhone, &v.CustomerEmail, &v.Date, &v.Time, &v.Notes, &v.Status, &v.CreatedAt, &v.BarberName, &v.ShopName, &v.ShopAddress, &v.ServiceName, &v.ServicePrice, &v.ServiceDuration, &v.BookingTimezone)
	return v, recordError(err)
}
