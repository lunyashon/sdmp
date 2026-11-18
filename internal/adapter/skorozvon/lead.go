package skorozvon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/lib/curl"
	"github.com/lunyashon/sdmp/internal/lib/ratelimit"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.Skorozvon = (*Client)(nil)

const (
	defaultBaseURL = "https://api.skorozvon.ru"
	tokenKey       = "skorozvon:tokens"
	fieldAccess    = "access_token"
	tokenTTL       = 600 * time.Second
)

var retryDelays = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	5 * time.Second,
	7 * time.Second,
	9 * time.Second,
}

type Config struct {
	BaseURL string
	Login   string
	APIKey  string
	AppID   string
	AppKey  string
	RPS     int
}

type Client struct {
	log   *slog.Logger
	http  *http.Client
	cfg   Config
	cache port.Redis
	pace  *ratelimit.Pace
	mu    sync.Mutex
}

func New(cfg Config, cache port.Redis, log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	return &Client{
		log:   log,
		cache: cache,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		cfg:  cfg,
		pace: ratelimit.NewPace(cfg.RPS),
	}
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, 0, err
	}

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	headers := map[string]string{
		"Accept":        "application/json",
		"Authorization": "Bearer " + token,
	}
	if body != nil {
		headers["Content-Type"] = "application/json"
	}

	if err := c.pace.Wait(ctx); err != nil {
		return nil, 0, err
	}

	raw, code, err := curl.CurlWithContext(ctx, c.http, c.cfg.BaseURL+path, method, reader, headers)
	if err != nil {
		return nil, 0, err
	}
	if code == http.StatusUnauthorized {
		_ = c.cache.Del(ctx, tokenKey)
		return raw, code, fmt.Errorf("%w: %s", domain.ErrUnauthorized, raw)
	}
	return raw, code, nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	access, _, err := c.cache.HGet(ctx, tokenKey, fieldAccess)
	if err != nil {
		return "", fmt.Errorf("get access_token: %w", err)
	}
	if access != "" {
		return access, nil
	}

	return c.fetchToken(ctx)
}

func (c *Client) fetchToken(ctx context.Context) (string, error) {
	form := url.Values{
		"grant_type":    {"password"},
		"username":      {c.cfg.Login},
		"api_key":       {c.cfg.APIKey},
		"client_id":     {c.cfg.AppID},
		"client_secret": {c.cfg.AppKey},
	}
	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/x-www-form-urlencoded",
	}

	raw, code, err := curl.CurlWithContext(
		ctx,
		c.http,
		c.cfg.BaseURL+"/oauth/token",
		http.MethodPost,
		strings.NewReader(form.Encode()),
		headers,
	)
	if err != nil {
		return "", fmt.Errorf("skorozvon token: %w", err)
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("%w: token http %d: %s", domain.ErrUnauthorized, code, raw)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("%w: empty access_token", domain.ErrUnauthorized)
	}

	if err := c.cache.HSet(ctx, tokenKey, map[string]string{
		fieldAccess: tok.AccessToken,
	}); err != nil {
		return "", fmt.Errorf("store access_token: %w", err)
	}
	if err := c.cache.Expire(ctx, tokenKey, tokenTTL); err != nil {
		return "", fmt.Errorf("expire access_token: %w", err)
	}

	return tok.AccessToken, nil
}

func (c *Client) SendLead(ctx context.Context, req port.SkorozvonLead) (port.SkorozvonSendResult, error) {
	if strings.TrimSpace(req.Phone) == "" {
		return port.SkorozvonSendResult{}, fmt.Errorf("%w: phone is empty", domain.ErrInvalidInput)
	}

	body := map[string]any{
		"phones":          req.Phone,
		"call_project_id": req.ProjectID,
	}
	if req.City != "" {
		body["city"] = req.City
	}
	if req.Region != "" {
		body["region"] = req.Region
	}
	if req.ExternalID != "" {
		body["external_id"] = req.ExternalID
	}
	if len(req.CustomFields) > 0 {
		body["custom_fields"] = req.CustomFields
	}

	raw, code, err := c.doJSON(ctx, http.MethodPost, "/api/v2/leads", body)
	out := port.SkorozvonSendResult{Status: code, Body: raw}
	if err != nil {
		return out, err
	}
	if code != http.StatusOK {
		return out, c.statusError(code, raw)
	}

	var created struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		return out, fmt.Errorf("decode create lead: %w", err)
	}
	if created.ID == 0 {
		return out, fmt.Errorf("create lead: missing id in response: %s", raw)
	}
	out.LeadID = created.ID
	return out, nil
}

func (c *Client) CheckProject(ctx context.Context, projectID int64) bool {
	if projectID <= 0 {
		return false
	}
	_, code, err := c.doJSON(ctx, http.MethodGet, "/api/v2/call_projects/"+strconv.FormatInt(projectID, 10)+"/statistic", nil)
	if err != nil {
		c.log.WarnContext(ctx, "skorozvon project check failed", "project_id", projectID, "error", err)
		return false
	}
	return code == http.StatusOK
}

func (c *Client) DeleteLead(ctx context.Context, id int) error {
	if id <= 0 {
		return fmt.Errorf("%w: lead id", domain.ErrInvalidInput)
	}

	path := "/api/v2/leads/" + strconv.Itoa(id)
	return c.withRetry(ctx, func() error {
		raw, code, err := c.doJSON(ctx, http.MethodDelete, path, nil)
		if err != nil {
			return err
		}
		if code >= 300 {
			return c.statusError(code, raw)
		}
		return nil
	})
}

func (c *Client) withRetry(ctx context.Context, fn func() error) error {
	var last error
	for i := 0; i <= len(retryDelays); i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		last = fn()
		if last == nil {
			return nil
		}
		if i == len(retryDelays) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelays[i]):
		}
	}
	return last
}

func (c *Client) statusError(code int, raw []byte) error {
	switch code {
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", domain.ErrUnauthorized, raw)
	case http.StatusForbidden:
		return fmt.Errorf("%w: %s", domain.ErrForbidden, raw)
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", domain.ErrNotFound, raw)
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return fmt.Errorf("%w: http %d: %s", domain.ErrInvalidInput, code, raw)
	default:
		return fmt.Errorf("skorozvon http %d: %s", code, raw)
	}
}
