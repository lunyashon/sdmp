package crm

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

// hardcoded status id for new leads
const bx24LeadStatusID = "14"

const (
	bx24WhichBaseUF = "UF_CRM_1698755876042" // Какая база — lead_row.base_label
	bx24ColdBaseUF  = "UF_CRM_1697524451888" // Холодная база — lead_row.cold_label

)

func LeadAddPayload(lead entity.LeadDetail) ([]byte, error) {
	return LeadAddPayloadMapped(lead, DefaultLeadFieldMap())
}

type LeadFieldMap struct {
	Phone             string
	City              string
	Region            string
	UUID              string
	WebID             string
	Comments          string
	Status            string
	StatusValue       string
	BaseLabel         string
	ColdLabel         string
	Operator          string
	Tag               string
	Name              string
	Title             string
	Zarya             string
	ZaryaOld          string
	SourceDescription string
	CallURL           string
}

func DefaultLeadFieldMap() LeadFieldMap {
	return LeadFieldMap{
		Phone:       "PHONE",
		City:        "UF_CRM_1638446549",
		UUID:        "UF_CRM_1788705605",
		Comments:    "COMMENTS",
		Status:      "STATUS_ID",
		StatusValue: bx24LeadStatusID,
		BaseLabel:   bx24WhichBaseUF,
		ColdLabel:   bx24ColdBaseUF,
	}
}

func LeadFieldMapFromRows(rows []entity.BitrixLeadMap) LeadFieldMap {
	out := LeadFieldMap{}
	for _, row := range rows {
		code := strings.TrimSpace(row.BitrixCode)
		val := strings.TrimSpace(row.FixedValue)
		switch strings.TrimSpace(row.SourceKey) {
		case "phone":
			out.Phone = code
		case "city":
			out.City = code
		case "region":
			out.Region = code
		case "uuid":
			out.UUID = code
		case "web_id":
			out.WebID = code
		case "comments":
			out.Comments = code
		case "status_id":
			out.Status = code
			out.StatusValue = val
		case "base_label":
			out.BaseLabel = code
		case "cold_label":
			out.ColdLabel = code
		case "operator":
			out.Operator = code
		case "tag":
			out.Tag = code
		case "name":
			out.Name = code
		case "title":
			out.Title = code
		case "zarya":
			out.Zarya = code
		case "zarya_old":
			out.ZaryaOld = code
		case "source_description":
			out.SourceDescription = code
		case "call_url":
			out.CallURL = code
		}
	}
	return out
}

func LeadAddPayloadMapped(lead entity.LeadDetail, m LeadFieldMap) ([]byte, error) {
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

	fields := map[string]any{}
	setMappedPhone(fields, m.Phone, strings.TrimSpace(lead.Phone))
	if code := strings.TrimSpace(m.Comments); code != "" {
		fields[code] = comments
	}
	if strings.TrimSpace(m.Status) != "" && strings.TrimSpace(m.StatusValue) != "" {
		fields[strings.TrimSpace(m.Status)] = strings.TrimSpace(m.StatusValue)
	}
	setMappedField(fields, m.UUID, strings.TrimSpace(lead.UUID))
	setMappedField(fields, m.WebID, strings.TrimSpace(lead.WebID))
	applyMappedAnketaFields(fields, lead.AnketaFields)
	applyMappedAnswers(fields, lead.Anketa)
	setLeadBaseLabels(fields, lead, m)
	setFieldIfEmpty(fields, m.City, strings.TrimSpace(lead.City))
	setFieldIfEmpty(fields, m.Region, strings.TrimSpace(lead.Region))
	setFieldIfEmpty(fields, m.Operator, strings.TrimSpace(lead.Operator))
	setFieldIfEmpty(fields, m.Tag, strings.TrimSpace(lead.Tag))
	setFieldIfEmpty(fields, m.Name, form["imya"])
	setFieldIfEmpty(fields, m.Title, form["imya"])
	setFieldIfEmpty(fields, m.Zarya, strings.TrimSpace(lead.Zarya))
	setFieldIfEmpty(fields, m.ZaryaOld, strings.TrimSpace(lead.ZaryaOld))
	setFieldIfEmpty(fields, m.SourceDescription, strings.TrimSpace(lead.SourceDescription))
	setFieldIfEmpty(fields, m.CallURL, ResolveCallURL(lead.CallURL, lead.CallID))

	return json.Marshal(map[string]any{"fields": fields})
}

// ResolveCallURL — явный call_url, иначе ссылка СК из numeric call_id (CRMCalls без URL не выдумываем).
func ResolveCallURL(callURL, callID string) string {
	if u := strings.TrimSpace(callURL); u != "" {
		return u
	}
	id := strings.TrimSpace(callID)
	if id == "" || !skorozvonNumericCallID(id) {
		return ""
	}
	return "https://app.skorozvon.ru/shards/5/api/calls/" + id + ".mp3" // shards/5 — фактический паттерн СК
}

func skorozvonNumericCallID(s string) bool {
	if s == "" || len(s) > 20 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func setMappedPhone(fields map[string]any, code, phone string) {
	code = strings.TrimSpace(code)
	phone = strings.TrimSpace(phone)
	if code == "" || phone == "" {
		return
	}
	if code == "PHONE" || strings.HasPrefix(code, "PHONE[") {
		fields["PHONE"] = []map[string]string{{
			"VALUE":      phone,
			"VALUE_TYPE": "WORK",
		}}
		return
	}
	fields[code] = phone
}

func setLeadBaseLabels(fields map[string]any, lead entity.LeadDetail, m LeadFieldMap) {
	if v := strings.TrimSpace(lead.BaseLabel); v != "" && strings.TrimSpace(m.BaseLabel) != "" {
		fields[strings.TrimSpace(m.BaseLabel)] = v
	}
	if v := strings.TrimSpace(lead.ColdLabel); v != "" && strings.TrimSpace(m.ColdLabel) != "" {
		fields[strings.TrimSpace(m.ColdLabel)] = v
	}
}

func protectedBitrixCode(code string) bool {
	switch strings.TrimSpace(code) {
	case bx24WhichBaseUF, bx24ColdBaseUF:
		return true
	default:
		return false
	}
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
	if code == "" || val == "" || protectedBitrixCode(code) {
		return
	}
	if path := splitBitrixPath(code); len(path) > 1 {
		setBitrixPath(fields, path, val)
		return
	}
	setFieldIfEmpty(fields, code, val)
}

func singleMappedBitrix(code string) bool {
	switch strings.TrimSpace(code) {
	case "UF_CRM_1747216350206", "UF_CRM_1676626557", "UF_CRM_1734523005552":
		return true
	default:
		return false
	}
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
	if singleMappedBitrix(code) {
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
