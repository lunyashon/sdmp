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
	ApplyCallResult(ctx context.Context, skorozvonLeadID string, in entity.LeadCallResult, snap entity.LeadFromSkorozvon) error
	ApplyFormResponse(ctx context.Context, skorozvonLeadID string, in entity.LeadFormResponse, snap entity.LeadFromSkorozvon) error
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

type skorozvonLead struct {
	ID           json.RawMessage `json:"id"`
	Name         string          `json:"name"`
	City         string          `json:"city"`
	Phones       json.RawMessage `json:"phones"`
	LeadPhones   json.RawMessage `json:"lead_phones"`
	Emails       json.RawMessage `json:"emails"`
	Tags         json.RawMessage `json:"tags"`
	Comment      string          `json:"comment"`
	CreatedAt    string          `json:"created_at"`
	CustomFields json.RawMessage `json:"custom_fields"`
}

type skorozvonEventEnvelope struct {
	Type    string        `json:"type"`
	Lead    skorozvonLead `json:"lead"`
	Contact skorozvonLead `json:"contact"`
	Call    struct {
		ID            json.RawMessage `json:"id"`
		RecordingURL  string          `json:"recording_url"`
		EndedAt       string          `json:"ended_at"`
		CallProjectID json.RawMessage `json:"call_project_id"`
		ResultID      json.RawMessage `json:"result_id"`
		Phone         string          `json:"phone"`
		Region        string          `json:"region"`
	} `json:"call"`
	CallResult struct {
		ResultID   json.RawMessage `json:"result_id"`
		ResultName string          `json:"result_name"`
		Comment    string          `json:"comment"`
	} `json:"call_result"`
	FormResponse struct {
		UserID     json.RawMessage       `json:"user_id"`
		FormID     json.RawMessage       `json:"form_id"`
		ScenarioID json.RawMessage       `json:"scenario_id"`
		FormName   string                `json:"form_name"`
		UpdatedAt  string                `json:"updated_at"`
		Answers    []skorozvonFormAnswer `json:"answers"`
	} `json:"form_response"`
}

type skorozvonFormAnswer struct {
	QuestionID   json.RawMessage `json:"question_id"`
	QuestionName string          `json:"question_name"`
	AnswerID     json.RawMessage `json:"answer_id"`
	AnswerValues []string        `json:"answer_values"`
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
		response.Error(c, http.StatusBadRequest, "cannot read body")
		return
	}

	if h.svc == nil || len(bytes.TrimSpace(raw)) == 0 {
		response.OK(c)
		return
	}

	var ev skorozvonEventEnvelope
	if err := json.Unmarshal(raw, &ev); err != nil {
		response.OK(c)
		return
	}

	switch strings.TrimSpace(ev.Type) {
	case "call_result":
		h.applyCallResult(c, ev)
	case "form_response":
		h.applyFormResponse(c, ev)
	default:
		response.OK(c)
	}
}

func (h *SkorozvonEventsHandler) applyCallResult(c *gin.Context, ev skorozvonEventEnvelope) {
	leadID := jsonScalar(ev.Lead.ID)
	in := entity.LeadCallResult{
		CallID:         jsonScalar(ev.Call.ID),
		CallURL:        strings.TrimSpace(ev.Call.RecordingURL),
		CallLastAt:     parseSkorozvonTime(ev.Call.EndedAt),
		CallResult:     parseInt64ID(jsonScalar(ev.CallResult.ResultID), jsonScalar(ev.Call.ResultID)),
		CallResultName: strings.TrimSpace(ev.CallResult.ResultName),
		ProjectNum:     jsonScalar(ev.Call.CallProjectID),
	}
	if err := h.svc.ApplyCallResult(c.Request.Context(), leadID, in, leadSnapshot(ev)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.Warn("skorozvon call_result lead not found", "skorozvon_lead_id", leadID)
			response.OK(c)
			return
		}
		response.HandleError(c, h.log, err)
		return
	}
	response.OK(c)
}

func (h *SkorozvonEventsHandler) applyFormResponse(c *gin.Context, ev skorozvonEventEnvelope) {
	leadID := jsonScalar(ev.Lead.ID)
	answers := make([]entity.SkorozvonAnketaAnswer, 0, len(ev.FormResponse.Answers))
	for _, a := range ev.FormResponse.Answers {
		answers = append(answers, entity.SkorozvonAnketaAnswer{
			QuestionID:   parseInt64(jsonScalar(a.QuestionID)),
			QuestionName: strings.TrimSpace(a.QuestionName),
			AnswerID:     parseInt64(jsonScalar(a.AnswerID)),
			AnswerValues: a.AnswerValues,
		})
	}
	in := entity.LeadFormResponse{
		CallID:     jsonScalar(ev.Call.ID),
		UserID:     parseInt64Ptr(jsonScalar(ev.FormResponse.UserID)),
		FormID:     parseInt64Ptr(jsonScalar(ev.FormResponse.FormID)),
		ScenarioID: parseInt64Ptr(jsonScalar(ev.FormResponse.ScenarioID)),
		FormName:   strings.TrimSpace(ev.FormResponse.FormName),
		UpdatedAt:  parseSkorozvonTime(ev.FormResponse.UpdatedAt),
		Answers:    answers,
	}
	if err := h.svc.ApplyFormResponse(c.Request.Context(), leadID, in, leadSnapshot(ev)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.Warn("skorozvon form_response lead not found", "skorozvon_lead_id", leadID)
			response.OK(c)
			return
		}
		response.HandleError(c, h.log, err)
		return
	}
	response.OK(c)
}

