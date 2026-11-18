package crm

import (
	"encoding/json"
	"testing"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

func TestLeadAddPayloadFromCallForm(t *testing.T) {
	raw, err := LeadAddPayload(entity.LeadDetail{
		UUID:        "lead-aaa",
		Phone:       "79001234567",
		Region:      "Москва",
		City:        "Химки",
		Operator:    "БИЛАЙН",
		BaseLabel:   "ВР",
		ColdLabel:   "ВР Билайн",
		CallFormRaw: `{"imya":"Иван","summa_dolga":"150000","vid_dolga":"кредит","imuschestvo":"нет","dohod":"50"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Fields["UF_CRM_1788705605"] != "lead-aaa" {
		t.Fatalf("uuid %v", got.Fields["UF_CRM_1788705605"])
	}
	if got.Fields["STATUS_ID"] != "14" {
		t.Fatalf("status %v", got.Fields["STATUS_ID"])
	}
	if got.Fields["UF_CRM_1698755876042"] != "ВР" {
		t.Fatalf("base %v", got.Fields["UF_CRM_1698755876042"])
	}
	if got.Fields["UF_CRM_1697524451888"] != "ВР Билайн" {
		t.Fatalf("cold %v", got.Fields["UF_CRM_1697524451888"])
	}
	if _, ok := got.Fields["UF_CRM_1747216350206"]; ok {
		t.Fatalf("region from lead_row %v", got.Fields["UF_CRM_1747216350206"])
	}
	if got.Fields["UF_CRM_1638446549"] != "Химки" {
		t.Fatalf("city %v", got.Fields["UF_CRM_1638446549"])
	}
	if _, ok := got.Fields["UF_CRM_1676626557"]; ok {
		t.Fatalf("operator from lead_row %v", got.Fields["UF_CRM_1676626557"])
	}
	if _, ok := got.Fields["UF_CRM_1734523005552"]; ok {
		t.Fatalf("operator telecom from lead_row %v", got.Fields["UF_CRM_1734523005552"])
	}
	want := "Имя клиента - Иван\nРегион - Москва\nГород - Химки\nСумма долга - 150000\nВид долга - кредит\nИмущество - нет\nДоход - 50\n"
	if got.Fields["COMMENTS"] != want {
		t.Fatalf("comments %q", got.Fields["COMMENTS"])
	}
	phones, _ := got.Fields["PHONE"].([]any)
	phone, _ := phones[0].(map[string]any)
	if phone["VALUE"] != "79001234567" || phone["VALUE_TYPE"] != "WORK" {
		t.Fatalf("phone %+v", phone)
	}
}

func TestLeadAddPayloadFromAnketa(t *testing.T) {
	raw, err := LeadAddPayload(entity.LeadDetail{
		UUID:   "u1",
		Phone:  "79001112233",
		Region: "Татарстан",
		City:   "Казань",
		Anketa: &entity.SkorozvonAnketa{
			Answers: []entity.SkorozvonAnketaAnswer{
				{QuestionName: "Имя клиента", AnswerValues: []string{"Пётр"}},
				{QuestionName: "Сумма долга", AnswerValues: []string{"200000"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Fields["COMMENTS"] != "Имя клиента - Пётр\nРегион - Татарстан\nГород - Казань\nСумма долга - 200000\nВид долга - \nИмущество - \nДоход - \n" {
		t.Fatalf("comments %q", got.Fields["COMMENTS"])
	}
}

func TestLeadAddPayloadFromAnketaFields(t *testing.T) {
	raw, err := LeadAddPayload(entity.LeadDetail{
		UUID:   "u1",
		Phone:  "79001112233",
		Region: "Краснодарский край",
		City:   "Краснодар",
		AnketaFields: []entity.SkorozvonAnketaFieldItem{
			{FieldID: 50000011778, FieldName: "Вид долга", EnumID: 1, EnumName: "Потребительские кредиты"},
			{FieldID: 50000011778, FieldName: "Вид долга", EnumID: 2, EnumName: "Автокредит (нет автомобиля)"},
			{FieldID: 50000009440, FieldName: "Сумма долга", EnumID: 3, EnumName: "Более 500 тыс."},
			{FieldID: 50000011779, FieldName: "Имущество", EnumID: 4, EnumName: "Нет"},
			{FieldID: 50000016217, FieldName: "Доход", EnumID: 5, EnumName: "Официальный"},
			{FieldID: 50000007333, FieldName: "ФИО", EnumID: 6, EnumName: "Иван"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := "Имя клиента - Иван\nРегион - Краснодарский край\nГород - Краснодар\nСумма долга - Более 500 тыс.\nВид долга - Потребительские кредиты, Автокредит (нет автомобиля)\nИмущество - Нет\nДоход - Официальный\n"
	if got.Fields["COMMENTS"] != want {
		t.Fatalf("comments %q", got.Fields["COMMENTS"])
	}
}

func TestLeadAddPayloadMapsBitrixCodes(t *testing.T) {
	raw, err := LeadAddPayload(entity.LeadDetail{
		UUID:     "2d6d7946-2ed1-455d-89f4-0f697ce3c655",
		Phone:    "79064830723",
		Region:   "Самарская область",
		City:     "Самара",
		Operator: "",
		AnketaFields: []entity.SkorozvonAnketaFieldItem{
			{FieldName: "Сумма долга", BitrixCode: "UF_CRM_1730630573577", EnumName: "300-400 тыс."},
			{FieldName: "Вид долга", BitrixCode: "UF_CRM_1730630661182", EnumName: "Микрозаймы"},
			{FieldName: "Вид долга", BitrixCode: "UF_CRM_1730630661182", EnumName: "ЖКХ"},
			{FieldName: "Имущество", BitrixCode: "UF_CRM_1730630726419", EnumName: "Есть, но готов расстаться"},
			{FieldName: "Доход", BitrixCode: "UF_CRM_1742819602566", EnumName: "Более 65 тыс."},
			{FieldName: "Регион клиента", BitrixCode: "UF_CRM_1747216350206", EnumName: "Самарская область"},
			{FieldName: "Оператор", BitrixCode: "UF_CRM_1676626557", EnumName: "Жарина Анастасия Юрьевна"},
			{FieldName: "Номер проекта", BitrixCode: "UF_CRM_1699340770806", EnumID: 32323},
			{FieldName: "GDMP_Z", BitrixCode: "UF_CRM_1747907679528", EnumID: 23423},
			{FieldName: "ID лида Б24", EnumID: 18013592},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Fields["UF_CRM_1730630573577"] != "300-400 тыс." {
		t.Fatalf("debt %v", got.Fields["UF_CRM_1730630573577"])
	}
	if got.Fields["UF_CRM_1730630661182"] != "Микрозаймы, ЖКХ" {
		t.Fatalf("kind %v", got.Fields["UF_CRM_1730630661182"])
	}
	if got.Fields["UF_CRM_1730630726419"] != "Есть, но готов расстаться" {
		t.Fatalf("property %v", got.Fields["UF_CRM_1730630726419"])
	}
	if got.Fields["UF_CRM_1742819602566"] != "Более 65 тыс." {
		t.Fatalf("income %v", got.Fields["UF_CRM_1742819602566"])
	}
	if got.Fields["UF_CRM_1676626557"] != "Жарина Анастасия Юрьевна" {
		t.Fatalf("op %v", got.Fields["UF_CRM_1676626557"])
	}
	if got.Fields["UF_CRM_1699340770806"] != "32323" {
		t.Fatalf("project %v", got.Fields["UF_CRM_1699340770806"])
	}
	if got.Fields["UF_CRM_1747907679528"] != "23423" {
		t.Fatalf("gdmp %v", got.Fields["UF_CRM_1747907679528"])
	}
	if _, ok := got.Fields["ID лида Б24"]; ok {
		t.Fatal("unmapped field leaked")
	}
	if got.Fields["UF_CRM_1747216350206"] != "Самарская область" || got.Fields["UF_CRM_1638446549"] != "Самара" {
		t.Fatalf("geo %v %v", got.Fields["UF_CRM_1747216350206"], got.Fields["UF_CRM_1638446549"])
	}
}

func TestLeadAddPayloadKeepsSplitBaseLabels(t *testing.T) {
	raw, err := LeadAddPayload(entity.LeadDetail{
		UUID:      "u1",
		Phone:     "79001112233",
		BaseLabel: "ВР",
		ColdLabel: "ВР МТС",
		AnketaFields: []entity.SkorozvonAnketaFieldItem{
			{FieldName: "База", BitrixCode: "UF_CRM_1698755876042", EnumName: "из Скорозвона"},
			{FieldName: "Холодная база", BitrixCode: "UF_CRM_1697524451888", EnumName: "тоже СК"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Fields["UF_CRM_1698755876042"] != "ВР" {
		t.Fatalf("base %v", got.Fields["UF_CRM_1698755876042"])
	}
	if got.Fields["UF_CRM_1697524451888"] != "ВР МТС" {
		t.Fatalf("cold %v", got.Fields["UF_CRM_1697524451888"])
	}
}

func TestLeadAddPayloadKeepsSingleMappedRegionAndOperator(t *testing.T) {
	raw, err := LeadAddPayload(entity.LeadDetail{
		UUID:     "u1",
		Phone:    "79001112233",
		Region:   "Москва и Московская область",
		City:     "Химки",
		Operator: "МТС",
		AnketaFields: []entity.SkorozvonAnketaFieldItem{
			{FieldName: "Регион клиента", BitrixCode: "UF_CRM_1747216350206", EnumName: "Амурская область"},
			{FieldName: "Регион клиента", BitrixCode: "UF_CRM_1747216350206", EnumName: "Самарская область"},
			{FieldName: "Оператор", BitrixCode: "UF_CRM_1676626557", EnumName: "Жарина Анастасия Юрьевна"},
			{FieldName: "Оператор", BitrixCode: "UF_CRM_1676626557", EnumName: "ещё один"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Fields["UF_CRM_1747216350206"] != "Амурская область" {
		t.Fatalf("region %v", got.Fields["UF_CRM_1747216350206"])
	}
	if got.Fields["UF_CRM_1676626557"] != "Жарина Анастасия Юрьевна" {
		t.Fatalf("operator %v", got.Fields["UF_CRM_1676626557"])
	}
	if _, ok := got.Fields["UF_CRM_1734523005552"]; ok {
		t.Fatalf("telecom operator %v", got.Fields["UF_CRM_1734523005552"])
	}
}

func TestLeadAddPayloadUsesSavedMap(t *testing.T) {
	raw, err := LeadAddPayloadMapped(entity.LeadDetail{
		UUID:              "u-map",
		Phone:             "79001112233",
		Region:            "Самара",
		City:              "Тольятти",
		Operator:          "МТС",
		Zarya:             "Чистый",
		SourceDescription: "Яндекс.Директ",
		CallURL:           "https://app.skorozvon.ru/shards/5/api/calls/1.mp3",
	}, LeadFieldMap{
		Phone:             "PHONE",
		City:              "UF_CRM_CITY",
		Region:            "UF_CRM_REGION",
		UUID:              "UF_CRM_UUID",
		Status:            "STATUS_ID",
		StatusValue:       "UC_P9ZKO2",
		Operator:          "UF_CRM_OP",
		Zarya:             "UF_CRM_ZARYA",
		SourceDescription: "SOURCE_DESCRIPTION",
		CallURL:           "UF_CRM_CALL_URL",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Fields["STATUS_ID"] != "UC_P9ZKO2" {
		t.Fatalf("status %v", got.Fields["STATUS_ID"])
	}
	if got.Fields["UF_CRM_CITY"] != "Тольятти" || got.Fields["UF_CRM_REGION"] != "Самара" {
		t.Fatalf("geo %v", got.Fields)
	}
	if got.Fields["UF_CRM_UUID"] != "u-map" || got.Fields["UF_CRM_OP"] != "МТС" {
		t.Fatalf("ids %v", got.Fields)
	}
	if got.Fields["UF_CRM_ZARYA"] != "Чистый" {
		t.Fatalf("zarya %v", got.Fields["UF_CRM_ZARYA"])
	}
	if got.Fields["SOURCE_DESCRIPTION"] != "Яндекс.Директ" {
		t.Fatalf("source_description %v", got.Fields["SOURCE_DESCRIPTION"])
	}
	if got.Fields["UF_CRM_CALL_URL"] != "https://app.skorozvon.ru/shards/5/api/calls/1.mp3" {
		t.Fatalf("call_url %v", got.Fields["UF_CRM_CALL_URL"])
	}
	if _, ok := got.Fields["COMMENTS"]; ok {
		t.Fatalf("comments should be unmapped")
	}
}

func TestLeadFieldMapFromRowsCallURL(t *testing.T) {
	got := LeadFieldMapFromRows([]entity.BitrixLeadMap{
		{SourceKey: "call_url", BitrixCode: "UF_CRM_CALL_URL"},
	})
	if got.CallURL != "UF_CRM_CALL_URL" {
		t.Fatalf("got %+v", got)
	}
}

func TestLeadAddPayloadResolvesCallURLFromCallID(t *testing.T) {
	raw, err := LeadAddPayloadMapped(entity.LeadDetail{
		Phone:  "79001112233",
		CallID: "51637832641",
	}, LeadFieldMap{
		Phone:   "PHONE",
		CallURL: "UF_CRM_1747171451142",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := "https://app.skorozvon.ru/shards/5/api/calls/51637832641.mp3"
	if got.Fields["UF_CRM_1747171451142"] != want {
		t.Fatalf("call_url %v", got.Fields["UF_CRM_1747171451142"])
	}
}

func TestLeadAddPayloadKeepsExplicitCallURL(t *testing.T) {
	raw, err := LeadAddPayloadMapped(entity.LeadDetail{
		Phone:   "79001112233",
		CallID:  "51637832641",
		CallURL: "https://app.skorozvon.ru/shards/5/api/calls/999.mp3",
	}, LeadFieldMap{
		Phone:   "PHONE",
		CallURL: "UF_CRM_1747171451142",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Fields["UF_CRM_1747171451142"] != "https://app.skorozvon.ru/shards/5/api/calls/999.mp3" {
		t.Fatalf("call_url %v", got.Fields["UF_CRM_1747171451142"])
	}
}

func TestLeadAddPayloadSkipsNonNumericCallID(t *testing.T) {
	raw, err := LeadAddPayloadMapped(entity.LeadDetail{
		Phone:  "79001112233",
		CallID: "call-abc",
	}, LeadFieldMap{
		Phone:   "PHONE",
		CallURL: "UF_CRM_1747171451142",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Fields["UF_CRM_1747171451142"]; ok {
		t.Fatalf("crmcalls without url must not invent link: %v", got.Fields["UF_CRM_1747171451142"])
	}
}

func TestResolveCallURL(t *testing.T) {
	if got := ResolveCallURL(" https://x.mp3 ", "1"); got != "https://x.mp3" {
		t.Fatalf("explicit %q", got)
	}
	if got := ResolveCallURL("", "51637832641"); got != "https://app.skorozvon.ru/shards/5/api/calls/51637832641.mp3" {
		t.Fatalf("from id %q", got)
	}
	if got := ResolveCallURL("", "call-9"); got != "" {
		t.Fatalf("non-numeric %q", got)
	}
}

func TestLeadFieldMapFromRowsSourceDescription(t *testing.T) {
	got := LeadFieldMapFromRows([]entity.BitrixLeadMap{
		{SourceKey: "source_description", BitrixCode: "SOURCE_DESCRIPTION"},
	})
	if got.SourceDescription != "SOURCE_DESCRIPTION" {
		t.Fatalf("got %+v", got)
	}
}

func TestLeadFieldMapFromRowsZarya(t *testing.T) {
	got := LeadFieldMapFromRows([]entity.BitrixLeadMap{
		{SourceKey: "zarya", BitrixCode: "UF_CRM_1730362141931"},
		{SourceKey: "zarya_old", BitrixCode: "UF_CRM_OLD_DUP"},
	})
	if got.Zarya != "UF_CRM_1730362141931" || got.ZaryaOld != "UF_CRM_OLD_DUP" {
		t.Fatalf("got %+v", got)
	}
}

func TestLeadAddPayloadMappedWebID(t *testing.T) {
	raw, err := LeadAddPayloadMapped(entity.LeadDetail{
		Phone: "79001234567",
		WebID: "web-42",
	}, LeadFieldMap{
		Phone: "PHONE",
		WebID: "UF_CRM_1752577998838",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Fields["UF_CRM_1752577998838"] != "web-42" {
		t.Fatalf("fields %+v", got.Fields)
	}
}
