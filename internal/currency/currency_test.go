package currency_test

import (
	"testing"

	"github.com/v-starostin/gophermart/internal/currency"
)

func TestConvertToPrimary(t *testing.T) {
	tt := []struct {
		value    int64
		expected float64
	}{
		{1234, 12.34},
		{567, 5.67},
		{32, 0.32},
		{0, 0},
	}

	for _, test := range tt {
		got := currency.ConvertToPrimary(test.value)
		if got != test.expected {
			t.Errorf("got %v, expected %v", got, test.expected)
		}
	}
}

func TestConvertToSubunit(t *testing.T) {
	tt := []struct {
		value    float64
		expected int64
	}{
		{12.34, 1234},
		{565.7, 56570},
		{32, 3200},
		{0, 0},
		{0.2, 20},
	}

	for _, test := range tt {
		got := currency.ConvertToSubunit(test.value)
		if got != test.expected {
			t.Errorf("got %v, expected %v", got, test.expected)
		}
	}
}
