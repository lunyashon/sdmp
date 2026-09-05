package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/usecase"
)

const skorozvonEventLogLimit = 32 * 1024

type SkorozvonEventAPI interface {
	ApplyCallResult(ctx context.Context, skorozvonLeadID string, in entity.LeadCallResult) error
}

type SkorozvonEventsHandler struct {
	svc SkorozvonEventAPI
	log *slog.Logger
}

func NewSkorozvonEventsHandler(svc SkorozvonEventAPI, log *slog.Logger) *SkorozvonEventsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &SkorozvonEventsHandler{svc: svc, log: log}
}

type skorozvonEventEnvelope struct {
	Type string `json:"type"`
	Lead struct {
		ID json.RawMessage `json:"id"`
	} `json:"lead"`
	Call struct {
		ID            json.RawMessage `json:"id"`
		RecordingURL  string          `json:"recording_url"`
		EndedAt       string          `json:"ended_at"`
		CallProjectID json.RawMessage `json:"call_project_id"`
		ResultID      json.RawMessage `json:"result_id"`
	} `json:"call"`
	CallResult struct {
		ResultID   json.RawMessage `json:"result_id"`
		ResultName string          `json:"result_name"`
		Comment    string          `json:"comment"`
	} `json:"call_result"`
}

func (h *SkorozvonEventsHandler) Receive(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.log.Info("skorozvon event read failed",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery,
			"error", err,
		)
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "cannot read body"})
		return
	}
	body, truncated := clipLogBody(raw, skorozvonEventLogLimit)
	h.log.Info("skorozvon event",
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"query", c.Request.URL.RawQuery,
		"content_type", c.GetHeader("Content-Type"),
		"content_length", len(raw),
		"truncated", truncated,
		"body", body,
	)

	if h.svc == nil || len(bytes.TrimSpace(raw)) == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	var ev skorozvonEventEnvelope
	if err := json.Unmarshal(raw, &ev); err != nil || strings.TrimSpace(ev.Type) != "call_result" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	leadID := jsonScalar(ev.Lead.ID)
	in := entity.LeadCallResult{
		CallID:     jsonScalar(ev.Call.ID),
		CallURL:    strings.TrimSpace(ev.Call.RecordingURL),
		CallLastAt: parseSkorozvonTime(ev.Call.EndedAt),
		CallResult: parseInt32ID(jsonScalar(ev.CallResult.ResultID), jsonScalar(ev.Call.ResultID)),
		ProjectNum: jsonScalar(ev.Call.CallProjectID),
	}
	if err := h.svc.ApplyCallResult(c.Request.Context(), leadID, in); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.Warn("skorozvon call_result lead not found", "skorozvon_lead_id", leadID)
			c.JSON(http.StatusOK, gin.H{"ok": true})
			return
		}
		response.HandleError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func jsonScalar(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return strings.TrimSpace(s)
		}
		return ""
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	return strings.TrimSpace(string(raw))
}

func parseInt32ID(vals ...string) *int32 {
	for _, raw := range vals {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			continue
		}
		v := int32(n)
		return &v
	}
	return nil
}

func parseSkorozvonTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{usecase.LeadDateLayout, "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, raw, clock.Location); err == nil {
			return &t
		}
	}
	return nil
}

func clipLogBody(raw []byte, limit int) (string, bool) {
	if len(raw) <= limit {
		return string(raw), false
	}
	cut := raw[:limit]
	if !utf8.Valid(cut) {
		for len(cut) > 0 && !utf8.Valid(cut) {
			cut = cut[:len(cut)-1]
		}
	}
	return string(cut) + "…", true
}
