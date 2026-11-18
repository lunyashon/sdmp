package regionname

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	reDash   = regexp.MustCompile(`\s*[—–-]\s*`)
	reSpaces = regexp.MustCompile(`\s+`)
)

// LookupKeys — варианты одного названия региона для поиска в region / region_alias.
// phone.search с ~окт.2026 кладёт субъект в territory: «Город Москва, Московская область»,
// «Кемеровская область-Кузбасс» и т.п. Старые алиасы — «г. Москва и …», «… - Кузбасс».
func LookupKeys(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 16)
	add := func(s string) {
		s = strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}

	seeds := []string{
		raw,
		swapCityWord(raw),
		commaToAnd(raw),
		commaToAnd(swapCityWord(raw)),
	}
	for _, seed := range seeds {
		add(seed)
		add(unifyDash(seed, "-"))
		add(unifyDash(seed, " - "))
		add(stripLeadingCityAbbrev(seed))
		add(unifyDash(stripLeadingCityAbbrev(seed), "-"))
		add(unifyDash(stripLeadingCityAbbrev(seed), " - "))
	}
	return out
}

func unifyDash(s, repl string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case 'ё':
			return 'е'
		case 'Ё':
			return 'Е'
		default:
			return r
		}
	}, s)
	return strings.TrimSpace(reDash.ReplaceAllString(s, repl))
}

func swapCityWord(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		rest := s[i:]
		lower := strings.ToLower(rest)
		if strings.HasPrefix(lower, "город") {
			next := len("город")
			if next < len(rest) {
				r := []rune(rest[next:])[0]
				if unicode.IsLetter(r) {
					b.WriteByte(s[i])
					i++
					continue
				}
			}
			b.WriteString("г.")
			i += len("город")
			if i < len(s) && s[i] == ' ' {
				b.WriteByte(' ')
				i++
			} else if i < len(s) && unicode.IsLetter(rune(s[i])) {
				b.WriteByte(' ')
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return strings.TrimSpace(b.String())
}

func commaToAnd(s string) string {
	return strings.ReplaceAll(s, ", ", " и ")
}

func stripLeadingCityAbbrev(s string) string {
	t := strings.TrimSpace(s)
	lower := strings.ToLower(t)
	for _, p := range []string{"г. ", "г.", "гор. ", "гор."} {
		if strings.HasPrefix(lower, p) {
			return strings.TrimSpace(t[len(p):])
		}
	}
	return t
}
