package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	httpgin "github.com/lunyashon/sdmp/internal/adapter/http/gin"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type stubSourceAPI struct {
	token   string
	items   []entity.Source
	created entity.Source
	err     error
}

func (s stubSourceAPI) List(context.Context) ([]entity.Source, error) { return nil, nil }
func (s stubSourceAPI) CreateToken(_ context.Context, sourceID int) (entity.Source, string, error) {
	if s.err != nil {
		return entity.Source{}, "", s.err
	}
	return entity.Source{ID: sourceID}, s.token, nil
}
func (s stubSourceAPI) ListByToken(context.Context, string) ([]entity.Source, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}
func (s stubSourceAPI) DeleteToken(context.Context, int) error { return s.err }
func (s stubSourceAPI) Delete(context.Context, int) error      { return s.err }
func (s stubSourceAPI) SetActive(_ context.Context, sourceID int, active bool) (entity.Source, error) {
	if s.err != nil {
		return entity.Source{}, s.err
	}
	return entity.Source{ID: sourceID, Name: "TECT", IsActive: active}, nil
}
func (s stubSourceAPI) Create(_ context.Context, in usecase.CreateSourceInput) (entity.Source, error) {
	if s.err != nil {
		return entity.Source{}, s.err
	}
	if s.created.ID != 0 {
		return s.created, nil
	}
	return entity.Source{ID: 400, Name: in.Name, BitrixCode: in.BitrixCode, IsActive: true}, nil
}
func (s stubSourceAPI) CreateChild(_ context.Context, in usecase.CreateChildInput) (entity.Source, error) {
	if s.err != nil {
		return entity.Source{}, s.err
	}
	if s.created.ID != 0 {
		return s.created, nil
	}
	parent := in.ParentID
	return entity.Source{ID: 999, ParentID: &parent, Name: in.Name, IsActive: true}, nil
}

func sourcesRouter(api stubSourceAPI) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(nil, nil),
		Sources: handler.NewSourcesHandler(api, nil),
		CRMLead: handler.NewCRMLeadHandler(&stubReceiver{id: 1}, nil),
	}, nil)
}

func TestCreateSourceToken(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/sources/12/token", nil)
	sourcesRouter(stubSourceAPI{token: "abcdefghijklmnop"}).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[dto.CreateTokenResponse](t, w.Body.Bytes())
	if got.SourceID != 12 || got.Token != "abcdefghijklmnop" {
		t.Fatalf("%+v", got)
	}
}

func TestListSourcesByToken(t *testing.T) {
	parent := 12
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/sources/by-token/abcdefghijklmnop", nil)
	sourcesRouter(stubSourceAPI{items: []entity.Source{
		{ID: 12, Name: "ВонтРезалт", BitrixCode: "UC_P8EDM3"},
		{ID: 230, ParentID: &parent, Name: "ВР Билайн", BitrixCode: "UC_MD8A2D"},
	}}).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[dto.SourcesResponse](t, w.Body.Bytes())
	if got.Count != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestListSourcesByTokenNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/sources/by-token/missingtokenxxxx", nil)
	sourcesRouter(stubSourceAPI{err: domain.ErrNotFound}).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestCreateChildSource(t *testing.T) {
	parent := 12
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/sources/12/children", bytes.NewBufferString(`{"name":"ВР Теле2"}`))
	req.Header.Set("Content-Type", "application/json")
	sourcesRouter(stubSourceAPI{created: entity.Source{
		ID: 320, ParentID: &parent, Name: "ВР Теле2", IsActive: true,
	}}).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[dto.SourceDTO](t, w.Body.Bytes())
	if got.ID != 320 || got.Name != "ВР Теле2" || got.ParentID == nil || *got.ParentID != 12 {
		t.Fatalf("%+v", got)
	}
}

func TestCreateParentSource(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/sources", bytes.NewBufferString(`{"name":"Новый источник","bitrix_code":"UC_TEST"}`))
	req.Header.Set("Content-Type", "application/json")
	sourcesRouter(stubSourceAPI{created: entity.Source{
		ID: 400, Name: "Новый источник", BitrixCode: "UC_TEST", IsActive: true,
	}}).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[dto.SourceDTO](t, w.Body.Bytes())
	if got.ID != 400 || got.Name != "Новый источник" || got.ParentID != nil || got.BitrixCode != "UC_TEST" || !got.IsActive {
		t.Fatalf("%+v", got)
	}
}

func TestCreateParentSourceBadJSON(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/sources", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	sourcesRouter(stubSourceAPI{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestDeleteSourceToken(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/rest/sources/12/token", nil)
	sourcesRouter(stubSourceAPI{}).ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestCreateChildSourceBadID(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/sources/0/children", bytes.NewBufferString(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	sourcesRouter(stubSourceAPI{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
