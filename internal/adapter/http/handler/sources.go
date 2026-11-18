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

type SourceAPI interface {
	List(ctx context.Context) ([]entity.Source, error)
	CreateToken(ctx context.Context, sourceID int) (entity.Source, string, error)
	ListByToken(ctx context.Context, token string) ([]entity.Source, error)
	Create(ctx context.Context, in usecase.CreateSourceInput) (entity.Source, error)
	CreateChild(ctx context.Context, in usecase.CreateChildInput) (entity.Source, error)
	Delete(ctx context.Context, sourceID int) error
	DeleteToken(ctx context.Context, sourceID int) error
	SetActive(ctx context.Context, sourceID int, active bool) (entity.Source, error)
}

type SourcesHandler struct {
	svc SourceAPI
	log *slog.Logger
}

func NewSourcesHandler(svc SourceAPI, log *slog.Logger) *SourcesHandler {
	if log == nil {
		log = slog.Default()
	}
	return &SourcesHandler{svc: svc, log: log}
}

// List
// @Summary      Источники
// @Tags         sources
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=dto.SourcesResponse}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/sources [get]
func (h *SourcesHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, sourcesResponse(items))
}

// Create
// @Summary      Создать родительский источник
// @Tags         sources
// @Accept       json
// @Produce      json
// @Param        body body dto.CreateSourceRequest true "Источник"
// @Success      201 {object} response.SuccessEnvelope{data=dto.SourceDTO}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/sources [post]
func (h *SourcesHandler) Create(c *gin.Context) {
	var req dto.CreateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid json")
		return
	}

	src, err := h.svc.Create(c.Request.Context(), usecase.CreateSourceInput{
		Name:       req.Name,
		BitrixCode: req.BitrixCode,
		ColdLabel:  req.ColdLabel,
		BaseLabel:  req.BaseLabel,
	})
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}

	response.JSON(c, http.StatusCreated, dto.NewSourceDTO(src))
}

// CreateToken
// @Summary      Выпустить токен источника
// @Tags         sources
// @Produce      json
// @Param        id path int true "ID корневого источника"
// @Success      200 {object} response.SuccessEnvelope{data=dto.CreateTokenResponse}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/sources/{id}/token [post]
func (h *SourcesHandler) CreateToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "source id is required")
		return
	}

	src, token, err := h.svc.CreateToken(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}

	response.Success(c, dto.CreateTokenResponse{
		SourceID: src.ID,
		Token:    token,
	})
}

// DeleteToken
// @Summary      Отозвать токен источника
// @Tags         sources
// @Param        id path int true "ID источника"
// @Success      204 {string} string
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/sources/{id}/token [delete]
func (h *SourcesHandler) DeleteToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "source id is required")
		return
	}
	if err := h.svc.DeleteToken(c.Request.Context(), id); err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// CreateChild
// @Summary      Создать подисточник
// @Tags         sources
// @Accept       json
// @Produce      json
// @Param        id   path int true "ID родителя"
// @Param        body body dto.CreateChildSourceRequest true "Подисточник"
// @Success      201 {object} response.SuccessEnvelope{data=dto.SourceDTO}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/sources/{id}/children [post]
func (h *SourcesHandler) CreateChild(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "source id is required")
		return
	}

	var req dto.CreateChildSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid json")
		return
	}

	src, err := h.svc.CreateChild(c.Request.Context(), usecase.CreateChildInput{
		ParentID:   id,
		Name:       req.Name,
		BitrixCode: req.BitrixCode,
		ColdLabel:  req.ColdLabel,
		BaseLabel:  req.BaseLabel,
	})
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}

	response.JSON(c, http.StatusCreated, dto.NewSourceDTO(src))
}

// ListByToken
// @Summary      Источники по токену
// @Tags         sources
// @Produce      json
// @Param        token path string true "Токен"
// @Success      200 {object} response.SuccessEnvelope{data=dto.SourcesResponse}
// @Failure      401 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/sources/by-token/{token} [get]
func (h *SourcesHandler) ListByToken(c *gin.Context) {
	items, err := h.svc.ListByToken(c.Request.Context(), c.Param("token"))
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, sourcesResponse(items))
}

// Delete
// @Summary      Удалить источник
// @Tags         sources
// @Param        id path int true "ID источника"
// @Success      204 {string} string
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/sources/{id} [delete]
func (h *SourcesHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "source id is required")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SetActive
// @Summary      Включить или выключить источник
// @Tags         sources
// @Accept       json
// @Produce      json
// @Param        id   path int true "ID источника"
// @Param        body body dto.UpdateSourceActiveRequest true "is_active"
// @Success      200 {object} response.SuccessEnvelope{data=dto.SourceDTO}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/sources/{id} [patch]
func (h *SourcesHandler) SetActive(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "source id is required")
		return
	}

	var req dto.UpdateSourceActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.IsActive == nil {
		response.Error(c, http.StatusBadRequest, "is_active is required")
		return
	}

	src, err := h.svc.SetActive(c.Request.Context(), id, *req.IsActive)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, dto.NewSourceDTO(src))
}

func sourcesResponse(items []entity.Source) dto.SourcesResponse {
	out := make([]dto.SourceDTO, 0, len(items))
	for _, src := range items {
		out = append(out, dto.NewSourceDTO(src))
	}
	return dto.SourcesResponse{
		Count:   len(out),
		Sources: out,
	}
}
