package storage

import (
	"barber-booking/models"
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestDiscoveryPages(t *testing.T) { Reset(); t.Cleanup(Reset); checkDiscoveryPages(t) }
func checkDiscoveryPages(t *testing.T) {
	ctx := context.Background()
	var shops []models.Barber
	create := func(state, area, status string) models.Barber {
		b, err := CreateBarber(models.Barber{FullName: "Public barber", Email: fmt.Sprintf("discovery-%d@example.com", len(shops)), PhoneNumber: "private phone", PasswordHash: "private hash", ShopName: "Shop", State: state, LocalArea: area, Status: status})
		if err != nil {
			t.Fatal(err)
		}
		shops = append(shops, b)
		return b
	}
	for i := 0; i < 26; i++ {
		create("LAGOS state", "Ikeja", models.BarberActive)
	}
	create("Lagos", "Agege", models.BarberActive)
	create("Lagos", "Victoria Island", models.BarberActive)
	create("FCT", "Abuja Municipal", models.BarberActive)
	create("Lagos", "Ikeja", models.BarberSuspended)
	create("Lagos", "Ikeja", models.BarberArchived)
	first := shops[0]
	var styles []models.Hairstyle
	for i, price := range []float64{1000, 500, 1, 2} {
		h, err := CreateHairstyle(models.Hairstyle{BarberID: first.ID, Name: "Style", Price: price, Duration: 30, IsAvailable: i != 2})
		if err != nil {
			t.Fatal(err)
		}
		styles = append(styles, h)
	}
	if _, err := DeleteHairstyleContext(ctx, styles[3].ID, first.ID); err != nil {
		t.Fatal(err)
	}
	for i, rating := range []int{5, 3} {
		buyer := newCustomer(t, fmt.Sprintf("discovery-review-%d@example.com", i))
		request := newBooking(buyer, first.ID, styles[0].ID)
		request.Time = fmt.Sprintf("%02d:00", 10+i)
		booking, err := CreateBooking(request)
		if err != nil {
			t.Fatal(err)
		}
		if err = TransitionBookingStatus(booking.ID, models.StatusAccepted, []string{models.StatusPending}, first.ID, 0); err != nil {
			t.Fatal(err)
		}
		if err = TransitionBookingStatus(booking.ID, models.StatusCompleted, []string{models.StatusAccepted}, first.ID, 0); err != nil {
			t.Fatal(err)
		}
		if _, err = CreateReview(ctx, models.Review{CustomerID: buyer.ID, BarberID: first.ID, Rating: rating}); err != nil {
			t.Fatal(err)
		}
	}
	filter := DiscoveryFilter{StateID: "NG025", AreaID: "NG025011"}
	one, err := DiscoveryPageResult(ctx, filter)
	if err != nil || one.Total != 26 || len(one.Items) != 24 || one.HasPrevious || !one.HasNext {
		t.Fatalf("first page %+v %v", one, err)
	}
	if b := one.Items[0]; b.Hairstyles != 2 || b.FromPrice != 500 || b.ReviewCount != 2 || b.Rating != 4 || b.PasswordHash != "" || b.Email != "" || b.PhoneNumber != "" {
		t.Fatalf("incorrect or private summary %+v", b)
	}
	filter.After = one.Items[23].ID
	two, err := DiscoveryPageResult(ctx, filter)
	if err != nil || len(two.Items) != 2 || !two.HasPrevious || two.HasNext {
		t.Fatalf("second page %+v %v", two, err)
	}
	if two.Items[0].ID <= one.Items[23].ID {
		t.Fatal("duplicate cursor boundary")
	}
	filter.After = 0
	filter.Before = two.Items[0].ID
	back, err := DiscoveryPageResult(ctx, filter)
	if err != nil || len(back.Items) != 24 || back.HasPrevious || !back.HasNext {
		t.Fatalf("back page %+v %v", back, err)
	}
	for i, b := range back.Items {
		if b.ID != one.Items[i].ID {
			t.Fatal("backward page changed order")
		}
	}
	added := create("Lagos", "Ikeja", models.BarberActive)
	if err := ArchiveBarber(ctx, shops[10].ID); err != nil {
		t.Fatal(err)
	}
	filter.Before = 0
	filter.After = one.Items[23].ID
	changed, err := DiscoveryPageResult(ctx, filter)
	if err != nil || changed.Total != 26 || len(changed.Items) != 3 || changed.Items[2].ID != added.ID {
		t.Fatalf("insert/delete shifted cursor %+v %v", changed, err)
	}
	filter.After = 0
	filter.AreaID = ""
	whole, err := DiscoveryPageResult(ctx, filter)
	if err != nil || whole.Total != 28 {
		t.Fatalf("whole state lost unknown legacy area %+v %v", whole, err)
	}
	for _, bad := range []DiscoveryFilter{{StateID: "bad"}, {StateID: "Lagos", AreaID: "Abuja Municipal"}, {StateID: "Lagos", After: 1, Before: 1}, {StateID: "Lagos", Limit: 49}, {StateID: "Lagos", After: -1}} {
		if _, err := DiscoveryPageResult(ctx, bad); !errors.Is(err, ErrInvalidDiscoveryFilter) {
			t.Fatalf("invalid filter accepted %+v %v", bad, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := DiscoveryPageResult(canceled, filter); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
	empty, err := DiscoveryPageResult(ctx, DiscoveryFilter{})
	if err != nil || len(empty.Items) != 0 || empty.Total != 0 {
		t.Fatal("unscoped discovery", empty, err)
	}
}
