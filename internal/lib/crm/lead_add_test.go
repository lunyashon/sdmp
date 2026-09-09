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
	if got.Fields["UF_CRM_1747216350206"] != "Москва" {
		t.Fatalf("region %v", got.Fields["UF_CRM_1747216350206"])
	}
	if got.Fields["UF_CRM_1638446549"] != "Химки" {
		t.Fatalf("city %v", got.Fields["UF_CRM_1638446549"])
	}
	if got.Fields["UF_CRM_1676626557"] != "БИЛАЙН" || got.Fields["UF_CRM_1734523005552"] != "БИЛАЙН" {
		t.Fatalf("operator %v %v", got.Fields["UF_CRM_1676626557"], got.Fields["UF_CRM_1734523005552"])
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
