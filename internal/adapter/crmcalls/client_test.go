package crmcalls

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

func TestSendLeadImportsIntoProject(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	importCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/contacts/import":
			importCalls++
			gotPath = r.URL.Path
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			if importCalls == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"code":"import_rate_limit","details":{"retry_after":1}}}`))
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"data":{"import_id":"imp-1"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/contacts/import/imp-1":
			_, _ = w.Write([]byte(`{"data":{"import_id":"imp-1","state":"completed"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/contacts":
			if r.URL.Query().Get("external_id") != "uuid-1" {
				t.Fatalf("lookup %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"ctc-1"}]}`))
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	c := New(Config{BaseURL: srv.URL, APIKey: "crmc_live_test"})
	c.http = srv.Client()
	res, err := c.SendLead(context.Background(), port.CrmcallsLead{
		Phone:      "79001234567",
		ExternalID: "uuid-1",
		ProjectID:  "proj-1",
		Region:     "Москва",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ContactID != "ctc-1" || res.Status != 200 {
		t.Fatalf("%+v", res)
	}
	if gotAuth != "Bearer crmc_live_test" {
		t.Fatalf("auth %s", gotAuth)
	}
	if gotPath != "/api/v1/contacts/import" {
		t.Fatalf("path %s", gotPath)
	}
	if !strings.Contains(gotBody, `"79001234567"`) || !strings.Contains(gotBody, `"uuid-1"`) || !strings.Contains(gotBody, `"proj-1"`) || !strings.Contains(gotBody, `"Москва"`) {
		t.Fatalf("body %s", gotBody)
	}
	if !strings.Contains(gotBody, `"data"`) || !strings.Contains(gotBody, `"deduplication"`) {
		t.Fatalf("import envelope %s", gotBody)
	}
}

func TestSendLeadRequiresProject(t *testing.T) {
	c := New(Config{BaseURL: "http://example.invalid", APIKey: "k"})
	if _, err := c.SendLead(context.Background(), port.CrmcallsLead{Phone: "79001234567"}); err == nil {
		t.Fatal("expected project_id error")
	}
}

func TestDownloadRecordingByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Fatalf("auth %s", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/api/recordings/call-1" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("ID3"))
	}))
	t.Cleanup(srv.Close)
	c := New(Config{BaseURL: srv.URL, APIKey: "k"})
	c.http = srv.Client()
	body, ctype, err := c.DownloadRecording(context.Background(), "call-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ID3" || ctype != "audio/mpeg" {
		t.Fatalf("%q %q", body, ctype)
	}
}

func TestListProjectsUnwrapsData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"p1","name":"Обзвон","status":"running"}]}`))
	}))
	t.Cleanup(srv.Close)
	c := New(Config{BaseURL: srv.URL, APIKey: "k"})
	c.http = srv.Client()
	items, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "p1" || items[0].Name != "Обзвон" {
		t.Fatalf("%+v", items)
	}
}

func TestDynamicPrefersStoredKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(srv.Close)
	d := NewDynamic(Config{BaseURL: srv.URL, APIKey: "env-key"}, stubSettings{
		apiKey: "db-key", baseURL: srv.URL,
	})
	if _, err := d.ListProjects(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer db-key" {
		t.Fatalf("auth %s", gotAuth)
	}
}

type stubSettings struct {
	apiKey  string
	baseURL string
}

func (s stubSettings) Get(context.Context) (entity.CrmcallsSettings, error) {
	return entity.CrmcallsSettings{APIKey: s.apiKey, BaseURL: s.baseURL}, nil
}

func TestExtractContactID(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"data": map[string]any{"id": "abc"}})
	if got := extractContactID(raw); got != "abc" {
		t.Fatalf("%s", got)
	}
}
