package storage

import (
	"testing"

	"barber-booking/models"
)

// newBarber stores an active barber and fails the test if it cannot.
func newBarber(t *testing.T, email string) models.Barber {
	t.Helper()

	barber, err := CreateBarber(models.Barber{
		FullName: "Test Barber",
		Email:    email,
		Status:   models.BarberActive,
	})

	if err != nil {
		t.Fatalf("CreateBarber(%q): %v", email, err)
	}

	return barber
}

// newCustomer stores a customer and fails the test if it cannot.
func newCustomer(t *testing.T, email string) models.Customer {
	t.Helper()

	customer, err := CreateCustomer(models.Customer{
		FullName: "Test Customer",
		Email:    email,
	})

	if err != nil {
		t.Fatalf("CreateCustomer(%q): %v", email, err)
	}

	return customer
}

// newHairstyle stores an available hairstyle for one barber.
func newHairstyle(t *testing.T, barberID int) models.Hairstyle {
	t.Helper()

	hairstyle, err := CreateHairstyle(models.Hairstyle{
		BarberID:    barberID,
		Name:        "Fade",
		Price:       2500,
		Duration:    30,
		IsAvailable: true,
	})

	if err != nil {
		t.Fatalf("CreateHairstyle: %v", err)
	}

	return hairstyle
}

// newBooking builds a pending booking for the given trio at 10:00.
func newBooking(customer models.Customer, barberID, hairstyleID int) models.Booking {
	return models.Booking{
		BarberID:      barberID,
		HairstyleID:   hairstyleID,
		CustomerID:    customer.ID,
		CustomerName:  customer.FullName,
		CustomerPhone: customer.PhoneNumber,
		CustomerEmail: customer.Email,
		Date:          "2030-01-01",
		Time:          "10:00",
		Status:        models.StatusPending,
	}
}

func TestCreateBookingRejectsTakenSlot(t *testing.T) {
	Reset()

	barber := newBarber(t, "slot@example.com")
	hairstyle := newHairstyle(t, barber.ID)
	first := newCustomer(t, "first-customer@example.com")
	second := newCustomer(t, "second-customer@example.com")

	if _, err := CreateBooking(newBooking(first, barber.ID, hairstyle.ID)); err != nil {
		t.Fatalf("first CreateBooking: %v", err)
	}

	if _, err := CreateBooking(newBooking(second, barber.ID, hairstyle.ID)); err != ErrSlotTaken {
		t.Fatalf("same slot twice: got %v, want ErrSlotTaken", err)
	}
}

func TestCancelledBookingFreesTheSlot(t *testing.T) {
	Reset()

	barber := newBarber(t, "freed@example.com")
	hairstyle := newHairstyle(t, barber.ID)
	first := newCustomer(t, "cancels@example.com")
	second := newCustomer(t, "rebooks@example.com")

	booking, err := CreateBooking(newBooking(first, barber.ID, hairstyle.ID))
	if err != nil {
		t.Fatalf("first CreateBooking: %v", err)
	}

	// The customer passes their own ID, so the store checks ownership
	// inside the write lock rather than trusting the request.
	if err := SetBookingStatus(booking.ID, models.StatusCancelled, 0, first.ID); err != nil {
		t.Fatalf("SetBookingStatus: %v", err)
	}

	if _, err := CreateBooking(newBooking(second, barber.ID, hairstyle.ID)); err != nil {
		t.Fatalf("rebooking a cancelled slot: %v", err)
	}
}

func TestSetBookingStatusRefusesOtherCustomer(t *testing.T) {
	Reset()

	barber := newBarber(t, "guard@example.com")
	hairstyle := newHairstyle(t, barber.ID)
	owner := newCustomer(t, "owner-customer@example.com")
	other := newCustomer(t, "other-customer@example.com")

	booking, err := CreateBooking(newBooking(owner, barber.ID, hairstyle.ID))
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}

	if err := SetBookingStatus(booking.ID, models.StatusCancelled, 0, other.ID); err != ErrForbidden {
		t.Fatalf("cancel as another customer: got %v, want ErrForbidden", err)
	}

	stored, found := FindBookingByID(booking.ID)
	if !found {
		t.Fatal("booking missing after refused cancel")
	}

	if stored.Status != models.StatusPending {
		t.Fatalf("status = %q, want it untouched", stored.Status)
	}
}

func TestCreateBarberRejectsDuplicateEmail(t *testing.T) {
	Reset()

	newBarber(t, "one@example.com")

	if _, err := CreateBarber(models.Barber{
		Email:  "one@example.com",
		Status: models.BarberActive,
	}); err != ErrEmailTaken {
		t.Fatalf("second CreateBarber: got %v, want ErrEmailTaken", err)
	}
}

func TestIDsDoNotCollideAfterDelete(t *testing.T) {
	Reset()

	first := newBarber(t, "first@example.com")
	second := newBarber(t, "second@example.com")

	if _, err := DeleteBarber(second.ID); err != nil {
		t.Fatalf("DeleteBarber: %v", err)
	}

	third := newBarber(t, "third@example.com")

	// len(barbers)+1 would have handed out second.ID again here, so the
	// new barber would have inherited the deleted one's bookings.
	if third.ID == second.ID || third.ID == first.ID {
		t.Fatalf("reused ID %d (first %d, deleted %d)",
			third.ID, first.ID, second.ID)
	}
}

