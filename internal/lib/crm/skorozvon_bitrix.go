package crm

import "strings"

var skorozvonNameToBitrix = map[string]string{
	"название":          "TITLE",
	"регион клиента":    "UF_CRM_1747216350206",
	"телефон":           "PHONE[0][VALUE]",
	"телефоны":          "PHONE[0][VALUE]",
	"номер проекта":     "UF_CRM_1699340770806",
	"сумма долга":       "UF_CRM_1730630573577",
	"оператор":          "UF_CRM_1676626557",
	"город клиента":     "UF_CRM_1638446549",
	"вид долга":         "UF_CRM_1730630661182",
	"другая информация": "UF_CRM_1730631041590",
	"имущество":         "UF_CRM_1730630726419",
	"дубликат":          "UF_CRM_1730362141931",
	"одм":               "UF_CRM_1737099727557",
	"доход":             "UF_CRM_1742819602566",
	"теги":              "UF_CRM_1729062300505",
	"gdmp_z":            "UF_CRM_1747907679528",
	"приставы":          "UF_CRM_1758104136848",
}

func BitrixCodeForSkorozvonName(name string) string {
	return skorozvonNameToBitrix[strings.ToLower(strings.TrimSpace(name))]
}
