package agreement

import (
	"fmt"
	"math"

	"golang.org/x/text/currency"
)

func NewMoney(major float64, code string) (Money, error) {
	unit, err := currency.ParseISO(code)
	if err != nil {
		return Money{}, fmt.Errorf("invalid currency %q: %w", code, err)
	}
	scale, _ := currency.Standard.Rounding(unit) // scale = number of decimal digits
	minor := int64(math.Round(major * math.Pow10(scale)))
	return Money{AmountMinor: minor, Currency: unit.String()}, nil
}

func (m Money) Major() (float64, error) {
	unit, err := currency.ParseISO(m.Currency)
	if err != nil {
		return 0, err
	}
	scale, _ := currency.Standard.Rounding(unit)
	return float64(m.AmountMinor) / math.Pow10(scale), nil
}
