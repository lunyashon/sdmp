package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type CrmcallsLinkAPI interface {
	List(ctx context.Context) ([]entity.CrmcallsLink, error)
	Create(ctx context.Context, in usecase.CrmcallsLinkInput) (entity.CrmcallsLink, error)
	Update(ctx context.Context, id int, in usecase.CrmcallsLinkInput) (entity.CrmcallsLink, error)
	Delete(ctx context.Context, id int) error
}

type CrmcallsProjectAPI interface {
	List(ctx context.Context) ([]entity.CrmcallsProject, error)
	Sync(ctx context.Context) ([]entity.CrmcallsProject, error)
	Refresh(ctx context.Context) ([]entity.CrmcallsProject, error)
	SendTest(ctx context.Context, projectID string) (entity.CrmcallsTestResult, error)
	Connection(ctx context.Context) (entity.CrmcallsConnection, error)
	Connect(ctx context.Context, apiKey, baseURL string) ([]entity.CrmcallsProject, error)
}

type CrmcallsPostbackAPI interface {
	ApplyCrmcallsPostback(ctx context.Context, in entity.CrmcallsPostback) error
}

type CrmcallsHandler struct {
	links       CrmcallsLinkAPI
	projects    CrmcallsProjectAPI
	events      CrmcallsPostbackAPI
	log         *slog.Logger
	postbackKey string
}

func NewCrmcallsHandler(links CrmcallsLinkAPI, projects CrmcallsProjectAPI, events CrmcallsPostbackAPI, log *slog.Logger) *CrmcallsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &CrmcallsHandler{links: links, projects: projects, events: events, log: log}
}

func (h *CrmcallsHandler) WithPostbackKey(key string) *CrmcallsHandler {
	if h == nil {
		return nil
	}
	h.postbackKey = strings.TrimSpace(key)
	return h
}

func (h *CrmcallsHandler) ListLinks(c *gin.Context) {
	if h.links == nil {
		response.List(c, []entity.CrmcallsLink{})
		return
	}
	items, err := h.links.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

func (h *CrmcallsHandler) CreateLink(c *gin.Context) {
	in, ok := bindCrmcallsLink(c)
	if !ok {
		return
	}
	item, err := h.links.Create(c.Request.Context(), in)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.JSON(c, http.StatusCreated, item)
}

func (h *CrmcallsHandler) UpdateLink(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "link id is required")
		return
	}
	in, ok := bindCrmcallsLink(c)
	if !ok {
		return
	}
	item, err := h.links.Update(c.Request.Context(), id, in)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, item)
}

