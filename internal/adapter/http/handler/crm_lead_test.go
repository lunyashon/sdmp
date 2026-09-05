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
)

func crmRouter(recv *stubReceiver) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(nil, nil),
		Sources: handler.NewSourcesHandler(nil, nil),
		CRMLead: handler.NewCRMLeadHandler(recv, nil),
	})
}

func TestCRMLeadAddJSON(t *testing.T) {
	recv := &stubReceiver{id: 42}
	body := `{"fields":{"TITLE":"Лид","NAME":"Иван","PHONE":[{"VALUE":"79001234567","VALUE_TYPE":"WORK"}]}}`
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
	if recv.in.Phone != "79001234567" || recv.in.Token != "hook-token" || recv.in.UserID != "1" {
		t.Fatalf("input %+v", recv.in)
	}
}

func TestCRMLeadAddFormURLEncoded(t *testing.T) {
	recv := &stubReceiver{id: 7}
	form := url.Values{}
	form.Set("fields[TITLE]", "Лид")
	form.Set("fields[PHONE][0][VALUE]", "79001112233")
	form.Set("fields[PHONE][0][VALUE_TYPE]", "WORK")
	req := httptest.NewRequest(http.MethodPost, "/rest/1/hook-token/crm.lead.add", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	crmRouter(recv).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if recv.in.Phone != "79001112233" {
		t.Fatalf("phone %q", recv.in.Phone)
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
