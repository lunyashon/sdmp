package phone

import (
	"strings"
	"unicode"
)

func Digits(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ForceCountry7(digits string) string {
	switch {
	case len(digits) == 10:
		return "7" + digits
	case len(digits) == 11:
		return "7" + digits[1:]
	default:
		return digits
	}
}

func Normalize(raw string) (string, bool) {
	digits := ForceCountry7(Digits(raw))
	if len(digits) != 11 {
		return "", false
	}
	return digits, true
}
