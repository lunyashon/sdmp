package skorozvon

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
)

type recCache struct {
	h map[string]map[string]string
}

func (c *recCache) Ping(context.Context) error { return nil }
func (c *recCache) HSet(_ context.Context, key string, values map[string]string) error {
	if c.h == nil {
		c.h = map[string]map[string]string{}
	}
	if c.h[key] == nil {
		c.h[key] = map[string]string{}
	}
	for k, v := range values {
		c.h[key][k] = v
	}
	return nil
}
func (c *recCache) HGet(_ context.Context, key, field string) (string, bool, error) {
	v, ok := c.h[key][field]
	return v, ok, nil
}
func (c *recCache) HGetAll(_ context.Context, key string) (map[string]string, error) {
	return c.h[key], nil
}
func (c *recCache) Expire(context.Context, string, time.Duration) error { return nil }
func (c *recCache) Del(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(c.h, k)
	}
	return nil
}
func (c *recCache) Close() error { return nil }

func TestDownloadCallRecordingUsesQueryToken(t *testing.T) {
	var sawAuth string
	var sawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/calls/51616666186.mp3" {
			t.Errorf("path %s", r.URL.Path)
		}
		sawAuth = r.Header.Get("Authorization")
		sawQuery = r.URL.Query().Get("access_token")
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = io.WriteString(w, "ID3fake")
	}))
	defer srv.Close()

	cli := New(Config{BaseURL: srv.URL}, &recCache{h: map[string]map[string]string{
		tokenKey: {fieldAccess: "tok-1"},
	}}, nil)
	raw, ctype, err := cli.DownloadCallRecording(context.Background(), "51616666186")
	if err != nil {
		t.Fatal(err)
	}
	if sawAuth != "" {
		t.Fatalf("authorization header %q", sawAuth)
	}
	if sawQuery != "tok-1" {
		t.Fatalf("query token %q", sawQuery)
	}
	if string(raw) != "ID3fake" || !strings.Contains(ctype, "mpeg") {
		t.Fatalf("%q %q", raw, ctype)
	}
}

func TestDownloadCallRecordingRejectsBadID(t *testing.T) {
	cli := New(Config{BaseURL: "http://example"}, &recCache{}, nil)
	_, _, err := cli.DownloadCallRecording(context.Background(), "../x")
	if err == nil || !strings.Contains(err.Error(), domain.ErrInvalidInput.Error()) {
		t.Fatalf("got %v", err)
	}
}
