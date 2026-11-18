package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type BitrixMapAPI interface {
	Get(ctx context.Context) (usecase.BitrixLeadMapView, error)
	Save(ctx context.Context, in usecase.BitrixLeadMapSave) error
	Refresh(ctx context.Context) (usecase.BitrixLeadMapView, error)
}

type BitrixMapsHandler struct {
	svc BitrixMapAPI
	log *slog.Logger
}

func NewBitrixMapsHandler(svc BitrixMapAPI, log *slog.Logger) *BitrixMapsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &BitrixMapsHandler{svc: svc, log: log}
}

// Get
// @Summary      Связка полей CDP / Скорозвон → Битрикс
// @Tags         bitrix
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=usecase.BitrixLeadMapView}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/bitrix/lead-map [get]
func (h *BitrixMapsHandler) Get(c *gin.Context) {
	view, err := h.svc.Get(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, view)
}

// Save
// @Summary      Сохранить связку полей Битрикс
// @Tags         bitrix
// @Accept       json
// @Param        body body usecase.BitrixLeadMapSave true "Маппинг"
// @Success      200 {object} response.SuccessEnvelope{data=usecase.BitrixLeadMapView}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/bitrix/lead-map [put]
func (h *BitrixMapsHandler) Save(c *gin.Context) {
	var req usecase.BitrixLeadMapSave
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.svc.Save(c.Request.Context(), req); err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	view, err := h.svc.Get(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, view)
}

// Refresh
// @Summary      Обновить справочники полей и статусов из Битрикс
// @Tags         bitrix
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=usecase.BitrixLeadMapView}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/bitrix/lead-map/refresh [post]
func (h *BitrixMapsHandler) Refresh(c *gin.Context) {
	view, err := h.svc.Refresh(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, view)
}