func TestUpdateBarberKeepsPasswordWhenBlank(t *testing.T) {
	Reset()

	barber := newBarber(t, "keep@example.com")
	barber.PasswordHash = "original-hash"

	if err := UpdateBarber(barber); err != nil {
		t.Fatalf("UpdateBarber with hash: %v", err)
	}

	barber.PasswordHash = ""
	barber.FullName = "Renamed"

	if err := UpdateBarber(barber); err != nil {
		t.Fatalf("UpdateBarber without hash: %v", err)
	}

	stored, found := FindBarberByID(barber.ID)
	if !found {
		t.Fatal("barber missing after update")
	}

	if stored.PasswordHash != "original-hash" {
		t.Fatalf("password hash = %q, want it preserved", stored.PasswordHash)
	}

	if stored.FullName != "Renamed" {
		t.Fatalf("full name = %q, want Renamed", stored.FullName)
	}
}

func TestFindHairstyleOwnedByRefusesOtherBarber(t *testing.T) {
	Reset()

	owner := newBarber(t, "owner@example.com")
	other := newBarber(t, "other@example.com")
	hairstyle := newHairstyle(t, owner.ID)

	if _, err := FindHairstyleOwnedBy(hairstyle.ID, other.ID); err != ErrForbidden {
		t.Fatalf("cross-barber read: got %v, want ErrForbidden", err)
	}

	if _, err := FindHairstyleOwnedBy(hairstyle.ID, owner.ID); err != nil {
		t.Fatalf("owner read: %v", err)
	}
}


func TestCreateBookingRejectsDurationOverlap(t *testing.T) {
	Reset()
	barber := newBarber(t, "overlap@example.com")
	style := newHairstyle(t, barber.ID)
	first := newCustomer(t, "overlap-first@example.com")
	second := newCustomer(t, "overlap-second@example.com")

	booking := newBooking(first, barber.ID, style.ID)
	booking.Time = "10:00"
	if _, err := CreateBooking(booking); err != nil {
		t.Fatalf("first CreateBooking: %v", err)
	}
	overlap := newBooking(second, barber.ID, style.ID)
	overlap.Time = "10:15"
	if _, err := CreateBooking(overlap); err != ErrSlotConflict {
		t.Fatalf("overlapping CreateBooking: got %v, want ErrSlotConflict", err)
	}
}

func TestCreateBookingAllowsAdjacentIntervals(t *testing.T) {
	Reset()
	barber := newBarber(t, "adjacent@example.com")
	style := newHairstyle(t, barber.ID)
	first := newCustomer(t, "adjacent-first@example.com")
	second := newCustomer(t, "adjacent-second@example.com")

	if _, err := CreateBooking(newBooking(first, barber.ID, style.ID)); err != nil {
		t.Fatalf("first CreateBooking: %v", err)
	}
	adjacent := newBooking(second, barber.ID, style.ID)
	adjacent.Time = "10:30"
	if _, err := CreateBooking(adjacent); err != nil {
		t.Fatalf("adjacent CreateBooking: %v", err)
	}
}

func TestTransitionBookingStatusRejectsStaleSourceState(t *testing.T) {
	Reset()
	barber := newBarber(t, "transition@example.com")
	style := newHairstyle(t, barber.ID)
	customer := newCustomer(t, "transition-customer@example.com")
	booking, err := CreateBooking(newBooking(customer, barber.ID, style.ID))
	if err != nil { t.Fatalf("CreateBooking: %v", err) }

	if err := TransitionBookingStatus(booking.ID, models.StatusAccepted, []string{models.StatusPending}, barber.ID, 0); err != nil {
		t.Fatalf("accept pending booking: %v", err)
	}
	if err := TransitionBookingStatus(booking.ID, models.StatusRejected, []string{models.StatusPending}, barber.ID, 0); err != ErrInvalidTransition {
		t.Fatalf("stale transition: got %v, want ErrInvalidTransition", err)
	}
	stored, found := FindBookingByID(booking.ID)
	if !found || stored.Status != models.StatusAccepted {
		t.Fatalf("status after stale transition = %q, want Accepted", stored.Status)
	}
}


func TestCartDoesNotConsumeStockUntilCheckout(t *testing.T) {
	Reset()
	barber:=newBarber(t,"cart-stock@example.com")
	customer:=newCustomer(t,"cart-stock-customer@example.com")
	product,err:=CreateProduct(models.Product{BarberID:barber.ID,Name:"Pomade",Price:2500,Quantity:2,IsAvailable:true})
	if err!=nil{t.Fatal(err)}
	if _,err=AddToCart(customer.ID,product.ID);err!=nil{t.Fatal(err)}
	stored,_:=FindProductByID(product.ID)
	if stored.Quantity!=2{t.Fatalf("stock after add-to-cart=%d, want 2",stored.Quantity)}
	if _,err=CheckoutCart(customer);err!=nil{t.Fatal(err)}
	stored,_=FindProductByID(product.ID)
	if stored.Quantity!=1{t.Fatalf("stock after checkout=%d, want 1",stored.Quantity)}
}

func TestCheckoutRejectsInactiveSellerWithoutMutatingCartOrStock(t *testing.T) {
	Reset()
	barber:=newBarber(t,"inactive-seller@example.com")
	customer:=newCustomer(t,"inactive-seller-customer@example.com")
	product,err:=CreateProduct(models.Product{BarberID:barber.ID,Name:"Oil",Price:1500,Quantity:3,IsAvailable:true});if err!=nil{t.Fatal(err)}
	if _,err=AddToCart(customer.ID,product.ID);err!=nil{t.Fatal(err)}
	for i:=range barbers{if barbers[i].ID==barber.ID{barbers[i].Status=models.BarberSuspended}}
	if _,err=CheckoutCart(customer);err==nil{t.Fatal("checkout succeeded for inactive seller")}
	if CartItemCount(customer.ID)!=1{t.Fatal("failed checkout cleared cart")}
	stored,_:=FindProductByID(product.ID);if stored.Quantity!=3{t.Fatalf("failed checkout changed stock to %d",stored.Quantity)}
}
