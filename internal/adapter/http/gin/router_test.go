package gin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	httpgin "github.com/lunyashon/sdmp/internal/adapter/http/gin"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
	"github.com/lunyashon/sdmp/internal/lib/config"
)

func TestSwaggerRedirectsToIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(nil, nil),
		Sources: handler.NewSourcesHandler(nil, nil),
	}, &config.Config{SwaggerVersion: "v1", SwaggerToken: "secret"})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/swagger/v1/secret/", nil))
	if w.Code != http.StatusFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/swagger/v1/secret/index.html" {
		t.Fatalf("location %q", loc)
	}
}

func TestSwaggerCRMIndexOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(nil, nil),
		Sources: handler.NewSourcesHandler(nil, nil),
	}, &config.Config{SwaggerVersion: "v1", SwaggerToken: "secret"})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/swagger/crm/v1/secret/index.html", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestSwaggerIndexOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(nil, nil),
		Sources: handler.NewSourcesHandler(nil, nil),
	}, &config.Config{SwaggerVersion: "v1", SwaggerToken: "secret"})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/swagger/v1/secret/index.html", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
