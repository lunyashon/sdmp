package crm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchLeadFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sdmp.lead.search" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("idempotency_key") != "lead-uuid" {
			t.Fatalf("key %q", r.URL.Query().Get("idempotency_key"))
		}
		_, _ = w.Write([]byte(`{"result":51037210991}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL+"/", srv.Client()).SearchLead(context.Background(), "lead-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 51037210991 {
		t.Fatalf("id %d", got.ID)
	}
}

func TestSearchLeadZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":0}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).SearchLead(context.Background(), "missing")
	if err != nil || got.ID != 0 {
		t.Fatalf("id=%d err=%v", got.ID, err)
	}
}

func TestSearchLeadRetries500ThenOK(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n < 3 {
			http.Error(w, "busy", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"result":42}`))
	}))
	defer srv.Close()

	cli := New(srv.URL, srv.Client())
	cli.retryWaits = []time.Duration{0, 0, 0}
	got, err := cli.SearchLead(context.Background(), "lead-uuid")
	if err != nil || got.ID != 42 || n != 3 {
		t.Fatalf("id=%d n=%d err=%v", got.ID, n, err)
	}
}

func TestSearchLeadRetriesTimeoutThenOK(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			time.Sleep(40 * time.Millisecond)
			return
		}
		_, _ = w.Write([]byte(`{"result":9}`))
	}))
	defer srv.Close()

	cli := New(srv.URL, &http.Client{Timeout: 15 * time.Millisecond})
	cli.retryWaits = []time.Duration{0, 0, 0}
	got, err := cli.SearchLead(context.Background(), "lead-uuid")
	if err != nil || got.ID != 9 || n != 2 {
		t.Fatalf("id=%d n=%d err=%v", got.ID, n, err)
	}
}

func TestSearchLeadNoRetryOn400(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer srv.Close()

	cli := New(srv.URL, srv.Client())
	cli.retryWaits = []time.Duration{0, 0, 0}
	_, err := cli.SearchLead(context.Background(), "lead-uuid")
	if err == nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestSearchLeadGivesUpAfter3Retries(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		http.Error(w, "busy", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cli := New(srv.URL, srv.Client())
	cli.retryWaits = []time.Duration{0, 0, 0}
	_, err := cli.SearchLead(context.Background(), "lead-uuid")
	if err == nil || n != 4 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestAddLeadPostsPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/crm.lead.add" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"fields":{"TITLE":"x"}}` {
			t.Fatalf("body %s", body)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":7}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).AddLead(context.Background(), []byte(`{"fields":{"TITLE":"x"}}`))
	if err != nil || got.Status != http.StatusOK {
		t.Fatalf("status=%d err=%v", got.Status, err)
	}
}

func TestAddLeadNoRetryOn500(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		http.Error(w, "busy", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cli := New(srv.URL, srv.Client())
	cli.retryWaits = []time.Duration{0, 0, 0}
	_, err := cli.AddLead(context.Background(), []byte(`{"fields":{"TITLE":"x"}}`))
	if err == nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestLeadFieldsUnwrapsBareObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crm.lead.fields" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"TITLE":{"TYPE":"string"},"UF_CRM_1":{"TITLE":"Город","TYPE":"string","ATTRIBUTES":["DYN"]},"ID":{"TYPE":"integer","ATTRIBUTES":["R-O"]}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).LeadFields(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]LeadFieldInfo{}
	for _, f := range got {
		byCode[f.Code] = f
	}
	if byCode["TITLE"].Title != "Название" || !byCode["TITLE"].Writable {
		t.Fatalf("title %+v", byCode["TITLE"])
	}
	if byCode["UF_CRM_1"].Title != "Город" {
		t.Fatalf("uf %+v", byCode["UF_CRM_1"])
	}
	if byCode["ID"].Writable {
		t.Fatalf("id should be readonly %+v", byCode["ID"])
	}
}

func TestSourceListBareArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crm.source.list" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"STATUS_ID":"UC_L26K09","NAME":"GDMP","SORT":10}]`))
	}))
	defer srv.Close()
	got, err := New(srv.URL, srv.Client()).SourceList(context.Background())
	if err != nil || len(got) != 1 || got[0].StatusID != "UC_L26K09" {
		t.Fatalf("%+v err=%v", got, err)
	}
}
