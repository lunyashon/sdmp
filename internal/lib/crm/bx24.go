package crm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/lib/curl"
)

var defaultRetryWaits = []time.Duration{5 * time.Second, 15 * time.Second, 25 * time.Second}

type Client struct {
	http       *http.Client
	base       string
	retryWaits []time.Duration
}

func New(base string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		http: httpClient,
		base: strings.TrimRight(strings.TrimSpace(base), "/") + "/",
	}
}

type LeadSearchResult struct {
	ID  int64
	Raw []byte
}

type LeadAddResult struct {
	ID     int64
	Raw    []byte
	Status int
}

func (c *Client) SearchLead(ctx context.Context, idempotencyKey string) (LeadSearchResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return LeadSearchResult{}, fmt.Errorf("idempotency_key is required")
	}
	if c.base == "/" {
		return LeadSearchResult{}, fmt.Errorf("bx24 url is empty")
	}

	u, err := url.Parse(c.base + "sdmp.lead.search")
	if err != nil {
		return LeadSearchResult{}, fmt.Errorf("bx24 search url: %w", err)
	}
	q := u.Query()
	q.Set("idempotency_key", idempotencyKey)
	u.RawQuery = q.Encode()

	raw, status, err := c.do(ctx, http.MethodGet, u.String(), nil, map[string]string{
		"Accept": "application/json",
	}, "bx24 search")
	out := LeadSearchResult{Raw: raw}
	if err != nil {
		return out, err
	}
	if status < 200 || status >= 300 {
		return out, fmt.Errorf("bx24 search: status %d: %s", status, strings.TrimSpace(string(raw)))
	}

	var env struct {
		Result           json.RawMessage `json:"result"`
		Error            string          `json:"error"`
		ErrorDescription string          `json:"error_description"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return out, fmt.Errorf("bx24 search decode: %w", err)
	}
	if env.Error != "" {
		if env.ErrorDescription != "" {
			return out, fmt.Errorf("bx24 search: %s: %s", env.Error, env.ErrorDescription)
		}
		return out, fmt.Errorf("bx24 search: %s", env.Error)
	}
	id, err := jsonNumber(env.Result)
	if err != nil {
		return out, fmt.Errorf("bx24 search result: %w", err)
	}
	out.ID = id
	return out, nil
}

func (c *Client) AddLead(ctx context.Context, payload []byte) (LeadAddResult, error) {
	if c.base == "/" {
		return LeadAddResult{}, fmt.Errorf("bx24 url is empty")
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return LeadAddResult{}, fmt.Errorf("payload is empty")
	}

	raw, status, err := curl.CurlWithContext(ctx, c.http, c.base+"crm.lead.add", http.MethodPost, bytes.NewReader(payload), map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	out := LeadAddResult{Raw: raw, Status: status}
	if err != nil {
		return out, fmt.Errorf("bx24 crm.lead.add: %w", err)
	}
	if status < 200 || status >= 300 {
		return out, fmt.Errorf("bx24 crm.lead.add: status %d: %s", status, strings.TrimSpace(string(raw)))
	}
	var env struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err == nil {
		if id, err := jsonNumber(env.Result); err == nil {
			out.ID = id
		}
	}
	return out, nil
}

func (c *Client) do(ctx context.Context, method, rawURL string, body []byte, headers map[string]string, op string) ([]byte, int, error) {
	waits := c.retryWaits
	if waits == nil {
		waits = defaultRetryWaits
	}

	var (
		lastRaw    []byte
		lastStatus int
		lastErr    error
	)
	for attempt := 0; attempt <= len(waits); attempt++ {
		if attempt > 0 {
			if err := sleep(ctx, waits[attempt-1]); err != nil {
				if lastErr != nil {
					return lastRaw, lastStatus, lastErr
				}
				return lastRaw, lastStatus, fmt.Errorf("%s: %w", op, err)
			}
		}

		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		raw, status, err := curl.CurlWithContext(ctx, c.http, rawURL, method, reader, headers)
		lastRaw, lastStatus = raw, status
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", op, err)
			if isTimeout(err) {
				continue
			}
			return raw, status, lastErr
		}
		if status == http.StatusInternalServerError {
			lastErr = fmt.Errorf("%s: status %d: %s", op, status, strings.TrimSpace(string(raw)))
			continue
		}
		return raw, status, nil
	}
	return lastRaw, lastStatus, lastErr
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func jsonNumber(raw json.RawMessage) (int64, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, nil
		}
		return strconv.ParseInt(s, 10, 64)
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, err
	}
	return n.Int64()
}
