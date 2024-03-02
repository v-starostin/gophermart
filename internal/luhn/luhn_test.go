package luhn_test

import (
	"testing"

	"github.com/v-starostin/go-musthave-diploma-tpl/internal/luhn"
)

func TestIsValid(t *testing.T) {
	tt := []struct {
		number   int
		expected bool
	}{
		{0, true},
		{5, false},
		{12, false},
		{42, true},
		{9259, false},
		{125, true},
	}

	for _, test := range tt {
		got := luhn.IsValid(test.number)
		if test.expected != got {
			t.Errorf("For %d: expected: %t, got: %t", test.number, test.expected, got)
		}
	}
}
