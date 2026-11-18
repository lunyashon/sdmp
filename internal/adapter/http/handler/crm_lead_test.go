package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"

	httpgin "github.com/lunyashon/sdmp/internal/adapter/http/gin"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/lib/config"
)

func crmRouter(recv *stubReceiver) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(nil, nil),
		Sources: handler.NewSourcesHandler(nil, nil),
		CRMLead: handler.NewCRMLeadHandler(recv, nil),
	}, nil)
}

func TestCRMLeadAddJSON(t *testing.T) {
	recv := &stubReceiver{id: 42}
	body := `{"fields":{"TITLE":"Лид","NAME":"Иван","PHONE":[{"VALUE":"79001234567","VALUE_TYPE":"WORK"}],"SOURCE_DESCRIPTION":"Яндекс.Директ"}}`
	req := httptest.NewRequest(http.MethodPost, "/rest/1/hook-token/crm.lead.add.json", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	crmRouter(recv).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got response.BitrixOK
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Result != 42 {
		t.Fatalf("result=%d", got.Result)
	}
	if got.Time.Start == 0 || got.Time.DateStart == "" || got.Time.DateFinish == "" {
		t.Fatalf("bitrix time %+v", got.Time)
	}
	if recv.in.Phone != "79001234567" || recv.in.Token != "hook-token" || recv.in.UserID != "1" || recv.in.SourceDescription != "Яндекс.Директ" {
		t.Fatalf("input %+v", recv.in)
	}
}

func TestCRMContactAddDoesNotCreateLead(t *testing.T) {
	recv := &stubReceiver{id: 9}
	body := `{"fields":{"PHONE":[{"VALUE":"79001112233"}]}}`
	req := httptest.NewRequest(http.MethodPost, "/rest/1/hook-token/crm.contact.add.json", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	crmRouter(recv).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if recv.in.Phone != "" {
		t.Fatalf("contact.add must not call Receive: %+v", recv.in)
	}
	var got response.BitrixOK
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Result == 0 {
		t.Fatalf("expected dummy contact id, body %s", w.Body.String())
	}
}

func TestCRMLeadAddFormURLEncoded(t *testing.T) {
	recv := &stubReceiver{id: 7}
	form := url.Values{}
	form.Set("fields[TITLE]", "Лид")
	form.Set("fields[PHONE][0][VALUE]", "79001112233")
	form.Set("fields[PHONE][0][VALUE_TYPE]", "WORK")
	form.Set("fields[SOURCE_DESCRIPTION]", "VK Ads")
	req := httptest.NewRequest(http.MethodPost, "/rest/1/hook-token/crm.lead.add", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	crmRouter(recv).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if recv.in.Phone != "79001112233" || recv.in.SourceDescription != "VK Ads" {
		t.Fatalf("input %+v", recv.in)
	}
}

func TestCRMLeadAddRateLimitedPerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recv := &stubReceiver{id: 1}
	r := httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(nil, nil),
		Sources: handler.NewSourcesHandler(nil, nil),
		CRMLead: handler.NewCRMLeadHandler(recv, nil),
	}, &config.Config{CRMLeadAddRPS: 1, CRMLeadAddBurst: 1})

	body := `{"fields":{"PHONE":[{"VALUE":"79001234567"}]}}`
	post := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/rest/1/"+token+"/crm.lead.add.json", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	if w := post("hook-a"); w.Code != http.StatusOK {
		t.Fatalf("first a: %d %s", w.Code, w.Body.String())
	}
	if w := post("hook-b"); w.Code != http.StatusOK {
		t.Fatalf("first b: %d %s", w.Code, w.Body.String())
	}
	w := post("hook-a")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second a: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	var got struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error != "QUERY_LIMIT_EXCEEDED" {
		t.Fatalf("error %+v", got)
	}
}

