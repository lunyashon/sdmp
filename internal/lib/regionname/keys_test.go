package regionname

import (
	"strings"
	"testing"
)

func containsFold(keys []string, want string) bool {
	for _, k := range keys {
		if strings.EqualFold(k, want) {
			return true
		}
	}
	return false
}

func TestLookupKeysHitsExistingAliases(t *testing.T) {
	cases := []struct {
		in, alias string
	}{
		{"Город Москва, Московская область", "г. Москва и Московская область"},
		{"Город Санкт-Петербург, Ленинградская область", "г. Санкт-Петербург и Ленинградская область"},
		{"Кемеровская область-Кузбасс", "Кемеровская область - Кузбасс"},
		{"Чувашская Республика-Чувашия", "Чувашская Республика - Чувашия"},
		{"Республика Северная Осетия-Алания", "Республика Северная Осетия - Алания"},
		{"Архангельская область, Ненецкий автономный округ", "Архангельская область и Ненецкий автономный округ"},
		{"Город Москва", "г. Москва"},
		{"Московская область, Город Москва", "Московская область и г. Москва"},
		{"Республика Крым, Город Севастополь", "Республика Крым и г. Севастополь"},
	}
	for _, tc := range cases {
		keys := LookupKeys(tc.in)
		if !containsFold(keys, tc.alias) {
			t.Fatalf("%q -> %v\nmissing alias %q", tc.in, keys, tc.alias)
		}
	}
}

func TestLookupKeysEmpty(t *testing.T) {
	if LookupKeys("  ") != nil {
		t.Fatal("empty")
	}
}
