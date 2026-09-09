package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type SplitAPI interface {
	List(ctx context.Context) ([]entity.SourceSplit, error)
	Get(ctx context.Context, sourceID int) (entity.SourceSplit, error)
	Replace(ctx context.Context, in entity.SourceSplit) (entity.SourceSplit, error)
}

type SplitsHandler struct {
	svc SplitAPI
	log *slog.Logger
}

func NewSplitsHandler(svc SplitAPI, log *slog.Logger) *SplitsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &SplitsHandler{svc: svc, log: log}
}

// List
// @Summary      Все правила дробления
// @Tags         splits
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=dto.SourceSplitItems}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/splits [get]
func (h *SplitsHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Items(c, items)
}

// Get
// @Summary      Дробление источника
// @Tags         splits
// @Produce      json
// @Param        id path int true "ID источника"
// @Success      200 {object} response.SuccessEnvelope{data=entity.SourceSplit}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/sources/{id}/split [get]
func (h *SplitsHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "source id is required")
		return
	}
	item, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, item)
}

// Replace
// @Summary      Заменить дробление источника
// @Tags         splits
// @Accept       json
// @Produce      json
// @Param        id   path int true "ID источника"
// @Param        body body entity.SourceSplit true "Правила"
// @Success      200 {object} response.SuccessEnvelope{data=entity.SourceSplit}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/sources/{id}/split [put]
func (h *SplitsHandler) Replace(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "source id is required")
		return
	}
	var in entity.SourceSplit
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	in.SourceID = id
	item, err := h.svc.Replace(c.Request.Context(), in)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, item)
}
