package operator

import "strings"

const (
	Beeline = "БИЛАЙН"
	MTS     = "МТС"
	Megafon = "МЕГАФОН"
	Tele2   = "ТЕЛЕ-2"
	Other   = "ДРУГИЕ"
)

// companyToGroup — временный маппинг юр. названий из определителя номера.
var companyToGroup = map[string]string{
	`ООО "Т2 МОБАЙЛ"`:             Tele2,
	`АО "МТТ"`:                    MTS,
	`ПАО МГТС`:                    MTS,
	`ПАО "МОБИЛЬНЫЕ ТЕЛЕСИСТЕМЫ"`: MTS,
	`ПАО "МТС"`:                   MTS,
	`ООО "СКАРТЕЛ"`:               Megafon,
	`ПАО "МЕГАФОН"`:               Megafon,
	`ПАО "ВЫМПЕЛКОМ"`:             Beeline,
	`ПАО "ВЫМПЕЛ-КОММУНИКАЦИИ"`:   Beeline,
}

var shortToGroup = map[string]string{
	"БИЛАЙН": Beeline, "BEELINE": Beeline, "ВЫМПЕЛКОМ": Beeline,
	"МТС": MTS, "MTS": MTS, "МГТС": MTS, "МТТ": MTS,
	"МЕГАФОН": Megafon, "MEGAFON": Megafon, "ЙОТА": Megafon, "YOTA": Megafon, "СКАРТЕЛ": Megafon,
	"ТЕЛЕ-2": Tele2, "ТЕЛЕ2": Tele2, "TELE2": Tele2, "T2": Tele2, "Т2": Tele2, "Т2 МОБАЙЛ": Tele2,
	"ДРУГИЕ": Other, "ДРУГОЕ": Other, "OTHER": Other,
}

func Resolve(raw string) string {
	key := normalize(raw)
	if key == "" {
		return Other
	}
	if g, ok := companyToGroup[key]; ok {
		return g
	}
	if g, ok := shortToGroup[key]; ok {
		return g
	}
	return Other
}

func normalize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	repl := strings.NewReplacer(
		"«", `"`, "»", `"`, "“", `"`, "”", `"`, "'", `"`,
		"ё", "е", "Ё", "Е",
	)
	return strings.ToUpper(repl.Replace(s))
}
