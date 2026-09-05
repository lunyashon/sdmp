package phonesearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/lib/curl"
)

var defaultRetryWaits = []time.Duration{time.Second, 3 * time.Second, 9 * time.Second}

type NumberInfo struct {
	Code           int    `json:"code"`
	From           int64  `json:"from"`
	To             int64  `json:"to"`
	Capacity       int64  `json:"capacity"`
	Operator       string `json:"operator"`
	Region         string `json:"region"`
	Territory      string `json:"territory"`
	INN            int64  `json:"inn"`
	MobileOperator string `json:"mobile_operator"`
}

type Result struct {
	Number NumberInfo `json:"number"`
}

type Client struct {
	http       *http.Client
	url        string
	token      string
	zaryaURL   string
	retryWaits []time.Duration
}

func New(url, token, zaryaURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		http:     httpClient,
		url:      strings.TrimRight(strings.TrimSpace(url), "/"),
		token:    strings.TrimSpace(token),
		zaryaURL: strings.TrimSpace(zaryaURL),
	}
}

func (c *Client) Search(ctx context.Context, phone string) (Result, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return Result{}, fmt.Errorf("phone is required")
	}
	if c.url == "" {
		return Result{}, fmt.Errorf("phone search url is empty")
	}
	if c.token == "" {
		return Result{}, fmt.Errorf("phone search token is empty")
	}

	body, err := json.Marshal(map[string]string{"phone": phone})
	if err != nil {
		return Result{}, err
	}

	raw, err := c.post(ctx, c.url, body, "phone search")
	if err != nil {
		return Result{}, err
	}

	var out Result
	if err := json.Unmarshal(raw, &out); err != nil {
		return Result{}, fmt.Errorf("phone search decode: %w", err)
	}
	return out, nil
}

type DuplicateCheckRequest struct {
	Phone string `json:"phone"`
}

type DuplicateCheckResponse struct {
	Result bool `json:"result"`
}

func (c *Client) CheckDuplicate(ctx context.Context, phone string) (bool, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return false, fmt.Errorf("phone is required")
	}
	if c.zaryaURL == "" {
		return false, fmt.Errorf("zarya url is empty")
	}
	if c.token == "" {
		return false, fmt.Errorf("phone search token is empty")
	}

	body, err := json.Marshal(DuplicateCheckRequest{Phone: phone})
	if err != nil {
		return false, fmt.Errorf("marshal request: %w", err)
	}

	raw, err := c.post(ctx, c.zaryaURL, body, "check duplicate")
	if err != nil {
		return false, err
	}

	var out DuplicateCheckResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return false, fmt.Errorf("check duplicate decode: %w", err)
	}
	return out.Result, nil
}

func (c *Client) post(ctx context.Context, url string, body []byte, op string) ([]byte, error) {
	waits := c.retryWaits
	if waits == nil {
		waits = defaultRetryWaits
	}

	var last error
	for attempt := 0; attempt <= len(waits); attempt++ {
		if attempt > 0 {
			if err := sleep(ctx, waits[attempt-1]); err != nil {
				return nil, fmt.Errorf("%s: %w", op, err)
			}
		}
		raw, status, err := curl.CurlWithContext(ctx, c.http, url, http.MethodPost, bytes.NewReader(body), map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + c.token,
		})
		if err != nil {
			last = fmt.Errorf("%s: %w", op, err)
			continue
		}
		if status < 200 || status >= 300 {
			last = fmt.Errorf("%s: status %d: %s", op, status, strings.TrimSpace(string(raw)))
			continue
		}
		return raw, nil
	}
	return nil, last
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
