package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	httpgin "github.com/lunyashon/sdmp/internal/adapter/http/gin"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type stubPartnerAPI struct {
	items []entity.Partner
	saved entity.Partner
	err   error
}

func (s *stubPartnerAPI) List(context.Context) ([]entity.Partner, error) { return s.items, s.err }
func (s *stubPartnerAPI) Create(_ context.Context, in usecase.PartnerInput) (entity.Partner, error) {
	if s.err != nil {
		return entity.Partner{}, s.err
	}
	s.saved = entity.Partner{ID: 3, Name: in.Name, WebhookURL: in.WebhookURL, IsActive: in.IsActive, ZaryaResults: in.ZaryaResults, RegionIDs: in.RegionIDs, StopSourceIDs: in.StopSourceIDs}
	return s.saved, nil
}
func (s *stubPartnerAPI) Update(_ context.Context, id int, in usecase.PartnerInput) (entity.Partner, error) {
	if s.err != nil {
		return entity.Partner{}, s.err
	}
	s.saved = entity.Partner{ID: id, Name: in.Name, WebhookURL: in.WebhookURL, IsActive: in.IsActive, ZaryaResults: in.ZaryaResults, RegionIDs: in.RegionIDs, StopSourceIDs: in.StopSourceIDs}
	return s.saved, nil
}

func (s *stubPartnerAPI) Refresh(context.Context) ([]entity.PartnerCheck, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []entity.PartnerCheck{{ID: 1, Name: "КР", OK: true, HTTPStatus: 200, Response: "ok"}}, nil
}

func (s *stubPartnerAPI) SendTest(_ context.Context, id int) (entity.PartnerCheck, error) {
	if s.err != nil {
		return entity.PartnerCheck{}, s.err
	}
	return entity.PartnerCheck{ID: id, Name: "КР", OK: true, HTTPStatus: 200, Response: "accepted"}, nil
}

func partnersRouter(api handler.PartnerAPI) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:    handler.NewLeadsHandler(nil, nil),
		Sources:  handler.NewSourcesHandler(stubSourceAPI{}, nil),
		Partners: handler.NewPartnersHandler(api, nil),
	})
}

func TestListPartners(t *testing.T) {
	api := &stubPartnerAPI{items: []entity.Partner{{ID: 1, Name: "РТ", ZaryaResults: []string{"dup"}}}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/partners", nil)
	partnersRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Count int              `json:"count"`
		Items []entity.Partner `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || got.Items[0].Name != "РТ" {
		t.Fatalf("%+v", got)
	}
}

func TestCreatePartner(t *testing.T) {
	api := &stubPartnerAPI{}
	body, _ := json.Marshal(map[string]any{
		"name": "КР", "webhook_url": "https://example.test", "is_active": true,
		"zarya_results": []string{"dup"}, "region_ids": []int{23}, "stop_source_ids": []int{},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/partners", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	partnersRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if api.saved.Name != "КР" || api.saved.RegionIDs[0] != 23 {
		t.Fatalf("%+v", api.saved)
	}
}

func TestUpdatePartner(t *testing.T) {
	api := &stubPartnerAPI{}
	body, _ := json.Marshal(map[string]any{
		"name": "ББ", "webhook_url": "https://bb.test", "is_active": false,
		"zarya_results": []string{"unique", "dup"},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/rest/partners/4", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	partnersRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if api.saved.ID != 4 || api.saved.IsActive || api.saved.Name != "ББ" {
		t.Fatalf("%+v", api.saved)
	}
}

func TestRefreshPartners(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/partners/refresh", nil)
	partnersRouter(&stubPartnerAPI{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got struct {
		Count int                   `json:"count"`
		Items []entity.PartnerCheck `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || !got.Items[0].OK || got.Items[0].HTTPStatus != 200 {
		t.Fatalf("%+v", got)
	}
}

func TestSendPartnerTest(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/partners/4/test", nil)
	partnersRouter(&stubPartnerAPI{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var got entity.PartnerCheck
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 4 || !got.OK || got.Response != "accepted" {
		t.Fatalf("%+v", got)
	}
}
