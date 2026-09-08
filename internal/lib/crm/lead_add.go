package crm

import (
	"encoding/json"
	"strings"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

func LeadAddPayload(lead entity.LeadDetail) ([]byte, error) {
	form := formAnswers(lead.CallFormRaw, lead.Anketa, lead.AnketaFields)
	comments := strings.Join([]string{
		"Имя клиента - " + form["imya"],
		"Регион - " + strings.TrimSpace(lead.Region),
		"Город - " + strings.TrimSpace(lead.City),
		"Сумма долга - " + form["summa_dolga"],
		"Вид долга - " + form["vid_dolga"],
		"Имущество - " + form["imuschestvo"],
		"Доход - " + form["dohod"],
		"",
	}, "\n")

	return json.Marshal(map[string]any{
		"fields": map[string]any{
			"PHONE": []map[string]string{{
				"VALUE":      strings.TrimSpace(lead.Phone),
				"VALUE_TYPE": "WORK",
			}},
			"COMMENTS":             comments,
			"UF_CRM_1788705605":    strings.TrimSpace(lead.UUID),
			"UF_CRM_1747216350206": strings.TrimSpace(lead.Region),
			"UF_CRM_1638446549":    strings.TrimSpace(lead.City),
			"UF_CRM_1676626557":    strings.TrimSpace(lead.Operator),
			"UF_CRM_1734523005552": strings.TrimSpace(lead.Operator),
		},
	})
}

func formAnswers(callFormRaw string, anketa *entity.SkorozvonAnketa, fields []entity.SkorozvonAnketaFieldItem) map[string]string {
	out := map[string]string{}
	for _, f := range fields {
		appendFormValue(out, formKey(f.FieldName), f.EnumName)
	}
	if anketa != nil {
		for _, a := range anketa.Answers {
			key := formKey(a.QuestionName)
			if out[key] != "" {
				continue
			}
			appendFormValue(out, key, strings.Join(a.AnswerValues, ", "))
		}
	}
	if raw := strings.TrimSpace(callFormRaw); raw != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			for k, v := range m {
				key := formKey(k)
				if key == "" || out[key] != "" {
					continue
				}
				appendFormValue(out, key, scalar(v))
			}
		}
	}
	return out
}

func appendFormValue(out map[string]string, key, val string) {
	val = strings.TrimSpace(val)
	if key == "" || val == "" {
		return
	}
	if out[key] == "" {
		out[key] = val
		return
	}
	for _, part := range strings.Split(out[key], ", ") {
		if part == val {
			return
		}
	}
	out[key] += ", " + val
}

func formKey(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "imya", "имя", "имя клиента", "фио":
		return "imya"
	case "summa_dolga", "сумма долга":
		return "summa_dolga"
	case "vid_dolga", "вид долга":
		return "vid_dolga"
	case "imuschestvo", "имущество":
		return "imuschestvo"
	case "dohod", "доход":
		return "dohod"
	default:
		return ""
	}
}
