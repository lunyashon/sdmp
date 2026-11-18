package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/bitrix"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/usecase"
)

const bitrixWebhookUserID = "1"

var (
	// BigData-IT проверяет user + crm + task + tasks_extended на входящем вебхуке.
	bitrixGrantedScopes = []string{"crm", "task", "tasks_extended", "user"}
	bitrixFullScopes    = []string{
		"bizproc", "calendar", "catalog", "crm", "department", "disk",
		"entity", "im", "imopenlines", "landing", "lists", "log",
		"mail", "messageservice", "placement", "sale", "sonet",
		"task", "tasks_extended", "telephony", "user", "userconsent",
	}
)

type LeadReceiver interface {
	Receive(ctx context.Context, in usecase.ReceiveLeadInput) (int64, error)
}

type CRMLeadHandler struct {
	svc LeadReceiver
	log *slog.Logger
}

func NewCRMLeadHandler(svc LeadReceiver, log *slog.Logger) *CRMLeadHandler {
	if log == nil {
		log = slog.Default()
	}
	return &CRMLeadHandler{svc: svc, log: log}
}

func (h *CRMLeadHandler) Add(c *gin.Context) {
	start := time.Now()
	if h.svc == nil {
		response.BitrixError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error")
		return
	}
	payload, err := bitrix.ParseRequest(c.Request)
	if err != nil {
		h.log.Error("crm.lead.add parse request failed", "error", err.Error())
		response.BitrixError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	h.logBitrixBody(c, payload)
	mapped := bitrix.MapLead(payload)
	id, err := h.svc.Receive(c.Request.Context(), usecase.ReceiveLeadInput{
		UserID:            c.Param("user_id"),
		Token:             c.Param("token"),
		Phone:             mapped.Phone,
		SourceDescription: mapped.SourceDescription,
		WebID:             mapped.WebID,
		Payload:           payload,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.BitrixResult(c, start, id)
}

func (h *CRMLeadHandler) Scope(c *gin.Context) {
	start := time.Now()
	if !h.bitrixAuth(c) {
		return
	}
	scopes := bitrixGrantedScopes
	if bitrixWantFull(c.Request) {
		scopes = bitrixFullScopes
	}
	response.BitrixData(c, start, scopes)
}

func (h *CRMLeadHandler) bitrixAuth(c *gin.Context) bool {
	if strings.TrimSpace(c.Param("user_id")) != bitrixWebhookUserID || strings.TrimSpace(c.Param("token")) == "" {
		response.BitrixError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid request credentials")
		return false
	}
	return true
}

func bitrixWantFull(r *http.Request) bool {
	if bitrixTruthy(r.URL.Query().Get("full")) {
		return true
	}
	if strings.Contains(r.Header.Get("Content-Type"), "json") && r.Body != nil {
		var body struct {
			Full any `json:"full"`
		}
		if json.NewDecoder(r.Body).Decode(&body) == nil && bitrixTruthyAny(body.Full) {
			return true
		}
	}
	_ = r.ParseForm()
	return bitrixTruthy(r.Form.Get("full"))
}

func bitrixTruthyAny(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return bitrixTruthy(x)
	case float64:
		return x != 0
	default:
		return false
	}
}

func bitrixTruthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func (h *CRMLeadHandler) logBitrixBody(c *gin.Context, payload map[string]any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		h.log.Info("bitrix incoming", "path", c.Request.URL.Path, "payload_error", err.Error())
		return
	}
	h.log.Info("bitrix incoming", "path", c.Request.URL.Path, "body", string(raw))
}

func (h *CRMLeadHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		response.BitrixError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid request credentials")
	case errors.Is(err, domain.ErrForbidden):
		response.BitrixError(c, http.StatusForbidden, "ACCESS_DENIED", "Access denied")
	case errors.Is(err, domain.ErrInvalidInput):
		response.BitrixError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
	default:
		h.log.Error("crm.lead.add failed", "error", err, "path", c.Request.URL.Path)
		response.BitrixError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error")
	}
}
