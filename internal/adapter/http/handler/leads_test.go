package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	httpgin "github.com/lunyashon/sdmp/internal/adapter/http/gin"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type stubReceiver struct {
	in  usecase.ReceiveLeadInput
	id  int64
	err error
}

func (s *stubReceiver) Receive(_ context.Context, in usecase.ReceiveLeadInput) (int64, error) {
	s.in = in
	return s.id, s.err
}

type stubLeadFilter struct {
	q      usecase.LeadFilterQuery
	res    usecase.LeadFilterResult
	detail entity.LeadDetail
	err    error
	getErr error
}

func (s *stubLeadFilter) Filter(_ context.Context, q usecase.LeadFilterQuery) (usecase.LeadFilterResult, error) {
	s.q = q
	return s.res, s.err
}

func (s *stubLeadFilter) IncomingStats(context.Context, string, string) ([]entity.LeadSourceCount, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []entity.LeadSourceCount{{SourceID: 1, Count: 12}}, nil
}

func (s *stubLeadFilter) EnrichmentStats(context.Context, string, string) ([]entity.LeadEnrichmentStat, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []entity.LeadEnrichmentStat{{
		SourceID: 1, Total: 10,
		Region:   entity.LeadGeoShare{Supplier: 6, Provider: 3, Unknown: 1},
		Operator: entity.LeadGeoShare{Supplier: 5, Provider: 4, Unknown: 1},
	}}, nil
}

func (s *stubLeadFilter) ZaryaStats(context.Context, string, string) ([]entity.LeadZaryaStat, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []entity.LeadZaryaStat{{SourceID: 1, Total: 8, Unique: 5, Dup: 3}}, nil
}

func (s *stubLeadFilter) Get(_ context.Context, id int64) (entity.LeadDetail, error) {
	if s.getErr != nil {
		return entity.LeadDetail{}, s.getErr
	}
	if s.detail.ID == 0 {
		s.detail.ID = id
	}
	return s.detail, nil
}

func leadsRouter(svc *stubLeadFilter) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(svc, nil),
		Sources: handler.NewSourcesHandler(nil, nil),
		CRMLead: handler.NewCRMLeadHandler(&stubReceiver{id: 1}, nil),
	})
}

func TestListLeads(t *testing.T) {
	created := time.Date(2026, 8, 27, 16, 27, 56, 0, time.Local)
	svc := &stubLeadFilter{res: usecase.LeadFilterResult{
		Total:  15,
		Limit:  10,
		Offset: 0,
		Leads: []entity.LeadListItem{{
			DateCreate:  created,
			Source:      "GDMP",
			ChildSource: "GDMP / МТС",
			ColdLabel:   "ВР",
			Region:      "Москва",
			Operator:    "БИЛАЙН",
			Stage:       "обогащена",
			Phone:       "79009876539",
			Zarya:       "Чистый",
		}},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads?date_from=01.08.2026+00:00:00&date_to=30.08.2026+23:59:59&sources=1&sources=12&cold_label=ВР&region=1&region=5&operator=2&operator=3&zarya=Чистый&zarya=Дубль&phone=79009876539", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got dto.LeadsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 15 || got.Count != 1 || got.Leads[0].Source != "GDMP" || got.Leads[0].ChildSource != "GDMP / МТС" || got.Leads[0].ColdLabel != "ВР" {
		t.Fatalf("%+v", got)
	}
	if got.Leads[0].DateCreate != "27.08.2026 16:27:56" {
		t.Fatalf("date %q", got.Leads[0].DateCreate)
	}
	if len(svc.q.Sources) != 2 || svc.q.Sources[0] != 1 || svc.q.ColdLabels[0] != "ВР" || len(svc.q.Regions) != 2 || svc.q.Regions[1] != 5 {
		t.Fatalf("query %+v", svc.q)
	}
	if len(svc.q.Operators) != 2 || svc.q.Operators[1] != 3 || len(svc.q.Zarya) != 2 || svc.q.Zarya[0] != "Чистый" {
		t.Fatalf("query %+v", svc.q)
	}
}

func TestListLeadsRequiresDates(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestGetLead(t *testing.T) {
	created := time.Date(2026, 8, 27, 16, 27, 56, 0, time.Local)
	srcID := 12
	svc := &stubLeadFilter{detail: entity.LeadDetail{
		ID:         42,
		UUID:       "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Phone:      "79001234567",
		PhoneValid: true,
		SourceID:   &srcID,
		Source:     "ВонтРезалт",
		Stage:      "обогащена",
		StatusID:   2,
		CreatedAt:  created,
		CallFormRaw: `{"imya":"Иван","summa_dolga":"150000"}`,
		Events: []entity.LeadEvent{{
			TypeID:     1,
			TypeName:   "принята",
			OccurredAt: created,
			Source:     3,
		}},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/42", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got dto.LeadDetailResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Lead.ID != 42 || got.Lead.Phone != "79001234567" || got.Lead.Source != "ВонтРезалт" {
		t.Fatalf("%+v", got.Lead)
	}
	form, ok := got.Lead.CallForm.(map[string]any)
	if !ok || form["imya"] != "Иван" {
		t.Fatalf("call_form %+v", got.Lead.CallForm)
	}
	if len(got.Lead.Events) != 1 || got.Lead.Events[0].Type != "принята" {
		t.Fatalf("events %+v", got.Lead.Events)
	}
}

func TestGetLeadNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/9", nil)
	leadsRouter(&stubLeadFilter{getErr: domain.ErrNotFound}).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestIncomingStats(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/incoming-stats?date_from=2026-08-14&date_to=2026-08-20", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Count int                      `json:"count"`
		Items []entity.LeadSourceCount `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || got.Items[0].SourceID != 1 || got.Items[0].Count != 12 {
		t.Fatalf("%+v", got)
	}
}

func TestEnrichmentStats(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/enrichment-stats?date_from=2026-08-14&date_to=2026-08-20", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Count int                         `json:"count"`
		Items []entity.LeadEnrichmentStat `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || got.Items[0].Region.Supplier != 6 || got.Items[0].Operator.Provider != 4 {
		t.Fatalf("%+v", got)
	}
}

func TestZaryaStats(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/zarya-stats?date_from=2026-08-14&date_to=2026-08-20", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Count int                   `json:"count"`
		Items []entity.LeadZaryaStat `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || got.Items[0].Unique != 5 || got.Items[0].Dup != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestGetLeadBadID(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/abc", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
