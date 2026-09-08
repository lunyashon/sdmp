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
