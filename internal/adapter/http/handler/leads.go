package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type LeadFilter interface {
	Filter(ctx context.Context, q usecase.LeadFilterQuery) (usecase.LeadFilterResult, error)
}

type LeadsHandler struct {
	svc LeadFilter
	log *slog.Logger
}

func NewLeadsHandler(svc LeadFilter, log *slog.Logger) *LeadsHandler {
	return &LeadsHandler{svc: svc, log: log}
}

func (h *LeadsHandler) List(c *gin.Context) {
	var q dto.LeadsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, http.StatusBadRequest, "source_id, date_from and date_to are required")
		return
	}

	res, err := h.svc.Filter(c.Request.Context(), usecase.LeadFilterQuery{
		SourceID: q.SourceID,
		DateFrom: q.DateFrom,
		DateTo:   q.DateTo,
	})
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}

	leads := make([]dto.LeadDTO, 0, len(res.Leads))
	for _, lead := range res.Leads {
		leads = append(leads, dto.NewLeadDTO(lead))
	}

	c.JSON(http.StatusOK, dto.LeadsResponse{
		SourceID:      res.SourceID,
		DateFrom:      res.DateFrom,
		DateTo:        res.DateTo,
		MonthsLoaded:  res.MonthsLoaded,
		MonthsMissing: res.MonthsMissing,
		Count:         len(leads),
		Leads:         leads,
	})
}
