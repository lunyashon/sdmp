package phonesearch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type %q", r.Header.Get("Content-Type"))
		}
		var in map[string]string
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatal(err)
		}
		if in["phone"] != "79031234567" {
			t.Errorf("phone %q", in["phone"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":{"code":903,"from":1000000,"to":1999999,"capacity":1000000,"operator":"ПАО \"ВЫМПЕЛКОМ\"","region":"Москва и Московская область","territory":"Город Москва, Московская область","inn":7713076301,"mobile_operator":"Билайн"}}`))
	}))
	defer srv.Close()

	cli := New(srv.URL, "test-token", "http://example", srv.Client())
	got, err := cli.Search(t.Context(), "79031234567")
	if err != nil {
		t.Fatal(err)
	}
	if got.Number.Code != 903 || got.Number.MobileOperator != "Билайн" || got.Number.INN != 7713076301 {
		t.Fatalf("%+v", got.Number)
	}
	if !strings.Contains(got.Number.Operator, "ВЫМПЕЛКОМ") {
		t.Fatalf("operator %q", got.Number.Operator)
	}
}

func TestSearchRequiresPhone(t *testing.T) {
	cli := New("http://example", "token", "http://example", nil)
	_, err := cli.Search(t.Context(), "  ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchHTTPError(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()

	cli := New(srv.URL, "bad", "http://example", srv.Client())
	cli.retryWaits = []time.Duration{0, 0, 0}
	_, err := cli.Search(t.Context(), "79031234567")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("got %v", err)
	}
	if n != 4 {
		t.Fatalf("attempts=%d", n)
	}
}

func TestSearchRetriesThenOK(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":{"code":903,"mobile_operator":"Билайн"}}`))
	}))
	defer srv.Close()

	cli := New(srv.URL, "test-token", srv.URL, srv.Client())
	cli.retryWaits = []time.Duration{0, 0, 0}
	got, err := cli.Search(t.Context(), "79031234567")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || got.Number.Code != 903 {
		t.Fatalf("n=%d got=%+v", n, got.Number)
	}
}
