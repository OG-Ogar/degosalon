// Package money defines the NGN decimal contract used by the existing
// numeric(12,2) columns. Arithmetic is performed only in integer kobo.
package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Amount int64

const Max Amount = 999999999999 // ₦9,999,999,999.99, matching numeric(12,2).

var ErrInvalid = errors.New("amount must be between 0 and 9999999999.99 with at most two decimal places")

func Line(price float64, quantity int) (Amount, error) {
	amount, err := FromFloat(price)
	if err != nil {
		return 0, err
	}
	return amount.Multiply(quantity)
}

// Parse rejects excess precision rather than silently rounding submitted money.
func Parse(value string) (Amount, error) {
	value = strings.TrimSpace(value)
	whole, fraction, _ := strings.Cut(value, ".")
	if whole == "" {
		whole = "0"
	}
	if value == "" || value == "." || len(fraction) > 2 {
		return 0, ErrInvalid
	}
	for _, part := range []string{whole, fraction} {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return 0, ErrInvalid
			}
		}
	}
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	if len(whole) > 10 {
		return 0, ErrInvalid
	}
	major, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, ErrInvalid
	}
	minor, err := strconv.ParseInt(fraction+strings.Repeat("0", 2-len(fraction)), 10, 64)
	if err != nil {
		return 0, ErrInvalid
	}
	amount := Amount(major*100 + minor)
	if amount > Max {
		return 0, ErrInvalid
	}
	return amount, nil
}

// FromFloat validates legacy model/SQL scan boundary values. No floating-point
// multiplication or addition is used to decide business totals.
func FromFloat(value float64) (Amount, error) {
	return Parse(strconv.FormatFloat(value, 'f', -1, 64))
}

func (amount Amount) String() string { return fmt.Sprintf("%d.%02d", amount/100, amount%100) }

// Float is a compatibility adapter for existing models/templates, never arithmetic.
func (amount Amount) Float() float64 {
	value, _ := strconv.ParseFloat(amount.String(), 64)
	return value
}

func (amount Amount) Multiply(quantity int) (Amount, error) {
	if amount < 0 || amount > Max || quantity < 0 || (amount != 0 && int64(quantity) > int64(Max/amount)) {
		return 0, ErrInvalid
	}
	return amount * Amount(quantity), nil
}

func (amount Amount) Add(other Amount) (Amount, error) {
	if amount < 0 || other < 0 || amount > Max || other > Max-amount {
		return 0, ErrInvalid
	}
	return amount + other, nil
}
