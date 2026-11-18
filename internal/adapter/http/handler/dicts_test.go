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
)

type stubDictAPI struct {
	groups   []entity.OperatorGroup
	statuses []entity.NamedRef
	added    []string
	err      error
}

func (s *stubDictAPI) ListRegions(context.Context) ([]entity.NamedRef, error) { return nil, nil }
func (s *stubDictAPI) ListStatuses(context.Context) ([]entity.NamedRef, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.statuses, nil
}
func (s *stubDictAPI) ListOperatorGroups(context.Context) ([]entity.OperatorGroup, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.groups, nil
}
func (s *stubDictAPI) AddOperatorAlias(_ context.Context, _ int, alias string) error {
	if s.err != nil {
		return s.err
	}
	s.added = append(s.added, alias)
	return nil
}
func (s *stubDictAPI) RemoveOperatorAlias(context.Context, int, string) error { return s.err }

func dictsRouter(api handler.DictAPI) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(nil, nil),
		Sources: handler.NewSourcesHandler(stubSourceAPI{}, nil),
		Dicts:   handler.NewDictsHandler(api, nil),
	}, nil)
}

func TestOperatorsReturnsGroupsWithAliases(t *testing.T) {
	api := &stubDictAPI{groups: []entity.OperatorGroup{
		{ID: 1, Name: "БИЛАЙН", Aliases: []string{"Билайн", "Вымпелком"}},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/operators", nil)
	dictsRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[struct {
		Count int                    `json:"count"`
		Items []entity.OperatorGroup `json:"items"`
	}](t, w.Body.Bytes())
	if got.Count != 1 || got.Items[0].Name != "БИЛАЙН" || len(got.Items[0].Aliases) != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestAddOperatorAlias(t *testing.T) {
	api := &stubDictAPI{}
	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"alias": "Билайн_1245"})
	req := httptest.NewRequest(http.MethodPost, "/rest/operators/1/aliases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	dictsRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if len(api.added) != 1 || api.added[0] != "Билайн_1245" {
		t.Fatalf("added %v", api.added)
	}
}

func TestStatusesReturnsIDAndName(t *testing.T) {
	api := &stubDictAPI{statuses: []entity.NamedRef{
		{ID: 1, Name: "принята"},
		{ID: 4, Name: "из звонилки"},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/statuses", nil)
	dictsRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[struct {
		Count int               `json:"count"`
		Items []entity.NamedRef `json:"items"`
	}](t, w.Body.Bytes())
	if got.Count != 2 || got.Items[0].ID != 1 || got.Items[0].Name != "принята" || got.Items[1].Name != "из звонилки" {
		t.Fatalf("%+v", got)
	}
}

func TestAddOperatorAliasRequiresValue(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/operators/1/aliases", bytes.NewReader([]byte(`{"alias":"  "}`)))
	req.Header.Set("Content-Type", "application/json")
	dictsRouter(&stubDictAPI{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
}
