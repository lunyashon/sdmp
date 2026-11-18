package crmcalls

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/ratelimit"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.Crmcalls = (*Client)(nil)

const defaultBaseURL = "https://cc.crmcalls.ru"

// Один импорт на ключ раз в 30 секунд — держим очередь в процессе.
var importMu sync.Mutex

type Config struct {
	BaseURL string
	APIKey  string
	RPS     int
}

type Client struct {
	http *http.Client
	cfg  Config
	pace *ratelimit.Pace
}

func New(cfg Config) *Client {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	if strings.HasPrefix(strings.ToLower(cfg.APIKey), "bearer ") {
		cfg.APIKey = strings.TrimSpace(cfg.APIKey[7:])
	}
	return &Client{
		http: &http.Client{Timeout: 30 * time.Second},
		cfg:  cfg,
		pace: ratelimit.NewPace(cfg.RPS),
	}
}

func (c *Client) SendLead(ctx context.Context, lead port.CrmcallsLead) (port.CrmcallsSendResult, error) {
	phone := strings.TrimSpace(lead.Phone)
	if phone == "" {
		return port.CrmcallsSendResult{}, fmt.Errorf("phone is empty")
	}
	project := strings.TrimSpace(lead.ProjectID)
	if project == "" {
		return port.CrmcallsSendResult{}, fmt.Errorf("project_id is required")
	}

	contact := map[string]any{"phones": []string{phone}}
	if id := strings.TrimSpace(lead.ExternalID); id != "" {
		contact["external_id"] = id
	}
	if name := strings.TrimSpace(lead.Name); name != "" {
		contact["full_name"] = name
	}
	if region := strings.TrimSpace(lead.Region); region != "" {
		contact["region"] = region
	}
	body := map[string]any{
		"data":          []any{contact},
		"deduplication": map[string]any{"mode": "update"},
		"project_id":    project,
	}

	importMu.Lock()
	defer importMu.Unlock()

	raw, status, err := c.postImport(ctx, body)
	out := port.CrmcallsSendResult{Status: status, Body: raw}
	if err != nil {
		return out, err
	}
	if status < 200 || status >= 300 {
		return out, fmt.Errorf("crmcalls HTTP %d: %s", status, trimBody(raw))
	}

	importID := extractImportID(raw)
	if importID != "" {
		waitRaw, waitStatus, waitErr := c.waitImport(ctx, importID)
		if len(waitRaw) > 0 {
			out.Body = waitRaw
			out.Status = waitStatus
		}
		if waitErr != nil {
			return out, waitErr
		}
	}
	out.ContactID = c.lookupContactID(ctx, strings.TrimSpace(lead.ExternalID), phone, importID)
	return out, nil
}

func (c *Client) postImport(ctx context.Context, body map[string]any) ([]byte, int, error) {
	raw, status, err := c.doJSON(ctx, http.MethodPost, "/api/v1/contacts/import", body)
	if status != http.StatusTooManyRequests {
		return raw, status, err
	}
	waitFor := importRetryAfter(raw)
	if err := sleepCtx(ctx, waitFor); err != nil {
		return raw, status, err
	}
	return c.doJSON(ctx, http.MethodPost, "/api/v1/contacts/import", body)
}

func (c *Client) waitImport(ctx context.Context, importID string) ([]byte, int, error) {
	deadline := time.Now().Add(90 * time.Second)
	pause := time.Second
	var last []byte
	var lastStatus int
	for {
		raw, status, err := c.doJSON(ctx, http.MethodGet, "/api/v1/contacts/import/"+url.PathEscape(importID), nil)
		last, lastStatus = raw, status
		if err != nil {
			return raw, status, err
		}
		if status < 200 || status >= 300 {
			return raw, status, fmt.Errorf("crmcalls HTTP %d: %s", status, trimBody(raw))
		}
		state := importState(raw)
		switch state {
		case "completed", "completed_with_errors":
			return raw, status, nil
		case "failed":
			return raw, status, fmt.Errorf("crmcalls import failed: %s", trimBody(raw))
		}
		if time.Now().After(deadline) {
			return last, lastStatus, fmt.Errorf("crmcalls import timeout: %s", trimBody(last))
		}
		if err := sleepCtx(ctx, pause); err != nil {
			return last, lastStatus, err
		}
		if pause < 5*time.Second {
			pause += time.Second
		}
	}
}

func (c *Client) lookupContactID(ctx context.Context, externalID, phone, importID string) string {
	queries := make([]string, 0, 3)
	if externalID != "" {
		queries = append(queries, "external_id="+url.QueryEscape(externalID))
	}
	if importID != "" {
		queries = append(queries, "import_id="+url.QueryEscape(importID))
	}
	if phone != "" {
		queries = append(queries, "phone="+url.QueryEscape(phone))
	}
	for _, qs := range queries {
		raw, status, err := c.doJSON(ctx, http.MethodGet, "/api/v1/contacts?"+qs, nil)
		if err != nil || status < 200 || status >= 300 {
			continue
		}
		if id := firstContactID(raw); id != "" {
			return id
		}
	}
	return ""
}

