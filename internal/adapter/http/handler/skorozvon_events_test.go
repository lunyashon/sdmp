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
	gotID string
	got   entity.LeadCallResult
	err   error
	n     int
}

func (s *stubCallResultAPI) ApplyCallResult(_ context.Context, skorozvonLeadID string, in entity.LeadCallResult) error {
	s.n++
	s.gotID = skorozvonLeadID
	s.got = in
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
	if !strings.Contains(w.Body.String(), `"ok":true`) {
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
		"lead": {"id": 123456788},
		"call": {
			"id": 987654321,
			"ended_at": "30.01.2019 10:06:08",
			"recording_url": "https://api.skorozvon.ru/api/calls/987654321.mp3",
			"call_project_id": "12345678",
			"result_id": 111
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
	if svc.got.CallLastAt == nil || svc.got.CallLastAt.In(clock.Location).Format("02.01.2006 15:04:05") != "30.01.2019 10:06:08" {
		t.Fatalf("ended %+v", svc.got.CallLastAt)
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

func TestReceiveCallResultUnknownLeadOK(t *testing.T) {
	svc := &stubCallResultAPI{err: domain.ErrNotFound}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/events", strings.NewReader(`{"type":"call_result","lead":{"id":1}}`))
	skorozvonEventsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
