package crm

import (
	"encoding/json"
	"strings"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

func LeadAddPayload(lead entity.LeadDetail) ([]byte, error) {
	form := formAnswers(lead.CallFormRaw, lead.Anketa)
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
			"COMMENTS":            comments,
			"UF_CRM_1788705605":   strings.TrimSpace(lead.UUID),
			"UF_CRM_1747216350206": strings.TrimSpace(lead.Region),
			"UF_CRM_1638446549":   strings.TrimSpace(lead.City),
			"UF_CRM_1676626557":   strings.TrimSpace(lead.Operator),
			"UF_CRM_1734523005552": strings.TrimSpace(lead.Operator),
		},
	})
}

func formAnswers(callFormRaw string, anketa *entity.SkorozvonAnketa) map[string]string {
	out := map[string]string{}
	if raw := strings.TrimSpace(callFormRaw); raw != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			for k, v := range m {
				if key := formKey(k); key != "" {
					out[key] = scalar(v)
				}
			}
		}
	}
	if anketa == nil {
		return out
	}
	for _, a := range anketa.Answers {
		key := formKey(a.QuestionName)
		if key == "" || out[key] != "" {
			continue
		}
		out[key] = strings.TrimSpace(strings.Join(a.AnswerValues, ", "))
	}
	return out
}

func formKey(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "imya", "имя", "имя клиента":
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