func (c *Client) ListProjects(ctx context.Context) ([]entity.CrmcallsProject, error) {
	raw, status, err := c.doJSON(ctx, http.MethodGet, "/api/v1/projects", nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("crmcalls HTTP %d: %s", status, trimBody(raw))
	}
	return parseProjects(raw), nil
}

func (c *Client) DownloadRecording(ctx context.Context, callIDOrURL string) ([]byte, string, error) {
	ref := strings.TrimSpace(callIDOrURL)
	if ref == "" {
		return nil, "", fmt.Errorf("crmcalls recording ref is empty")
	}
	u := ref
	if !strings.HasPrefix(strings.ToLower(ref), "http://") && !strings.HasPrefix(strings.ToLower(ref), "https://") {
		u = c.cfg.BaseURL + "/api/recordings/" + url.PathEscape(ref)
	}
	if c.cfg.APIKey == "" {
		return nil, "", fmt.Errorf("crmcalls api key is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Accept", "audio/mpeg, application/octet-stream, */*")
	if err := c.pace.Wait(ctx); err != nil {
		return nil, "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		ctype := strings.TrimSpace(resp.Header.Get("Content-Type"))
		if ctype == "" {
			ctype = "audio/mpeg"
		}
		return raw, ctype, nil
	case http.StatusAccepted:
		return nil, "", fmt.Errorf("crmcalls recording not ready: %s", trimBody(raw))
	case http.StatusNotFound:
		return nil, "", fmt.Errorf("crmcalls recording unavailable: %s", trimBody(raw))
	default:
		return nil, "", fmt.Errorf("crmcalls recording HTTP %d: %s", resp.StatusCode, trimBody(raw))
	}
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	if c.cfg.APIKey == "" {
		return nil, 0, fmt.Errorf("crmcalls api key is empty")
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.pace.Wait(ctx); err != nil {
		return nil, 0, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func extractContactID(raw []byte) string {
	if id := firstContactID(raw); id != "" {
		return id
	}
	var decoded struct {
		Data json.RawMessage `json:"data"`
		ID   string          `json:"id"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return ""
	}
	if id := strings.TrimSpace(decoded.ID); id != "" {
		return id
	}
	var inner struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(decoded.Data, &inner); err == nil {
		return strings.TrimSpace(inner.ID)
	}
	return ""
}

func firstContactID(raw []byte) string {
	var decoded struct {
		Data json.RawMessage `json:"data"`
	}
	payload := raw
	if err := json.Unmarshal(raw, &decoded); err == nil && len(decoded.Data) > 0 {
		payload = decoded.Data
	}
	var items []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload, &items); err == nil {
		for _, item := range items {
			if id := strings.TrimSpace(item.ID); id != "" {
				return id
			}
		}
	}
	var one struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload, &one); err == nil {
		return strings.TrimSpace(one.ID)
	}
	return ""
}

func extractImportID(raw []byte) string {
	var decoded struct {
		Data     json.RawMessage `json:"data"`
		ImportID string          `json:"import_id"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return ""
	}
	if id := strings.TrimSpace(decoded.ImportID); id != "" {
		return id
	}
	var inner struct {
		ImportID string `json:"import_id"`
	}
	if err := json.Unmarshal(decoded.Data, &inner); err != nil {
		return ""
	}
	return strings.TrimSpace(inner.ImportID)
}

func importState(raw []byte) string {
	var decoded struct {
		Data   json.RawMessage `json:"data"`
		State  string          `json:"state"`
		Status string          `json:"status"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return ""
	}
	if s := strings.ToLower(strings.TrimSpace(decoded.State)); s != "" {
		return s
	}
	if s := strings.ToLower(strings.TrimSpace(decoded.Status)); s != "" {
		return s
	}
	var inner struct {
		State  string `json:"state"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(decoded.Data, &inner); err != nil {
		return ""
	}
	if s := strings.ToLower(strings.TrimSpace(inner.State)); s != "" {
		return s
	}
	return strings.ToLower(strings.TrimSpace(inner.Status))
}

func importRetryAfter(raw []byte) time.Duration {
	var decoded struct {
		Error struct {
			Details struct {
				RetryAfter int `json:"retry_after"`
			} `json:"details"`
		} `json:"error"`
	}
	n := 30
	if err := json.Unmarshal(raw, &decoded); err == nil && decoded.Error.Details.RetryAfter > 0 {
		n = decoded.Error.Details.RetryAfter
	}
	if n > 60 {
		n = 60
	}
	return time.Duration(n) * time.Second
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
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

func parseProjects(raw []byte) []entity.CrmcallsProject {
	var decoded struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	payload := decoded.Data
	if len(payload) == 0 {
		payload = raw
	}
	var items []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(payload, &items); err != nil {
		var wrap struct {
			Items json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(payload, &wrap); err == nil && len(wrap.Items) > 0 {
			_ = json.Unmarshal(wrap.Items, &items)
		}
	}
	out := make([]entity.CrmcallsProject, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = strings.TrimSpace(item.Title)
		}
		if name == "" {
			name = id
		}
		out = append(out, entity.CrmcallsProject{ID: id, Name: name, Status: strings.TrimSpace(item.Status)})
	}
	return out
}

func trimBody(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 300 {
		return s[:300]
	}
	return s
}
