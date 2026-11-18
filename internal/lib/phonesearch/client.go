package phonesearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/lib/curl"
)

var ErrNotFound = errors.New("not found")

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

// BlankRegion — заглушки вместо региона. phone.search с ~14.10.2026 отдаёт region="-"
// и кладёт субъект в territory.
func BlankRegion(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "-", "—", "–", "city", "n/a", "na", "нет", "null", "none":
		return true
	default:
		return false
	}
}

func (n NumberInfo) RegionName() string {
	if r := strings.TrimSpace(n.Region); !BlankRegion(r) {
		return r
	}
	return strings.TrimSpace(n.Territory)
}

type Client struct {
	http           *http.Client
	url            string
	token          string
	zaryaURL       string
	zaryaBitrixURL string
	forceClean     bool
	retryWaits     []time.Duration
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

// WithForceClean — не ходить на crm-zennit.ru, считать проверку «Чистый».
func (c *Client) WithForceClean(on bool) *Client {
	if c == nil {
		return nil
	}
	c.forceClean = on
	return c
}

func (c *Client) skipZennit(rawURL string) bool {
	return c != nil && c.forceClean && strings.Contains(strings.ToLower(rawURL), "crm-zennit.ru")
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
	if errors.Is(err, ErrNotFound) {
		return Result{}, nil
	}
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
	if c.skipZennit(c.zaryaURL) {
		return false, nil
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

func (c *Client) WithZaryaBitrix(bitrixURL string) *Client {
	if c == nil {
		return nil
	}
	c.zaryaBitrixURL = strings.TrimRight(strings.TrimSpace(bitrixURL), "/") + "/"
	if c.zaryaBitrixURL == "/" {
		c.zaryaBitrixURL = ""
	}
	return c
}

// Константы старого робота Зари (t.php / CRestR).
const (
	zaryaMacroregionField = "UF_CRM_1647447791"
	zaryaMacroNorthID     = "880" // NORTH / Север
	zaryaMacroSouthID     = "881" // SOUTH / Юг
	zaryaNorthWindowSec   = 7776000
)

// CheckDuplicateOld — 1:1 со старым роботом: findbycomm → последний LEAD → crm.lead.get →
// UF_CRM_1647447791 (880 NORTH / 881 SOUTH) и DATE_CREATE (90 суток для Севера).
// Нет лида → Чистый (write=true, duplicate=false).
// Север/Юг (ДС менее 90 / ДС более 90 / ДЮ) → дубль (write=true, duplicate=true).
// Пустой/чужой макрорегион → как робот: DESTINATION не пишем (write=false).
// write=false также если вебхук не задан.
func (c *Client) CheckDuplicateOld(ctx context.Context, phone string) (duplicate bool, write bool, err error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return false, false, fmt.Errorf("phone is required")
	}
	if c == nil || c.zaryaBitrixURL == "" {
		return false, false, nil
	}
	if c.skipZennit(c.zaryaBitrixURL) {
		return false, true, nil
	}

	body, err := json.Marshal(map[string]any{
		"type":        "PHONE",
		"values":      []string{phone},
		"entity_type": "LEAD",
	})
	if err != nil {
		return false, false, err
	}

	raw, err := c.postHeaders(ctx, c.zaryaBitrixURL+"crm.duplicate.findbycomm", body, "zarya old duplicate", map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return false, false, err
	}

	var env struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return false, false, fmt.Errorf("zarya old duplicate decode: %w", err)
	}
	if env.Error != "" {
		return false, false, fmt.Errorf("zarya old duplicate: %s", env.Error)
	}

	leadID, found := findbycommLastLeadID(env.Result)
	if !found {
		return false, true, nil
	}

	macro, dateCreate, err := c.zaryaLeadGet(ctx, leadID)
	if err != nil {
		// findbycomm отдал id, но lead.get = Not found (удалён/нет доступа) — как «нет лида» → Чистый.
		if errors.Is(err, ErrNotFound) {
			return false, true, nil
		}
		return false, false, err
	}
	dup, write := oldDuplicateVerdict(macro, dateCreate, time.Now())
	return dup, write, nil
}

func (c *Client) zaryaLeadGet(ctx context.Context, id string) (macro, dateCreate string, err error) {
	body, err := json.Marshal(map[string]any{"id": id})
	if err != nil {
		return "", "", err
	}
	raw, err := c.postHeaders(ctx, c.zaryaBitrixURL+"crm.lead.get", body, "zarya old lead get", map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return "", "", err
	}
	var env struct {
		Result           map[string]any `json:"result"`
		Error            string         `json:"error"`
		ErrorDescription string         `json:"error_description"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", "", fmt.Errorf("zarya old lead get decode: %w", err)
	}
	if env.Error != "" {
		msg := env.Error
		if env.ErrorDescription != "" {
			msg += ": " + env.ErrorDescription
		}
		return "", "", fmt.Errorf("zarya old lead get: %s", msg)
	}
	if env.Result == nil {
		return "", "", fmt.Errorf("zarya old lead get: empty result")
	}
	return stringifyBitrixValue(env.Result[zaryaMacroregionField]), stringifyBitrixValue(env.Result["DATE_CREATE"]), nil
}

// oldDuplicateVerdict — proceedResult из робота. Оба северных исхода и Юг пишем как дубль.
func oldDuplicateVerdict(macro, dateCreate string, now time.Time) (duplicate, write bool) {
	switch strings.TrimSpace(macro) {
	case zaryaMacroNorthID:
		created, ok := parseBitrixDate(dateCreate)
		age := now.Unix()
		if ok {
			age = now.Unix() - created.Unix()
		}
		if age <= zaryaNorthWindowSec {
			return true, true // ДС менее 90
		}
		return true, true // ДС более 90
	case zaryaMacroSouthID:
		return true, true // ДЮ
	default:
		return false, false
	}
}

func findbycommLastLeadID(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return "", false
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		return "", false
	}
	leads, ok := asMap["LEAD"]
	if !ok {
		return "", false
	}
	leads = bytes.TrimSpace(leads)
	if len(leads) == 0 || string(leads) == "null" {
		return "", false
	}
	var ids []any
	if err := json.Unmarshal(leads, &ids); err != nil {
		return "", false
	}
	if len(ids) == 0 {
		return "", false
	}
	id := stringifyBitrixValue(ids[len(ids)-1])
	return id, id != ""
}

func stringifyBitrixValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return strings.TrimSpace(x.String())
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case []any:
		if len(x) == 0 {
			return ""
		}
		return stringifyBitrixValue(x[0])
	default:
		s := strings.TrimSpace(fmt.Sprint(x))
		if s == "" || s == "<nil>" {
			return ""
		}
		return s
	}
}

func parseBitrixDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"02.01.2006 15:04:05",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (c *Client) post(ctx context.Context, url string, body []byte, op string) ([]byte, error) {
	return c.postHeaders(ctx, url, body, op, map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + c.token,
	})
}

func (c *Client) postHeaders(ctx context.Context, url string, body []byte, op string, headers map[string]string) ([]byte, error) {
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
		raw, status, err := curl.CurlWithContext(ctx, c.http, url, http.MethodPost, bytes.NewReader(body), headers)
		if err != nil {
			last = fmt.Errorf("%s: %w", op, err)
			continue
		}
		bodyStr := strings.TrimSpace(string(raw))
		if status == http.StatusNotFound || bitrixHTTPNotFound(status, bodyStr) {
			return nil, fmt.Errorf("%s: %w: %s", op, ErrNotFound, bodyStr)
		}
		if status < 200 || status >= 300 {
			last = fmt.Errorf("%s: status %d: %s", op, status, bodyStr)
			continue
		}
		return raw, nil
	}
	return nil, last
}

// bitrixHTTPNotFound — Bitrix часто отдаёт 400 + error_description "Not found" вместо 404.
func bitrixHTTPNotFound(status int, body string) bool {
	if status != http.StatusBadRequest {
		return false
	}
	lower := strings.ToLower(body)
	return strings.Contains(lower, `"error_description":"not found"`) ||
		strings.Contains(lower, `"error":"not found"`) ||
		(strings.Contains(lower, "not found") && strings.Contains(lower, "error"))
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
