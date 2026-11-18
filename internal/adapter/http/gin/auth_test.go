package gin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gongin "github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/lib/auth"
	"github.com/lunyashon/sdmp/internal/lib/config"
)

func TestPublicREST(t *testing.T) {
	ok := []string{
		"/rest/auth/login",
		"/rest/skorozvon/events",
		"/rest/crmcalls/events",
		"/rest/recordings/abc.mp3",
		"/rest/1/tok/crm.lead.add.json",
		"/rest/1/tok/scope",
	}
	for _, p := range ok {
		if !publicREST(p) {
			t.Fatalf("public %s", p)
		}
	}
	closed := []string{"/rest/leads", "/rest/sources", "/rest/crmcalls/links", "/rest/skorozvon/links"}
	for _, p := range closed {
		if publicREST(p) {
			t.Fatalf("must require auth %s", p)
		}
	}
}

func TestRequireAuth(t *testing.T) {
	gongin.SetMode(gongin.TestMode)
	cfg := &config.Config{AuthLogin: "admin", AuthPassword: "pw"}
	r := gongin.New()
	r.Use(requireAuth(cfg))
	r.GET("/rest/leads", func(c *gongin.Context) { c.Status(http.StatusOK) })
	r.POST("/rest/skorozvon/events", func(c *gongin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/rest/leads", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no token %d", w.Code)
	}

	tok, err := auth.Issue(cfg.AuthPassword, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/rest/leads", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("with token %d %s", w.Code, w.Body)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/rest/skorozvon/events", strings.NewReader("{}")))
	if w.Code != http.StatusOK {
		t.Fatalf("events %d", w.Code)
	}
}
