package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type LeadAPI interface {
	Filter(ctx context.Context, q usecase.LeadFilterQuery) (usecase.LeadFilterResult, error)
	IncomingStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadSourceCount, error)
	EnrichmentStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadEnrichmentStat, error)
	ZaryaStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadZaryaStat, error)
	Get(ctx context.Context, id int64) (entity.LeadDetail, error)
}

type LeadsHandler struct {
	svc LeadAPI
	log *slog.Logger
}

func NewLeadsHandler(svc LeadAPI, log *slog.Logger) *LeadsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &LeadsHandler{svc: svc, log: log}
}

func (h *LeadsHandler) List(c *gin.Context) {
	q, err := dto.ParseLeadsQuery(c.Request.URL.Query())
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	res, err := h.svc.Filter(c.Request.Context(), usecase.LeadFilterQuery{
		DateFrom:   q.DateFrom,
		DateTo:     q.DateTo,
		Sources:    q.Sources,
		ColdLabels: q.ColdLabels,
		Regions:    q.Regions,
		Operators:  q.Operators,
		Statuses:   q.Statuses,
		Zarya:      q.Zarya,
		Phone:      q.Phone,
		City:       q.City,
		Tag:        q.Tag,
		CallResult: q.CallResult,
		Limit:      q.Limit,
		Offset:     q.Offset,
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
		Count:  len(leads),
		Total:  res.Total,
		Limit:  res.Limit,
		Offset: res.Offset,
		Leads:  leads,
	})
}

func (h *LeadsHandler) IncomingStats(c *gin.Context) {
	from, to, ok := statsDates(c)
	if !ok {
		return
	}
	items, err := h.svc.IncomingStats(c.Request.Context(), from, to)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	if items == nil {
		items = []entity.LeadSourceCount{}
	}
	c.JSON(http.StatusOK, gin.H{"count": len(items), "items": items})
}

func (h *LeadsHandler) EnrichmentStats(c *gin.Context) {
	from, to, ok := statsDates(c)
	if !ok {
		return
	}
	items, err := h.svc.EnrichmentStats(c.Request.Context(), from, to)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	if items == nil {
		items = []entity.LeadEnrichmentStat{}
	}
	c.JSON(http.StatusOK, gin.H{"count": len(items), "items": items})
}

func (h *LeadsHandler) ZaryaStats(c *gin.Context) {
	from, to, ok := statsDates(c)
	if !ok {
		return
	}
	items, err := h.svc.ZaryaStats(c.Request.Context(), from, to)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	if items == nil {
		items = []entity.LeadZaryaStat{}
	}
	c.JSON(http.StatusOK, gin.H{"count": len(items), "items": items})
}

func statsDates(c *gin.Context) (string, string, bool) {
	from := c.Query("date_from")
	to := c.Query("date_to")
	if from == "" || to == "" {
		response.Error(c, http.StatusBadRequest, "date_from and date_to are required")
		return "", "", false
	}
	return from, to, true
}

func (h *LeadsHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "lead id is required")
		return
	}

	lead, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}

	c.JSON(http.StatusOK, dto.LeadDetailResponse{Lead: dto.NewLeadDetailDTO(lead)})
}
