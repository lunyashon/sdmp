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

func (h *SourcesHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, sourcesResponse(items))
}

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

	c.JSON(http.StatusOK, dto.CreateTokenResponse{
		SourceID: src.ID,
		Token:    token,
	})
}

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

	c.JSON(http.StatusCreated, dto.NewSourceDTO(src))
}

func (h *SourcesHandler) ListByToken(c *gin.Context) {
	items, err := h.svc.ListByToken(c.Request.Context(), c.Param("token"))
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, sourcesResponse(items))
}

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
	c.JSON(http.StatusOK, dto.NewSourceDTO(src))
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