func (h *CrmcallsHandler) DeleteLink(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "link id is required")
		return
	}
	if err := h.links.Delete(c.Request.Context(), id); err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CrmcallsHandler) ListProjects(c *gin.Context) {
	if h.projects == nil {
		response.List(c, []entity.CrmcallsProject{})
		return
	}
	items, err := h.projects.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

func (h *CrmcallsHandler) SyncProjects(c *gin.Context) {
	if h.projects == nil {
		response.Error(c, http.StatusBadRequest, "crmcalls api is not configured")
		return
	}
	items, err := h.projects.Sync(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

func (h *CrmcallsHandler) RefreshProjects(c *gin.Context) {
	if h.projects == nil {
		response.Error(c, http.StatusBadRequest, "crmcalls api is not configured")
		return
	}
	items, err := h.projects.Refresh(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

func (h *CrmcallsHandler) SendTest(c *gin.Context) {
	if h.projects == nil {
		response.Error(c, http.StatusBadRequest, "crmcalls api is not configured")
		return
	}
	item, err := h.projects.SendTest(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, item)
}

func (h *CrmcallsHandler) GetSettings(c *gin.Context) {
	if h.projects == nil {
		response.Success(c, entity.CrmcallsConnection{BaseURL: "https://cc.crmcalls.ru"})
		return
	}
	item, err := h.projects.Connection(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, item)
}

func (h *CrmcallsHandler) SaveSettings(c *gin.Context) {
	if h.projects == nil {
		response.Error(c, http.StatusBadRequest, "crmcalls api is not configured")
		return
	}
	var req dto.CrmcallsSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid json")
		return
	}
	items, err := h.projects.Connect(c.Request.Context(), req.APIKey, req.BaseURL)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

func (h *CrmcallsHandler) Receive(c *gin.Context) {
	if !crmcallsKeyOK(h.postbackKey, c.GetHeader("X-API-KEY")) {
		h.log.Warn("crmcalls postback forbidden", "remote", c.ClientIP())
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.log.Warn("crmcalls postback read failed", "error", err, "method", c.Request.Method, "path", c.Request.URL.Path)
		response.Error(c, http.StatusBadRequest, "cannot read body")
		return
	}
	h.log.Info("crmcalls postback",
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"query", c.Request.URL.RawQuery,
		"content_type", c.Request.Header.Get("Content-Type"),
		"user_agent", c.Request.Header.Get("User-Agent"),
		"remote", c.ClientIP(),
		"body", crmcallsLogBody(raw),
	)
	in := parseCrmcallsPostback(raw, c.Request.URL.Query())
	h.log.Info("crmcalls postback parsed",
		"id", in.ID,
		"name", in.Name,
		"phone", in.Phone,
		"result", in.Result,
		"call_id", in.CallID,
		"call_url", in.CallURL,
	)
	if h.events == nil || (in.ID == "" && in.Phone == "" && in.Result == "" && in.Name == "" && in.CallID == "" && in.CallURL == "") {
		response.OK(c)
		return
	}
	if err := h.events.ApplyCrmcallsPostback(c.Request.Context(), in); err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			h.log.Warn("crmcalls postback ignored", "error", err, "id", in.ID, "phone", in.Phone, "result", in.Result)
			response.OK(c)
			return
		}
		h.log.Error("crmcalls postback failed", "error", err, "id", in.ID, "phone", in.Phone)
		response.HandleError(c, h.log, err)
		return
	}
	h.log.Info("crmcalls postback applied", "id", in.ID, "phone", in.Phone, "result", in.Result)
	response.OK(c)
}

func crmcallsKeyOK(secret, got string) bool {
	secret = strings.TrimSpace(secret)
	got = strings.TrimSpace(got)
	if secret == "" || got == "" || len(secret) != len(got) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(secret), []byte(got)) == 1
}

func crmcallsLogBody(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return ""
	}
	const max = 8000
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func bindCrmcallsLink(c *gin.Context) (usecase.CrmcallsLinkInput, bool) {
	var req dto.CrmcallsLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid json")
		return usecase.CrmcallsLinkInput{}, false
	}
	return usecase.CrmcallsLinkInput{
		Name:           req.Name,
		Type:           req.Type,
		ProjectID:      strings.TrimSpace(req.ProjectID),
		ChildSourceIDs: req.ChildSourceIDs,
		RegionIDs:      req.RegionIDs,
	}, true
}

func parseCrmcallsPostback(raw []byte, query map[string][]string) entity.CrmcallsPostback {
	var in entity.CrmcallsPostback
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err == nil && payload != nil {
		in.ID = crmcallsFirstString(payload, "id", "contact.id", "contact_id", "data.id")
		in.Name = crmcallsFirstString(payload, "name", "contact.name", "fio", "data.name")
		in.Phone = crmcallsFirstString(payload, "phone", "contact.phone", "data.phone")
		in.Result = crmcallsFirstString(payload, "result", "result_name", "call.result", "data.result")
		in.CallID = crmcallsFirstString(payload, "call.id", "call_id", "data.call_id", "data.call.id")
		in.CallURL = crmcallsFirstString(payload,
			"call.record_url", "call.record", "record_url", "recording_url", "call.recording_url", "data.record_url",
		)
		if at := crmcallsFirstString(payload, "occurred_at", "call.ended_at", "call.connected_at", "data.occurred_at"); at != "" {
			in.CallLastAt = parseCrmcallsTime(at)
		}
	} else if vals, err := url.ParseQuery(string(raw)); err == nil {
		in.ID = strings.TrimSpace(vals.Get("id"))
		in.Name = strings.TrimSpace(vals.Get("name"))
		in.Phone = strings.TrimSpace(vals.Get("phone"))
		in.Result = strings.TrimSpace(vals.Get("result"))
		in.CallID = strings.TrimSpace(firstNonEmpty(vals.Get("call.id"), vals.Get("call_id")))
		in.CallURL = strings.TrimSpace(firstNonEmpty(vals.Get("call.record_url"), vals.Get("record_url"), vals.Get("recording_url")))
		if at := strings.TrimSpace(firstNonEmpty(vals.Get("occurred_at"), vals.Get("call.ended_at"))); at != "" {
			in.CallLastAt = parseCrmcallsTime(at)
		}
	}
	if in.ID == "" {
		in.ID = queryFirst(query, "id")
	}
	if in.Name == "" {
		in.Name = queryFirst(query, "name")
	}
	if in.Phone == "" {
		in.Phone = queryFirst(query, "phone")
	}
	if in.Result == "" {
		in.Result = queryFirst(query, "result")
	}
	if in.CallID == "" {
		in.CallID = queryFirst(query, "call_id")
	}
	if in.CallURL == "" {
		in.CallURL = firstNonEmpty(queryFirst(query, "record_url"), queryFirst(query, "call.record_url"))
	}
	return in
}

func parseCrmcallsTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "02.01.2006 15:04:05"} {
		if t, err := time.ParseInLocation(layout, raw, clock.Location); err == nil {
			return &t
		}
		if t, err := time.Parse(layout, raw); err == nil {
			return &t
		}
	}
	return nil
}

func crmcallsFirstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := crmcallsLookupPath(m, key); ok {
			switch t := v.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					return s
				}
			case float64:
				return strconv.FormatInt(int64(t), 10)
			case json.Number:
				return t.String()
			}
		}
	}
	return ""
}

func crmcallsLookupPath(m map[string]any, path string) (any, bool) {
	if v, ok := m[path]; ok {
		return v, true
	}
	cur := any(m)
	for _, part := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func queryFirst(q map[string][]string, key string) string {
	vals := q[key]
	if len(vals) == 0 {
		return ""
	}
	return strings.TrimSpace(vals[0])
}
