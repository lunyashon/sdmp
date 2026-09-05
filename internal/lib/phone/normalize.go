package phone

import (
	"strings"
	"unicode"
)

func Normalize(raw string) (string, bool) {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	switch {
	case len(digits) == 11 && digits[0] == '8':
		digits = "7" + digits[1:]
	case len(digits) == 10:
		digits = "7" + digits
	}
	if len(digits) != 11 || digits[0] != '7' {
		return "", false
	}
	return digits, true
}
