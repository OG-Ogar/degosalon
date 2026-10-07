package money

import (
	"math"
	"testing"
)

func TestDecimalContract(t *testing.T) {
	for text, want := range map[string]Amount{"0": 0, ".29": 29, "1.2": 120, "9999999999.99": Max, "0001.01": 101} {
		got, err := Parse(text)
		if err != nil || got != want {
			t.Fatalf("%q: %v %v", text, got, err)
		}
		if roundTrip, err := FromFloat(got.Float()); err != nil || roundTrip != got {
			t.Fatalf("roundtrip %q: %v %v", text, roundTrip, err)
		}
	}
	for _, text := range []string{"", ".", "NaN", "Inf", "-1", "1e2", "1.001", "1.2.3", "10000000000", "999999999999999999999999"} {
		if _, err := Parse(text); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), -.01, .001, 1e10} {
		if _, err := FromFloat(value); err == nil {
			t.Fatalf("accepted %v", value)
		}
	}
}

func TestExactArithmeticAndBounds(t *testing.T) {
	a, _ := Parse("0.10")
	b, _ := Parse("0.20")
	total, err := a.Add(b)
	if err != nil || total.String() != "0.30" {
		t.Fatal(total, err)
	}
	line, err := Amount(29).Multiply(3)
	if err != nil || line.String() != "0.87" {
		t.Fatal(line, err)
	}
	if _, err := Max.Multiply(2); err == nil {
		t.Fatal("multiplication overflow accepted")
	}
	if _, err := Max.Add(1); err == nil {
		t.Fatal("sum overflow accepted")
	}
	if _, err := Amount(1).Multiply(-1); err == nil {
		t.Fatal("negative quantity accepted")
	}
}
