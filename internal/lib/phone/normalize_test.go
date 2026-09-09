package phone

import "testing"

func TestNormalizeAlwaysSetsFirstDigitTo7(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"89001234567", "79001234567", true},
		{"79001234567", "79001234567", true},
		{"99001234567", "79001234567", true},
		{"9001234567", "79001234567", true},
		{"+7 900 123-45-67", "79001234567", true},
		{"8 (900) 123 45 67", "79001234567", true},
		{"dgdfgdfg", "", false},
		{"123", "", false},
	}
	for _, tc := range cases {
		got, ok := Normalize(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("%q: got (%q, %v) want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