func leadSnapshot(ev skorozvonEventEnvelope) entity.LeadFromSkorozvon {
	phone := firstPhone(ev.Lead.Phones)
	if phone == "" {
		phone = firstPhone(ev.Lead.LeadPhones)
	}
	if phone == "" {
		phone = firstPhone(ev.Contact.Phones)
	}
	if phone == "" {
		phone = firstPhone(ev.Contact.LeadPhones)
	}
	if phone == "" {
		phone = strings.TrimSpace(ev.Call.Phone)
	}
	created := time.Time{}
	if t := parseSkorozvonTime(firstNonEmpty(ev.Lead.CreatedAt, ev.Contact.CreatedAt)); t != nil {
		created = *t
	}
	return entity.LeadFromSkorozvon{
		Phone:        phone,
		City:         firstNonEmpty(ev.Lead.City, ev.Contact.City),
		Region:       strings.TrimSpace(ev.Call.Region),
		Tag:          firstNonEmpty(firstString(ev.Lead.Tags), firstString(ev.Contact.Tags)),
		LegacyRaw:    skorozvonLegacyJSON(ev),
		CreatedAt:    created,
		CustomFields: mergeCustomFields(ev.Lead.CustomFields, ev.Contact.CustomFields),
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func mergeCustomFields(blobs ...json.RawMessage) []entity.SkorozvonCustomValue {
	byID := map[int64]entity.SkorozvonCustomValue{}
	order := make([]int64, 0)
	for _, blob := range blobs {
		blob = bytes.TrimSpace(blob)
		if len(blob) == 0 || string(blob) == "null" {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(blob, &obj); err != nil {
			continue
		}
		for key, raw := range obj {
			id := parseCustomFieldID(key)
			if id == 0 {
				continue
			}
			if _, exists := byID[id]; !exists {
				order = append(order, id)
			}
			byID[id] = entity.SkorozvonCustomValue{FieldID: id, Raw: append([]byte(nil), raw...)}
		}
	}
	out := make([]entity.SkorozvonCustomValue, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

func parseCustomFieldID(key string) int64 {
	key = strings.TrimSpace(key)
	if len(key) < 7 || !strings.EqualFold(key[:6], "FIELD_") {
		return 0
	}
	n, err := strconv.ParseInt(key[6:], 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func skorozvonLegacyJSON(ev skorozvonEventEnvelope) string {
	extra := map[string]any{}
	if v := firstNonEmpty(ev.Lead.Name, ev.Contact.Name); v != "" {
		extra["name"] = v
	}
	if v := firstNonEmpty(ev.Lead.Comment, ev.Contact.Comment); v != "" {
		extra["comment"] = v
	}
	if emails := parseStringList(ev.Lead.Emails); len(emails) > 0 {
		extra["emails"] = emails
	} else if emails := parseStringList(ev.Contact.Emails); len(emails) > 0 {
		extra["emails"] = emails
	}
	if tags := parseStringList(ev.Lead.Tags); len(tags) > 0 {
		extra["tags"] = tags
	} else if tags := parseStringList(ev.Contact.Tags); len(tags) > 0 {
		extra["tags"] = tags
	}
	if len(extra) == 0 {
		return ""
	}
	raw, err := json.Marshal(map[string]any{"skorozvon": extra})
	if err != nil {
		return ""
	}
	return string(raw)
}

func firstPhone(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if s := phoneFromObject(raw); s != "" {
		return s
	}
	for _, s := range parseStringList(raw) {
		if s != "" {
			return s
		}
	}
	if len(raw) == 0 || string(raw) == "null" || raw[0] != '[' {
		return jsonScalar(raw)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	for _, item := range items {
		item = bytes.TrimSpace(item)
		if len(item) == 0 {
			continue
		}
		if item[0] == '{' {
			if s := phoneFromObject(item); s != "" {
				return s
			}
			continue
		}
		if s := jsonScalar(item); s != "" {
			return s
		}
	}
	return ""
}

func phoneFromObject(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"phone", "number", "value"} {
		if v, ok := obj[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstString(raw json.RawMessage) string {
	if items := parseStringList(raw); len(items) > 0 {
		return items[0]
	}
	return jsonScalar(raw)
}

func parseStringList(raw json.RawMessage) []string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if raw[0] == '"' {
		if s := jsonScalar(raw); s != "" {
			return []string{s}
		}
		return nil
	}
	if raw[0] != '[' {
		if s := jsonScalar(raw); s != "" {
			return []string{s}
		}
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := jsonScalar(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func jsonScalar(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if raw[0] == '{' || raw[0] == '[' {
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

func parseInt64(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func parseInt64Ptr(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func parseInt64ID(vals ...string) *int64 {
	for _, raw := range vals {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		v := n
		return &v
	}
	return nil
}

func parseSkorozvonTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{usecase.LeadDateLayout, "2006-01-02 15:04:05", time.RFC3339Nano, time.RFC3339} {
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
