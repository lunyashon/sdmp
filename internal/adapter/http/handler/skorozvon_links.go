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

type SkorozvonLinkAPI interface {
	List(ctx context.Context) ([]entity.SkorozvonLink, error)
	Create(ctx context.Context, in usecase.SkorozvonLinkInput) (entity.SkorozvonLink, error)
	Update(ctx context.Context, id int, in usecase.SkorozvonLinkInput) (entity.SkorozvonLink, error)
}

type SkorozvonLinksHandler struct {
	svc SkorozvonLinkAPI
	log *slog.Logger
}

func NewSkorozvonLinksHandler(svc SkorozvonLinkAPI, log *slog.Logger) *SkorozvonLinksHandler {
	if log == nil {
		log = slog.Default()
	}
	return &SkorozvonLinksHandler{svc: svc, log: log}
}


// List
// @Summary      Связки Скорозвона
// @Tags         skorozvon
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=dto.SkorozvonLinkList}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/skorozvon/links [get]
func (h *SkorozvonLinksHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// Create
// @Summary      Создать связку Скорозвона
// @Tags         skorozvon
// @Accept       json
// @Produce      json
// @Param        body body dto.SkorozvonLinkRequest true "Связка"
// @Success      201 {object} response.SuccessEnvelope{data=entity.SkorozvonLink}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/skorozvon/links [post]
func (h *SkorozvonLinksHandler) Create(c *gin.Context) {
	in, ok := bindSkorozvonLink(c)
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
// @Summary      Обновить связку Скорозвона
// @Tags         skorozvon
// @Accept       json
// @Produce      json
// @Param        id   path int true "ID связки"
// @Param        body body dto.SkorozvonLinkRequest true "Связка"
// @Success      200 {object} response.SuccessEnvelope{data=entity.SkorozvonLink}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/skorozvon/links/{id} [put]
func (h *SkorozvonLinksHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "link id is required")
		return
	}
	in, ok := bindSkorozvonLink(c)
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

func bindSkorozvonLink(c *gin.Context) (usecase.SkorozvonLinkInput, bool) {
	var req dto.SkorozvonLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid json")
		return usecase.SkorozvonLinkInput{}, false
	}
	return usecase.SkorozvonLinkInput{
		Name:           req.Name,
		Type:           req.Type,
		ProjectID:      req.ProjectID,
		ChildSourceIDs: req.ChildSourceIDs,
		RegionIDs:      req.RegionIDs,
	}, true
}
