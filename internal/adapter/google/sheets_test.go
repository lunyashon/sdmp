package google

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAppendRowPostsPhone(t *testing.T) {
	key := mustRSA(t)
	var gotRange, gotAuth, gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Fatalf("grant %s", r.Form.Get("grant_type"))
		}
		if r.Form.Get("assertion") == "" {
			t.Fatal("missing assertion")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok-1","expires_in":3600}`))
	})
	mux.HandleFunc("/v4/spreadsheets/", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotRange = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"updates":{"updatedRows":1}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := New()
	client.http = srv.Client()
	client.http.Transport = rewriteHost{base: srv.URL, next: srv.Client().Transport}

	creds, _ := json.Marshal(map[string]string{
		"client_email": "dev-service@example.iam.gserviceaccount.com",
		"private_key":  pemPKCS8(t, key),
		"token_uri":    srv.URL + "/token",
	})
	status, body, err := client.AppendRow(context.Background(), creds, "sheet-id", "Телефоны", []string{"79001234567"})
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 || !strings.Contains(string(body), "updatedRows") {
		t.Fatalf("status %d body %s", status, body)
	}
	if gotAuth != "Bearer tok-1" {
		t.Fatalf("auth %s", gotAuth)
	}
	if !strings.Contains(gotRange, "/v4/spreadsheets/sheet-id/values/") || !strings.Contains(gotRange, "A:ZZ") {
		t.Fatalf("range path %s", gotRange)
	}
	if !strings.Contains(gotBody, `"79001234567"`) {
		t.Fatalf("body %s", gotBody)
	}
}

func TestHeaderRowReadsFirstSheet(t *testing.T) {
	key := mustRSA(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok-1","expires_in":3600}`))
	})
	mux.HandleFunc("/v4/spreadsheets/sheet-id", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sheets":[{"properties":{"title":"Лиды"}},{"properties":{"title":"Архив"}}]}`))
	})
	mux.HandleFunc("/v4/spreadsheets/sheet-id/values/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "1:1") {
			t.Fatalf("path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[["Телефон","","Город"]]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := New()
	client.http = srv.Client()
	client.http.Transport = rewriteHost{base: srv.URL, next: srv.Client().Transport}
	creds, _ := json.Marshal(map[string]string{
		"client_email": "dev-service@example.iam.gserviceaccount.com",
		"private_key":  pemPKCS8(t, key),
		"token_uri":    srv.URL + "/token",
	})
	sheet, columns, sheets, err := client.HeaderRow(context.Background(), creds, "sheet-id", "")
	if err != nil {
		t.Fatal(err)
	}
	if sheet != "Лиды" || strings.Join(columns, "|") != "Телефон|Колонка 2|Город" {
		t.Fatalf("sheet %s columns %v", sheet, columns)
	}
	if strings.Join(sheets, "|") != "Лиды|Архив" {
		t.Fatalf("sheets %v", sheets)
	}
}

func TestAccessTokenCached(t *testing.T) {
	key := mustRSA(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"access_token":"tok-cached","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)

	client := New()
	client.http = srv.Client()
	sa := serviceAccount{
		ClientEmail: "dev-service@example.iam.gserviceaccount.com",
		PrivateKey:  pemPKCS8(t, key),
		TokenURI:    srv.URL,
	}
	now := time.Unix(1_700_000_000, 0)
	client.now = func() time.Time { return now }
	if _, err := client.accessToken(context.Background(), sa); err != nil {
		t.Fatal(err)
	}
	if _, err := client.accessToken(context.Background(), sa); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("token calls %d", calls)
	}
}

type rewriteHost struct {
	base string
	next http.RoundTripper
}

func (r rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	u, err := url.Parse(r.base)
	if err != nil {
		return nil, err
	}
	if strings.Contains(req.URL.Host, "sheets.googleapis.com") {
		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		req.Host = u.Host
	}
	next := r.next
	if next == nil {
		next = http.DefaultTransport
	}
	return next.RoundTrip(req)
}

func mustRSA(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func pemPKCS8(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestLiveAppendRow(t *testing.T) {
	path := os.Getenv("GOOGLE_SHEETS_CREDS")
	if os.Getenv("GOOGLE_SHEETS_LIVE") == "" || path == "" {
		t.Skip("set GOOGLE_SHEETS_LIVE=1 and GOOGLE_SHEETS_CREDS")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	spreadsheetID := os.Getenv("GOOGLE_SHEETS_ID")
	sheetName := os.Getenv("GOOGLE_SHEETS_TAB")
	if spreadsheetID == "" || sheetName == "" {
		t.Fatal("GOOGLE_SHEETS_ID and GOOGLE_SHEETS_TAB are required")
	}
	status, body, err := New().AppendRow(context.Background(), raw, spreadsheetID, sheetName, []string{"70000000000"})
	if err != nil {
		t.Fatalf("append: %v body=%s", err, body)
	}
	if status < 200 || status >= 300 {
		t.Fatalf("status %d body %s", status, body)
	}
}
