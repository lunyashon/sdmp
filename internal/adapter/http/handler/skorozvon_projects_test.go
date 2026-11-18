package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	httpgin "github.com/lunyashon/sdmp/internal/adapter/http/gin"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type stubSkorozvonProjectAPI struct {
	items  []entity.SkorozvonProject
	total  int
	offset int
	name   string
	err    error
	syncs  int
	uuids  []string
}

func (s *stubSkorozvonProjectAPI) ListPage(_ context.Context, offset int, name string) ([]entity.SkorozvonProject, int, error) {
	s.offset = offset
	s.name = name
	return s.items, s.total, s.err
}
func (s *stubSkorozvonProjectAPI) Sync(context.Context) ([]entity.SkorozvonProject, error) {
	s.syncs++
	return s.items, s.err
}
func (s *stubSkorozvonProjectAPI) Refresh(_ context.Context, uuids []string) ([]entity.SkorozvonProject, error) {
	s.uuids = append([]string(nil), uuids...)
	return s.items, s.err
}
func (s *stubSkorozvonProjectAPI) SendTest(context.Context, string) (int, error) {
	return 0, s.err
}

func skorozvonProjectsRouter(api handler.SkorozvonProjectAPI) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:             handler.NewLeadsHandler(nil, nil),
		Sources:           handler.NewSourcesHandler(stubSourceAPI{}, nil),
		SkorozvonProjects: handler.NewSkorozvonProjectsHandler(api, nil),
	}, nil)
}

func TestListSkorozvonProjects(t *testing.T) {
	api := &stubSkorozvonProjectAPI{
		items: []entity.SkorozvonProject{{
			UUID: "u1", ProjectID: 50000167215, Name: "ВР КАБИНЕТ #1 (НОВЫЙ 2)",
		}},
		total: 12,
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/skorozvon/projects/list?offset=5&name=%D0%BA%D0%B0%D0%B1%D0%B8%D0%BD%D0%B5%D1%82", nil)
	skorozvonProjectsRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if api.offset != 5 || api.name != "кабинет" {
		t.Fatalf("query offset=%d name=%q", api.offset, api.name)
	}
	got := decodeData[struct {
		Count  int                       `json:"count"`
		Total  int                       `json:"total"`
		Limit  int                       `json:"limit"`
		Offset int                       `json:"offset"`
		Items  []entity.SkorozvonProject `json:"items"`
	}](t, w.Body.Bytes())
	if got.Count != 1 || got.Total != 12 || got.Limit != usecase.SkorozvonProjectListLimit || got.Offset != 5 {
		t.Fatalf("%+v", got)
	}
}

func TestListSkorozvonProjectsBadOffset(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/skorozvon/projects/list?offset=-1", nil)
	skorozvonProjectsRouter(&stubSkorozvonProjectAPI{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestSyncSkorozvonProjects(t *testing.T) {
	api := &stubSkorozvonProjectAPI{items: []entity.SkorozvonProject{{
		UUID: "u1", ProjectID: 50000167215, Name: "ВР КАБИНЕТ #1 (НОВЫЙ 2)",
	}}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/projects/sync", nil)
	skorozvonProjectsRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if api.syncs != 1 {
		t.Fatalf("syncs %d", api.syncs)
	}
}

func TestRefreshSkorozvonProjectsByUUID(t *testing.T) {
	api := &stubSkorozvonProjectAPI{items: []entity.SkorozvonProject{{UUID: "b", ProjectID: 2}}}
	body := bytes.NewReader([]byte(`{"uuids":["b"]}`))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/projects/refresh", body)
	req.Header.Set("Content-Type", "application/json")
	skorozvonProjectsRouter(api).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if len(api.uuids) != 1 || api.uuids[0] != "b" {
		t.Fatalf("uuids %+v", api.uuids)
	}
}

func TestCreateSkorozvonProjectRemoved(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/skorozvon/projects", nil)
	skorozvonProjectsRouter(&stubSkorozvonProjectAPI{}).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestOldProjectsListRemoved(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/skorozvon/projects", nil)
	skorozvonProjectsRouter(&stubSkorozvonProjectAPI{}).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
