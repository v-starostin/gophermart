package luhn_test

import (
	"testing"

	"github.com/v-starostin/gophermart/internal/luhn"
)

func TestIsValid(t *testing.T) {
	tt := []struct {
		orderNumber string
		expected    bool
	}{
		{"0", true},
		{"5", false},
		{"12", false},
		{"42", true},
		{"9259", false},
		{"125", true},
	}

	for _, test := range tt {
		got := luhn.IsValid(test.orderNumber)
		if test.expected != got {
			t.Errorf("For %s: expected: %t, got: %t", test.orderNumber, test.expected, got)
		}
	}
}
