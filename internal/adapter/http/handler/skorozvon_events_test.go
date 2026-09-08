package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	httpgin "github.com/lunyashon/sdmp/internal/adapter/http/gin"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
)

type stubCallResultAPI struct {
	gotID   string
	got     entity.LeadCallResult
	gotForm entity.LeadFormResponse
	gotSnap entity.LeadFromSkorozvon
	err     error
	n       int
}

func (s *stubCallResultAPI) ApplyCallResult(_ context.Context, skorozvonLeadID string, in entity.LeadCallResult, snap entity.LeadFromSkorozvon) error {
	s.n++
	s.gotID = skorozvonLeadID
	s.got = in
	s.gotSnap = snap
	return s.err
}

func (s *stubCallResultAPI) ApplyFormResponse(_ context.Context, skorozvonLeadID string, in entity.LeadFormResponse, snap entity.LeadFromSkorozvon) error {
	s.n++
	s.gotID = skorozvonLeadID
	s.gotForm = in
	s.gotSnap = snap
	return s.err
}

func skorozvonEventsRouter(svc handler.SkorozvonEventAPI) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:           handler.NewLeadsHandler(nil, nil),
		Sources:         handler.NewSourcesHandler(stubSourceAPI{}, nil),
		SkorozvonEvents: handler.NewSkorozvonEventsHandler(svc, nil),
	})
}

func TestReceiveSkorozvonEventPOST(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/events", strings.NewReader(`{"type":"form_response"}`))
	req.Header.Set("Content-Type", "application/json")
	skorozvonEventsRouter(nil).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"success":true`) || !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Fatalf("body %s", w.Body.String())
	}
}

func TestReceiveSkorozvonEventGET(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/skorozvon/events?probe=1", nil)
	skorozvonEventsRouter(nil).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestReceiveSkorozvonEventPATCH(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/rest/skorozvon/events", strings.NewReader(`[]`))
	skorozvonEventsRouter(nil).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestReceiveCallResultAppliesFields(t *testing.T) {
	svc := &stubCallResultAPI{}
	body := `{
		"type": "call_result",
		"lead": {"id": 123456788, "name": "Иван", "city": "Москва", "phones": "+79001234567", "tags": ["тест"], "custom_fields": {"FIELD_50000009406": [50000015671], "FIELD_50000011777": [50000004560]}},
		"contact": {"id": 123456789, "phones": "+79000000001"},
		"call": {
			"id": 987654321,
			"ended_at": "30.01.2019 10:06:08",
			"recording_url": "https://api.skorozvon.ru/api/calls/987654321.mp3",
			"call_project_id": "12345678",
			"result_id": 111,
			"region": "Москва и Московская область"
		},
		"call_result": {"result_id": 54321, "result_name": "Результат звонка"}
	}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	skorozvonEventsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.n != 1 || svc.gotID != "123456788" {
		t.Fatalf("id %q n=%d", svc.gotID, svc.n)
	}
	if svc.got.CallID != "987654321" || svc.got.CallURL == "" || svc.got.ProjectNum != "12345678" {
		t.Fatalf("%+v", svc.got)
	}
	if svc.got.CallResult == nil || *svc.got.CallResult != 54321 {
		t.Fatalf("call_result %+v", svc.got.CallResult)
	}
	if svc.got.CallResultName != "Результат звонка" {
		t.Fatalf("result_name %q", svc.got.CallResultName)
	}
	if svc.got.CallLastAt == nil || svc.got.CallLastAt.In(clock.Location).Format("02.01.2006 15:04:05") != "30.01.2019 10:06:08" {
		t.Fatalf("ended %+v", svc.got.CallLastAt)
	}
	if svc.gotSnap.Phone != "+79001234567" || svc.gotSnap.City != "Москва" || svc.gotSnap.Tag != "тест" {
		t.Fatalf("snap %+v", svc.gotSnap)
	}
	if svc.gotSnap.Region != "Москва и Московская область" {
		t.Fatalf("region %q", svc.gotSnap.Region)
	}
	gotFields := map[int64]string{}
	for _, f := range svc.gotSnap.CustomFields {
		gotFields[f.FieldID] = string(f.Raw)
	}
	if gotFields[50000009406] != "[50000015671]" || gotFields[50000011777] != "[50000004560]" {
		t.Fatalf("custom_fields %+v", svc.gotSnap.CustomFields)
	}
	if !strings.Contains(svc.gotSnap.LegacyRaw, `"name":"Иван"`) {
		t.Fatalf("legacy %s", svc.gotSnap.LegacyRaw)
	}
}

