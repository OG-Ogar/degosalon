package handlers

import (
    "strings"
    "testing"
)

func TestNavigationPreservesDiscoveryAndActiveDestination(t *testing.T) {
    view := navigation("customer", "barbers", struct { CSRF, ResultsLink string }{CSRF: "token", ResultsLink: "/customer/barbers?state=Lagos&area=Ikeja"})
    if view.CSRF != "token" { t.Fatal("missing logout CSRF") }
    found := false
    for _, link := range view.Links {
        if link.Active {
            found = true
            if link.Path != "/customer/barbers?state=Lagos&area=Ikeja" { t.Fatalf("lost discovery context: %s", link.Path) }
        }
        if !strings.HasPrefix(link.Path, "/customer/") { t.Fatal("cross-role destination") }
    }
    if !found { t.Fatal("no active destination") }
    barber := navigation("barber", "schedule", nil)
    for _, link := range barber.Links {
        if link.Active && link.Path == "/barber/schedule" { return }
    }
    t.Fatal("missing barber schedule")
}
func TestNairaPresentation(t *testing.T) {
    for _, item := range []struct { amount float64; want string }{{9000, "₦9,000.00"}, {150000.5, "₦150,000.50"}, {0, "₦0.00"}} {
        if got := naira(item.amount); got != item.want { t.Fatalf("got %q, want %q", got, item.want) }
    }
}
