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
	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type stubCrmcallsPostback struct {
	got entity.CrmcallsPostback
	n   int
}

func (s *stubCrmcallsPostback) ApplyCrmcallsPostback(_ context.Context, in entity.CrmcallsPostback) error {
	s.n++
	s.got = in
	return nil
}

const testCrmcallsPostbackKey = "postback-test-key"

func crmcallsRouter(svc handler.CrmcallsPostbackAPI) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:    handler.NewLeadsHandler(nil, nil),
		Sources:  handler.NewSourcesHandler(stubSourceAPI{}, nil),
		Crmcalls: handler.NewCrmcallsHandler(nil, nil, svc, nil).WithPostbackKey(testCrmcallsPostbackKey),
	}, nil)
}

func withCrmcallsKey(req *http.Request) *http.Request {
	req.Header.Set("X-API-KEY", testCrmcallsPostbackKey)
	return req
}

func TestReceiveCrmcallsPostbackJSON(t *testing.T) {
	svc := &stubCrmcallsPostback{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/crmcalls/events", strings.NewReader(`{"name":"Иван","phone":"79001234567","result":"Передан лид","id":"ctc-1","call":{"id":"call-9","record_url":"https://cc.crmcalls.ru/api/recordings/call-9"},"occurred_at":"2026-10-20T11:52:02+03:00"}`))
	req.Header.Set("Content-Type", "application/json")
	crmcallsRouter(svc).ServeHTTP(w, withCrmcallsKey(req))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.n != 1 || svc.got.Phone != "79001234567" || svc.got.Result != "Передан лид" || svc.got.ID != "ctc-1" || svc.got.Name != "Иван" {
		t.Fatalf("%+v", svc.got)
	}
	if svc.got.CallID != "call-9" || svc.got.CallURL != "https://cc.crmcalls.ru/api/recordings/call-9" || svc.got.CallLastAt == nil {
		t.Fatalf("recording %+v", svc.got)
	}
}

func TestReceiveCrmcallsPostbackForm(t *testing.T) {
	svc := &stubCrmcallsPostback{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/crmcalls/events", strings.NewReader(`name=Иван&phone=79001234567&result=Передан+лид&id=ctc-2`))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	crmcallsRouter(svc).ServeHTTP(w, withCrmcallsKey(req))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.n != 1 || svc.got.ID != "ctc-2" || svc.got.Phone != "79001234567" || svc.got.Result != "Передан лид" {
		t.Fatalf("%+v", svc.got)
	}
}

func TestReceiveCrmcallsPostbackEmptyOK(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/crmcalls/events", nil)
	crmcallsRouter(nil).ServeHTTP(w, withCrmcallsKey(req))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestReceiveCrmcallsPostbackQuery(t *testing.T) {
	svc := &stubCrmcallsPostback{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/crmcalls/events?id=ctc-q&phone=79001112233&result=ok&name=Test", nil)
	crmcallsRouter(svc).ServeHTTP(w, withCrmcallsKey(req))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if svc.n != 1 || svc.got.ID != "ctc-q" || svc.got.Phone != "79001112233" || svc.got.Result != "ok" || svc.got.Name != "Test" {
		t.Fatalf("%+v", svc.got)
	}
}

type stubCrmcallsProjectsAPI struct {
	items []entity.CrmcallsProject
	conn  entity.CrmcallsConnection
	key   string
	err   error
}

func (s *stubCrmcallsProjectsAPI) List(context.Context) ([]entity.CrmcallsProject, error) {
	return s.items, s.err
}
func (s *stubCrmcallsProjectsAPI) Sync(context.Context) ([]entity.CrmcallsProject, error) {
	return s.items, s.err
}
func (s *stubCrmcallsProjectsAPI) Connection(context.Context) (entity.CrmcallsConnection, error) {
	return s.conn, s.err
}
func (s *stubCrmcallsProjectsAPI) Connect(_ context.Context, apiKey, _ string) ([]entity.CrmcallsProject, error) {
	s.key = apiKey
	return s.items, s.err
}
func (s *stubCrmcallsProjectsAPI) Refresh(context.Context) ([]entity.CrmcallsProject, error) {
	return s.items, s.err
}
func (s *stubCrmcallsProjectsAPI) SendTest(_ context.Context, id string) (entity.CrmcallsTestResult, error) {
	s.key = id
	return entity.CrmcallsTestResult{ContactID: "ctc-1", HTTPStatus: 200}, s.err
}

func crmcallsProjectsRouter(api handler.CrmcallsProjectAPI) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:    handler.NewLeadsHandler(nil, nil),
		Sources:  handler.NewSourcesHandler(stubSourceAPI{}, nil),
		Crmcalls: handler.NewCrmcallsHandler(nil, api, nil, nil),
	}, nil)
}

func TestReceiveCrmcallsPostbackRequiresKey(t *testing.T) {
	svc := &stubCrmcallsPostback{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/crmcalls/events", strings.NewReader(`{"phone":"79001234567","result":"Передан лид"}`))
	crmcallsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized || svc.n != 0 {
		t.Fatalf("status %d n %d", w.Code, svc.n)
	}
}

func TestGetCrmcallsSettings(t *testing.T) {
	api := &stubCrmcallsProjectsAPI{conn: entity.CrmcallsConnection{
		HasKey: true, BaseURL: "https://cc.crmcalls.ru", APIKeyMasked: "crmc_live_••••test", ProjectCount: 2,
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/crmcalls/settings", nil)
	crmcallsProjectsRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"has_key":true`) || !strings.Contains(w.Body.String(), `crmc_live_`) {
		t.Fatalf("%s", w.Body.String())
	}
}

func TestSaveCrmcallsSettings(t *testing.T) {
	api := &stubCrmcallsProjectsAPI{items: []entity.CrmcallsProject{{ID: "p1", Name: "Обзвон"}}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/rest/crmcalls/settings", strings.NewReader(`{"api_key":"crmc_live_x","base_url":"https://cc.crmcalls.ru"}`))
	req.Header.Set("Content-Type", "application/json")
	crmcallsProjectsRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if api.key != "crmc_live_x" {
		t.Fatalf("key %s", api.key)
	}
	if !strings.Contains(w.Body.String(), `"p1"`) {
		t.Fatalf("%s", w.Body.String())
	}
}

func TestSendCrmcallsProjectTest(t *testing.T) {
	api := &stubCrmcallsProjectsAPI{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/crmcalls/projects/p1/test", nil)
	crmcallsProjectsRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if api.key != "p1" || !strings.Contains(w.Body.String(), `"ctc-1"`) {
		t.Fatalf("%s key=%s", w.Body.String(), api.key)
	}
}