func TestReceiveCallResultMissingLeadID(t *testing.T) {
	svc := &stubCallResultAPI{err: domain.ErrInvalidInput}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/events", strings.NewReader(`{"type":"call_result"}`))
	skorozvonEventsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestReceiveFormResponseAppliesAnswers(t *testing.T) {
	svc := &stubCallResultAPI{}
	body := `{
		"type": "form_response",
		"lead": {"id": 123456788},
		"call": {"id": 987654321},
		"form_response": {
			"user_id": 12345,
			"form_id": 1,
			"scenario_id": 12345,
			"form_name": "Анкета",
			"updated_at": "30.01.2019 10:06:59",
			"answers": [
				{"question_id": 1, "question_name": "Город", "answer_id": 1, "answer_values": ["Екатеринбург"]},
				{"question_id": 2, "question_name": "Адрес", "answer_id": 2, "answer_values": ["г. Екатеринбург, ул. Татищева 49А"]}
			]
		}
	}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	skorozvonEventsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.n != 1 || svc.gotID != "123456788" {
		t.Fatalf("id %q n=%d", svc.gotID, svc.n)
	}
	if svc.gotForm.CallID != "987654321" || svc.gotForm.FormName != "Анкета" {
		t.Fatalf("%+v", svc.gotForm)
	}
	if svc.gotForm.FormID == nil || *svc.gotForm.FormID != 1 || len(svc.gotForm.Answers) != 2 {
		t.Fatalf("%+v", svc.gotForm)
	}
	if svc.gotForm.Answers[0].QuestionName != "Город" || svc.gotForm.Answers[0].AnswerValues[0] != "Екатеринбург" {
		t.Fatalf("%+v", svc.gotForm.Answers)
	}
}

func TestReceiveCallResultLeadPhonesArray(t *testing.T) {
	svc := &stubCallResultAPI{}
	body := `{
		"type": "call_result",
		"lead": {"id": 7, "phones": ["+79001112233", "+79004445566"], "city": "Казань"},
		"call": {"id": 1, "phone": "+70000000000"}
	}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/events", strings.NewReader(body))
	skorozvonEventsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.gotSnap.Phone != "+79001112233" || svc.gotSnap.City != "Казань" {
		t.Fatalf("snap %+v", svc.gotSnap)
	}
}

func TestReceiveCallResultLeadPhoneObject(t *testing.T) {
	svc := &stubCallResultAPI{}
	body := `{
		"type": "call_result",
		"lead": {"id": 8, "phones": [{"phone": "+79007778899"}]}
	}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/events", strings.NewReader(body))
	skorozvonEventsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.gotSnap.Phone != "+79007778899" {
		t.Fatalf("snap %+v", svc.gotSnap)
	}
}

func TestReceiveCallResultUnknownLeadOK(t *testing.T) {
	svc := &stubCallResultAPI{err: domain.ErrNotFound}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/events", strings.NewReader(`{"type":"call_result","lead":{"id":1}}`))
	skorozvonEventsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestReceiveCallResultParsesNameAndLargeResultID(t *testing.T) {
	svc := &stubCallResultAPI{}
	body := `{
		"type": "call_result",
		"lead": {
			"id": 51038587780,
			"city": null,
			"phones": "+79277363936",
			"lead_phones": ["+7 (927) 736-39-36"],
			"tags": ["ВР Другие"],
			"custom_fields": {"FIELD_50000011777": [50000004578]}
		},
		"call": {
			"id": 51590794835,
			"phone": "+79277363936",
			"ended_at": "2026-09-08T13:20:32.000+04:00",
			"result_id": 50000228189,
			"region": "Самарская область"
		},
		"call_result": {"result_id": 50000228189, "result_name": "Передан лид"}
	}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/events", strings.NewReader(body))
	skorozvonEventsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.gotID != "51038587780" {
		t.Fatalf("id %q", svc.gotID)
	}
	if svc.got.CallResultName != "Передан лид" {
		t.Fatalf("result_name %q", svc.got.CallResultName)
	}
	if svc.got.CallResult == nil || *svc.got.CallResult != 50000228189 {
		t.Fatalf("call_result %+v", svc.got.CallResult)
	}
	if svc.gotSnap.Phone != "+79277363936" || svc.gotSnap.Region != "Самарская область" {
		t.Fatalf("snap %+v", svc.gotSnap)
	}
	if svc.got.CallLastAt == nil {
		t.Fatal("ended_at")
	}
}
