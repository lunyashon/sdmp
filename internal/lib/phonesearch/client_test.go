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

func TestSearchNotFoundNoRetry(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		http.Error(w, `{"error":"phone not found","phone":"79409993206"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	cli := New(srv.URL, "test-token", "http://example", srv.Client())
	cli.retryWaits = []time.Duration{time.Second, time.Second, time.Second}
	got, err := cli.Search(t.Context(), "79409993206")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("attempts=%d", n)
	}
	if got.Number.Operator != "" || got.Number.Region != "" {
		t.Fatalf("%+v", got.Number)
	}
}

func TestRegionNameFallsBackToTerritory(t *testing.T) {
	n := NumberInfo{Region: "-", Territory: "Самарская область"}
	if got := n.RegionName(); got != "Самарская область" {
		t.Fatalf("%q", got)
	}
	n.Region = "Московская область"
	if got := n.RegionName(); got != "Московская область" {
		t.Fatalf("%q", got)
	}
	if !BlankRegion("-") || !BlankRegion("city") || BlankRegion("Татарстан") {
		t.Fatal("BlankRegion")
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

func TestForceCleanSkipsZennit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s", r.URL.Path)
	}))
	defer srv.Close()

	cli := New(srv.URL, "token", "https://crm-zennit.ru/api/v2/x/soda.lead.search", srv.Client()).
		WithZaryaBitrix("https://crm-zennit.ru/rest/1/abc/").
		WithForceClean(true)
	dup, err := cli.CheckDuplicate(t.Context(), "79001234567")
	if err != nil || dup {
		t.Fatalf("new dup=%v err=%v", dup, err)
	}
	oldDup, write, err := cli.CheckDuplicateOld(t.Context(), "79001234567")
	if err != nil || oldDup || !write {
		t.Fatalf("old dup=%v write=%v err=%v", oldDup, write, err)
	}
}

func TestCheckDuplicateOldDisabled(t *testing.T) {
	cli := New("http://example", "token", "http://example", nil)
	dup, write, err := cli.CheckDuplicateOld(t.Context(), "79001234567")
	if err != nil || write || dup {
		t.Fatalf("dup=%v write=%v err=%v", dup, write, err)
	}
}

func TestCheckDuplicateOldClean(t *testing.T) {
	var gotGet bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/crm.lead.get") {
			gotGet = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[]}`))
	}))
	defer srv.Close()

	cli := New(srv.URL, "token", srv.URL, srv.Client()).WithZaryaBitrix(srv.URL)
	dup, write, err := cli.CheckDuplicateOld(t.Context(), "79001234567")
	if err != nil || !write || dup || gotGet {
		t.Fatalf("dup=%v write=%v gotGet=%v err=%v", dup, write, gotGet, err)
	}
}

func TestCheckDuplicateOldNorthUsesLastLead(t *testing.T) {
	var gotID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/crm.duplicate.findbycomm"):
			_, _ = w.Write([]byte(`{"result":{"LEAD":[101,202]}}`))
		case strings.HasSuffix(r.URL.Path, "/crm.lead.get"):
			var in map[string]any
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				t.Error(err)
			}
			gotID = stringifyBitrixValue(in["id"])
			_, _ = w.Write([]byte(`{"result":{"ID":"202","DATE_CREATE":"2026-10-01T12:00:00+03:00","UF_CRM_1647447791":880}}`))
		default:
			t.Errorf("path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cli := New(srv.URL, "token", srv.URL, srv.Client()).WithZaryaBitrix(srv.URL)
	dup, write, err := cli.CheckDuplicateOld(t.Context(), "79001234567")
	if err != nil || !write || !dup {
		t.Fatalf("dup=%v write=%v err=%v", dup, write, err)
	}
	if gotID != "202" {
		t.Fatalf("lead id %q", gotID)
	}
}

func TestCheckDuplicateOldSouth(t *testing.T) {
	srv := zaryaOldBitrixServer(t, "881", "01.01.2020 00:00:00")
	defer srv.Close()

	cli := New(srv.URL, "token", srv.URL, srv.Client()).WithZaryaBitrix(srv.URL)
	dup, write, err := cli.CheckDuplicateOld(t.Context(), "79001234567")
	if err != nil || !write || !dup {
		t.Fatalf("dup=%v write=%v err=%v", dup, write, err)
	}
}

func TestCheckDuplicateOldLeadGetNotFoundIsClean(t *testing.T) {
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/crm.duplicate.findbycomm"):
			_, _ = w.Write([]byte(`{"result":{"LEAD":[999]}}`))
		case strings.HasSuffix(r.URL.Path, "/crm.lead.get"):
			gets++
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"","error_description":"Not found"}`))
		default:
			t.Errorf("path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cli := New(srv.URL, "token", srv.URL, srv.Client()).WithZaryaBitrix(srv.URL)
	dup, write, err := cli.CheckDuplicateOld(t.Context(), "79001234567")
	if err != nil || !write || dup {
		t.Fatalf("dup=%v write=%v err=%v", dup, write, err)
	}
	if gets != 1 {
		t.Fatalf("lead.get retries=%d want 1", gets)
	}
}

func TestCheckDuplicateOldEmptyMacroSkip(t *testing.T) {
	srv := zaryaOldBitrixServer(t, "", "2026-10-01T12:00:00+03:00")
	defer srv.Close()

	cli := New(srv.URL, "token", srv.URL, srv.Client()).WithZaryaBitrix(srv.URL)
	dup, write, err := cli.CheckDuplicateOld(t.Context(), "79001234567")
	if err != nil || write || dup {
		t.Fatalf("dup=%v write=%v err=%v", dup, write, err)
	}
}

func TestCheckDuplicateOldUnknownMacroSkip(t *testing.T) {
	srv := zaryaOldBitrixServer(t, "999", "2026-10-01T12:00:00+03:00")
	defer srv.Close()

	cli := New(srv.URL, "token", srv.URL, srv.Client()).WithZaryaBitrix(srv.URL)
	dup, write, err := cli.CheckDuplicateOld(t.Context(), "79001234567")
	if err != nil || write || dup {
		t.Fatalf("dup=%v write=%v err=%v", dup, write, err)
	}
}

func TestOldDuplicateVerdict(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		macro, date string
		dup, write  bool
	}{
		{"880", "2026-10-01T12:00:00Z", true, true},
		{"880", "2020-01-01T12:00:00Z", true, true},
		{"880", "", true, true},
		{"881", "2020-01-01 00:00:00", true, true},
		{"", "2026-10-01T12:00:00Z", false, false},
		{"999", "2026-10-01T12:00:00Z", false, false},
	}
	for _, tc := range cases {
		dup, write := oldDuplicateVerdict(tc.macro, tc.date, now)
		if dup != tc.dup || write != tc.write {
			t.Fatalf("%q %q: dup=%v write=%v want %v %v", tc.macro, tc.date, dup, write, tc.dup, tc.write)
		}
	}
}

func zaryaOldBitrixServer(t *testing.T, macro, dateCreate string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/crm.duplicate.findbycomm"):
			_, _ = w.Write([]byte(`{"result":{"LEAD":[55]}}`))
		case strings.HasSuffix(r.URL.Path, "/crm.lead.get"):
			out := map[string]any{
				"result": map[string]any{
					"ID":                  "55",
					"DATE_CREATE":         dateCreate,
					zaryaMacroregionField: macro,
				},
			}
			_ = json.NewEncoder(w).Encode(out)
		default:
			t.Errorf("path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}
