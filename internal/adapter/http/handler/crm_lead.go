package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/bitrix"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/usecase"
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
		response.BitrixError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	mapped := bitrix.MapLead(payload)
	id, err := h.svc.Receive(c.Request.Context(), usecase.ReceiveLeadInput{
		UserID:     c.Param("user_id"),
		Token:      c.Param("token"),
		Phone:      mapped.Phone,
		SourceCode: mapped.SourceID,
		WebID:      mapped.WebID,
		Payload:    payload,
	})
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.BitrixResult(c, start, id)
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
