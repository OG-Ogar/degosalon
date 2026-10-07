package storage

import (
	"barber-booking/internal/money"
	"barber-booking/models"
	"context"
	"errors"
	"math"
	"testing"
)

func TestMoneyCheckout(t *testing.T) { Reset(); t.Cleanup(Reset); checkMoneyCheckout(t) }

func checkMoneyCheckout(t *testing.T) {
	ctx := context.Background()
	seller := newBarber(t, "money_seller@example.com")
	buyer := newCustomer(t, "money_buyer@example.com")
	for _, price := range []float64{math.NaN(), math.Inf(1), -.01, .001, 1e10} {
		if _, err := CreateProduct(models.Product{BarberID: seller.ID, Price: price}); !errors.Is(err, money.ErrInvalid) {
			t.Fatalf("product %v: %v", price, err)
		}
		if _, err := CreateHairstyle(models.Hairstyle{BarberID: seller.ID, Price: price}); !errors.Is(err, money.ErrInvalid) {
			t.Fatalf("service %v: %v", price, err)
		}
		if _, err := CreateProductOrder(models.ProductOrder{CustomerID: buyer.ID, TotalAmount: price}); !errors.Is(err, money.ErrInvalid) {
			t.Fatalf("order %v: %v", price, err)
		}
	}
	var productsToBuy []models.Product
	for index, price := range []float64{.10, .20} {
		product, err := CreateProduct(models.Product{BarberID: seller.ID, Name: "Fractional product", Price: price, Quantity: 3, IsAvailable: true})
		if err != nil {
			t.Fatal(err)
		}
		productsToBuy = append(productsToBuy, product)
		if _, err := AddToCart(buyer.ID, product.ID); err != nil {
			t.Fatal(index, err)
		}
	}
	order, err := CheckoutCart(buyer)
	if err != nil || order.TotalAmount != .30 {
		t.Fatalf("decimal total = %.18f: %v", order.TotalAmount, err)
	}
	large, err := CreateProduct(models.Product{BarberID: seller.ID, Name: "Bounded product", Price: money.Max.Float(), Quantity: 2, IsAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddToCart(buyer.ID, large.ID); err != nil {
		t.Fatal(err)
	}
	if err := SetCartQuantity(ctx, buyer.ID, large.ID, 2); err != nil {
		t.Fatal(err)
	}
	key := "33333333333333333333333333333333"
	_, err = CheckoutCartWithKey(ctx, buyer, key)
	var domainErr CheckoutError
	if !errors.As(err, &domainErr) {
		t.Fatalf("overflow checkout = %v", err)
	}
	stored, err := FindProductByIDResult(ctx, large.ID)
	if err != nil || stored.Quantity != 2 {
		t.Fatalf("overflow consumed stock: %+v %v", stored, err)
	}
	cart, err := ListCartItems(ctx, buyer.ID)
	if err != nil || len(cart) != 1 || cart[0].Quantity != 2 {
		t.Fatalf("overflow consumed cart: %+v %v", cart, err)
	}
	orders, err := ListProductOrdersByCustomer(ctx, buyer.ID)
	if err != nil || len(orders) != 1 {
		t.Fatalf("overflow persisted order: %+v %v", orders, err)
	}
	// An overflow rejection must not claim the replay key.
	if err := SetCartQuantity(ctx, buyer.ID, large.ID, 1); err != nil {
		t.Fatal(err)
	}
	retry, err := CheckoutCartWithKey(ctx, buyer, key)
	if err != nil || retry.TotalAmount != money.Max.Float() {
		t.Fatalf("bounded retry: %+v %v", retry, err)
	}
	for _, price := range []float64{6000000000, 5000000000} {
		product, err := CreateProduct(models.Product{BarberID: seller.ID, Name: "Sum limit", Price: price, Quantity: 1, IsAvailable: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := AddToCart(buyer.ID, product.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CheckoutCart(buyer); !errors.As(err, &domainErr) {
		t.Fatalf("sum overflow accepted: %v", err)
	}
	remaining, err := ListCartItems(ctx, buyer.ID)
	if err != nil || len(remaining) != 2 {
		t.Fatalf("sum overflow changed cart: %+v %v", remaining, err)
	}
}
