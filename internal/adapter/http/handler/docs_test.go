package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	httpgin "github.com/lunyashon/sdmp/internal/adapter/http/gin"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
)

func TestDownloadCRMLeadAddDocx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(nil, nil),
		Sources: handler.NewSourcesHandler(stubSourceAPI{}, nil),
		Docs:    handler.NewDocsHandler(nil),
	}, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/docs/crm.lead.add.docx", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "crm-lead-add.docx") {
		t.Fatalf("disposition %q", got)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "wordprocessingml.document") {
		t.Fatalf("content-type %q", ct)
	}
	if w.Body.Len() < 4 || string(w.Body.Bytes()[:2]) != "PK" {
		t.Fatalf("not a docx zip, len=%d", w.Body.Len())
	}
}
