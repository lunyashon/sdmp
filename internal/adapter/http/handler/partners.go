package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type PartnerAPI interface {
	List(ctx context.Context) ([]entity.Partner, error)
	Create(ctx context.Context, in usecase.PartnerInput) (entity.Partner, error)
	Update(ctx context.Context, id int, in usecase.PartnerInput) (entity.Partner, error)
	Refresh(ctx context.Context) ([]entity.PartnerCheck, error)
	SendTest(ctx context.Context, id int) (entity.PartnerCheck, error)
}

type PartnersHandler struct {
	svc PartnerAPI
	log *slog.Logger
}

func NewPartnersHandler(svc PartnerAPI, log *slog.Logger) *PartnersHandler {
	if log == nil {
		log = slog.Default()
	}
	return &PartnersHandler{svc: svc, log: log}
}

type partnerBody struct {
	Name          string   `json:"name"`
	WebhookURL    string   `json:"webhook_url"`
	IsActive      *bool    `json:"is_active"`
	ZaryaResults  []string `json:"zarya_results"`
	RegionIDs     []int    `json:"region_ids"`
	StopSourceIDs []int    `json:"stop_source_ids"`
}

func (h *PartnersHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	if items == nil {
		items = []entity.Partner{}
	}
	c.JSON(http.StatusOK, gin.H{"count": len(items), "items": items})
}

func (h *PartnersHandler) Create(c *gin.Context) {
	in, ok := bindPartner(c, true)
	if !ok {
		return
	}
	item, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *PartnersHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "partner id is required")
		return
	}
	in, ok := bindPartner(c, false)
	if !ok {
		return
	}
	item, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *PartnersHandler) Refresh(c *gin.Context) {
	items, err := h.svc.Refresh(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	if items == nil {
		items = []entity.PartnerCheck{}
	}
	c.JSON(http.StatusOK, gin.H{"count": len(items), "items": items})
}

func (h *PartnersHandler) SendTest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "partner id is required")
		return
	}
	item, err := h.svc.SendTest(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func bindPartner(c *gin.Context, create bool) (usecase.PartnerInput, bool) {
	var req partnerBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid json")
		return usecase.PartnerInput{}, false
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	} else if !create {
		active = false
	}
	return usecase.PartnerInput{
		Name:          req.Name,
		WebhookURL:    req.WebhookURL,
		IsActive:      active,
		ZaryaResults:  req.ZaryaResults,
		RegionIDs:     req.RegionIDs,
		StopSourceIDs: req.StopSourceIDs,
	}, true
}
