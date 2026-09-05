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

type skorozvonLinkBody struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	ProjectID      int64  `json:"project_id"`
	ChildSourceIDs []int  `json:"child_source_ids"`
	RegionIDs      []int  `json:"region_ids"`
}

func (h *SkorozvonLinksHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	if items == nil {
		items = []entity.SkorozvonLink{}
	}
	c.JSON(http.StatusOK, gin.H{"count": len(items), "items": items})
}

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
	c.JSON(http.StatusCreated, item)
}

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
	c.JSON(http.StatusOK, item)
}

func bindSkorozvonLink(c *gin.Context) (usecase.SkorozvonLinkInput, bool) {
	var req skorozvonLinkBody
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