func TestCRMLeadAddUnauthorized(t *testing.T) {
	recv := &stubReceiver{err: domain.ErrUnauthorized}
	body := `{"fields":{"PHONE":[{"VALUE":"79001234567"}]}}`
	req := httptest.NewRequest(http.MethodPost, "/rest/1/bad/crm.lead.add.json", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	crmRouter(recv).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error != "INVALID_CREDENTIALS" {
		t.Fatalf("error %+v", got)
	}
}

func TestCRMLeadScope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rest/1/4524vby182ytwhs9/scope", nil)
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Result []string            `json:"result"`
		Time   response.BitrixTime `json:"time"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"crm": true, "user": true, "task": true, "tasks_extended": true}
	if len(got.Result) != len(want) {
		t.Fatalf("result %#v", got.Result)
	}
	for _, s := range got.Result {
		if !want[s] {
			t.Fatalf("unexpected scope %q in %#v", s, got.Result)
		}
		delete(want, s)
	}
	if len(want) != 0 {
		t.Fatalf("missing scopes %#v", want)
	}
	if got.Time.Start == 0 || got.Time.DateStart == "" {
		t.Fatalf("time %+v", got.Time)
	}
}

func TestCRMLeadScopeJSONSuffix(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rest/1/hook/scope.json", nil)
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestCRMLeadScopeFull(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rest/1/hook/scope?full=true", nil)
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Result []string `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Result) < 2 {
		t.Fatalf("full scopes %#v", got.Result)
	}
	found := false
	for _, s := range got.Result {
		if s == "crm" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing crm in %#v", got.Result)
	}
}

func TestCRMLeadScopeUnauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rest/2/hook/scope", nil)
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestCRMLeadMimicProfile(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rest/1/hook/profile", nil)
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Result map[string]any      `json:"result"`
		Time   response.BitrixTime `json:"time"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Result["ID"] != "1" || got.Result["ADMIN"] != true {
		t.Fatalf("profile %#v", got.Result)
	}
}

func TestCRMLeadMimicUserGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rest/1/hook/user.get.json?filter[ID]=2", nil)
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Result []map[string]any `json:"result"`
		Total  int              `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || len(got.Result) != 1 || got.Result[0]["ID"] != "2" {
		t.Fatalf("users %#v total=%d", got.Result, got.Total)
	}
}

func TestCRMLeadMimicStatusListSource(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rest/1/hook/crm.status.list?filter[ENTITY_ID]=SOURCE", nil)
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Result []map[string]any `json:"result"`
		Total  int              `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total == 0 || len(got.Result) == 0 {
		t.Fatalf("empty sources %#v", got.Result)
	}
	found := false
	for _, item := range got.Result {
		if item["ENTITY_ID"] != "SOURCE" {
			t.Fatalf("unexpected entity %#v", item)
		}
		if item["STATUS_ID"] == "WEB" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing WEB in %#v", got.Result)
	}
}

func TestCRMLeadMimicTaskAdd(t *testing.T) {
	body := `{"fields":{"TITLE":"Идентификация","RESPONSIBLE_ID":"2"}}`
	req := httptest.NewRequest(http.MethodPost, "/rest/1/hook/tasks.task.add", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Result struct {
			Task map[string]any `json:"task"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Result.Task["id"] == nil || got.Result.Task["id"] == "" || got.Result.Task["title"] != "Идентификация" {
		t.Fatalf("task %#v", got.Result.Task)
	}
	if got.Result.Task["responsibleId"] != "2" {
		t.Fatalf("responsible %#v", got.Result.Task)
	}
}

func TestCRMLeadMimicLeadFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rest/1/hook/crm.lead.fields", nil)
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Result map[string]map[string]any `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"TITLE", "PHONE", "SOURCE_ID", "UF_CRM_1752577998838", "UF_CRM_1734523005552"} {
		if got.Result[code] == nil {
			t.Fatalf("missing %s in %#v", code, got.Result)
		}
	}
	if got.Result["PHONE"]["title"] != "Телефон" || got.Result["PHONE"]["type"] != "string" {
		t.Fatalf("phone %#v", got.Result["PHONE"])
	}
}

func TestCRMLeadMimicUserfieldList(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rest/1/hook/crm.userfield.list", nil)
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Result []map[string]any `json:"result"`
		Total  int              `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total == 0 || len(got.Result) == 0 {
		t.Fatalf("empty userfields %#v", got.Result)
	}
	found := false
	for _, item := range got.Result {
		if item["FIELD_NAME"] == "UF_CRM_1752577998838" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing web id %#v", got.Result)
	}
}

func TestCRMLeadMimicUnknownMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/rest/1/hook/crm.quote.add", nil)
	w := httptest.NewRecorder()
	crmRouter(&stubReceiver{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error != "ERROR_METHOD_NOT_FOUND" {
		t.Fatalf("error %+v", got)
	}
}
