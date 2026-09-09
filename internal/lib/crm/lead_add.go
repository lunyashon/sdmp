package crm

import (
	"encoding/json"
	"strconv"
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

	fields := map[string]any{
		"PHONE": []map[string]string{{
			"VALUE":      strings.TrimSpace(lead.Phone),
			"VALUE_TYPE": "WORK",
		}},
		"COMMENTS":          comments,
		"UF_CRM_1788705605": strings.TrimSpace(lead.UUID),
	}
	applyMappedAnketaFields(fields, lead.AnketaFields)
	applyMappedAnswers(fields, lead.Anketa)
	setFieldIfEmpty(fields, "UF_CRM_1747216350206", strings.TrimSpace(lead.Region))
	setFieldIfEmpty(fields, "UF_CRM_1638446549", strings.TrimSpace(lead.City))
	setFieldIfEmpty(fields, "UF_CRM_1676626557", strings.TrimSpace(lead.Operator))
	setFieldIfEmpty(fields, "UF_CRM_1734523005552", strings.TrimSpace(lead.Operator))

	return json.Marshal(map[string]any{"fields": fields})
}

func applyMappedAnketaFields(fields map[string]any, rows []entity.SkorozvonAnketaFieldItem) {
	for _, row := range rows {
		code := strings.TrimSpace(row.BitrixCode)
		if code == "" {
			code = BitrixCodeForSkorozvonName(row.FieldName)
		}
		setMappedField(fields, code, anketaFieldValue(row))
	}
}

func applyMappedAnswers(fields map[string]any, anketa *entity.SkorozvonAnketa) {
	if anketa == nil {
		return
	}
	for _, a := range anketa.Answers {
		setMappedField(fields, BitrixCodeForSkorozvonName(a.QuestionName), strings.Join(a.AnswerValues, ", "))
	}
}

func anketaFieldValue(row entity.SkorozvonAnketaFieldItem) string {
	if name := strings.TrimSpace(row.EnumName); name != "" {
		return name
	}
	if row.EnumID != 0 {
		return strconv.FormatInt(row.EnumID, 10)
	}
	return ""
}

func setMappedField(fields map[string]any, code, val string) {
	code = strings.TrimSpace(code)
	val = strings.TrimSpace(val)
	if code == "" || val == "" {
		return
	}
	if path := splitBitrixPath(code); len(path) > 1 {
		setBitrixPath(fields, path, val)
		return
	}
	setFieldIfEmpty(fields, code, val)
}

func setFieldIfEmpty(fields map[string]any, code, val string) {
	val = strings.TrimSpace(val)
	if code == "" || val == "" {
		return
	}
	cur, ok := fields[code]
	if !ok || cur == nil || cur == "" {
		fields[code] = val
		return
	}
	s, ok := cur.(string)
	if !ok || s == "" {
		return
	}
	for _, part := range strings.Split(s, ", ") {
		if part == val {
			return
		}
	}
	fields[code] = s + ", " + val
}

func splitBitrixPath(code string) []string {
	if !strings.Contains(code, "[") {
		return []string{code}
	}
	out := make([]string, 0, 3)
	rest := code
	if i := strings.Index(rest, "["); i > 0 {
		out = append(out, rest[:i])
		rest = rest[i:]
	}
	for rest != "" {
		if !strings.HasPrefix(rest, "[") {
			return nil
		}
		end := strings.Index(rest, "]")
		if end <= 1 {
			return nil
		}
		out = append(out, rest[1:end])
		rest = rest[end+1:]
	}
	return out
}

func setBitrixPath(fields map[string]any, path []string, val string) {
	if len(path) != 3 || path[0] != "PHONE" || path[2] != "VALUE" {
		return
	}
	phones, _ := fields["PHONE"].([]map[string]string)
	if len(phones) == 0 {
		fields["PHONE"] = []map[string]string{{"VALUE": val, "VALUE_TYPE": "WORK"}}
		return
	}
	if strings.TrimSpace(phones[0]["VALUE"]) == "" {
		phones[0]["VALUE"] = val
		fields["PHONE"] = phones
	}
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
