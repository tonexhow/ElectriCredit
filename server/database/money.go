package database

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// MoneyScale stores Philippine peso values as integers with five decimal
// places. 1 peso = 100000 units, so ₱0.00012 is stored exactly as 12.
const MoneyScale int64 = 100000

// ParseMoneyUnits converts a decimal peso string into fixed five-decimal
// integer units without using floating point arithmetic.
func ParseMoneyUnits(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("money value is required")
	}

	negative := false
	if value[0] == '-' || value[0] == '+' {
		negative = value[0] == '-'
		value = value[1:]
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("invalid money value")
	}

	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, errors.New("invalid money value")
	}

	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 5 {
		return 0, errors.New("money values support at most 5 decimal places")
	}
	for _, r := range fraction {
		if r < '0' || r > '9' {
			return 0, errors.New("invalid money value")
		}
	}
	fraction += strings.Repeat("0", 5-len(fraction))

	fractionUnits := int64(0)
	if fraction != "" {
		fractionUnits, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, errors.New("invalid money value")
		}
	}

	if whole > (1<<63-1-fractionUnits)/MoneyScale {
		return 0, errors.New("money value is too large")
	}

	units := whole*MoneyScale + fractionUnits
	if negative {
		units = -units
	}
	return units, nil
}

// FormatMoneyUnits returns a canonical decimal peso value with five decimal
// places. The frontend may trim trailing zeros for display.
func FormatMoneyUnits(units int64) string {
	negative := units < 0
	if negative {
		units = -units
	}

	whole := units / MoneyScale
	fraction := units % MoneyScale
	prefix := ""
	if negative {
		prefix = "-"
	}

	return fmt.Sprintf("%s%d.%05d", prefix, whole, fraction)
}
