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


// List
// @Summary      Партнёры
// @Tags         partners
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=dto.PartnerList}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/partners [get]
func (h *PartnersHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// Create
// @Summary      Создать партнёра
// @Tags         partners
// @Accept       json
// @Produce      json
// @Param        body body dto.PartnerRequest true "Партнёр"
// @Success      201 {object} response.SuccessEnvelope{data=entity.Partner}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/partners [post]
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
	response.JSON(c, http.StatusCreated, item)
}

// Update
// @Summary      Обновить партнёра
// @Tags         partners
// @Accept       json
// @Produce      json
// @Param        id   path int true "ID партнёра"
// @Param        body body dto.PartnerRequest true "Партнёр"
// @Success      200 {object} response.SuccessEnvelope{data=entity.Partner}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/partners/{id} [put]
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
	response.Success(c, item)
}

// Refresh
// @Summary      Проверить webhook всех партнёров
// @Tags         partners
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=dto.PartnerCheckList}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/partners/refresh [post]
func (h *PartnersHandler) Refresh(c *gin.Context) {
	items, err := h.svc.Refresh(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// SendTest
// @Summary      Тестовая отправка партнёру
// @Tags         partners
// @Produce      json
// @Param        id path int true "ID партнёра"
// @Success      200 {object} response.SuccessEnvelope{data=entity.PartnerCheck}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/partners/{id}/test [post]
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
	response.Success(c, item)
}

func bindPartner(c *gin.Context, create bool) (usecase.PartnerInput, bool) {
	var req dto.PartnerRequest
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
		Body:          req.Body,
		ContentType:   req.ContentType,
	}, true
}
