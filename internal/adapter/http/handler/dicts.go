package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type DictAPI interface {
	ListRegions(ctx context.Context) ([]entity.NamedRef, error)
	ListStatuses(ctx context.Context) ([]entity.NamedRef, error)
	ListOperatorGroups(ctx context.Context) ([]entity.OperatorGroup, error)
	AddOperatorAlias(ctx context.Context, operatorID int, alias string) error
	RemoveOperatorAlias(ctx context.Context, operatorID int, alias string) error
}

type DictsHandler struct {
	svc DictAPI
	log *slog.Logger
}

func NewDictsHandler(svc DictAPI, log *slog.Logger) *DictsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &DictsHandler{svc: svc, log: log}
}

func (h *DictsHandler) Regions(c *gin.Context) {
	items, err := h.svc.ListRegions(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

func (h *DictsHandler) Statuses(c *gin.Context) {
	items, err := h.svc.ListStatuses(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

func (h *DictsHandler) Operators(c *gin.Context) {
	items, err := h.svc.ListOperatorGroups(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

func (h *DictsHandler) AddOperatorAlias(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "operator id is required")
		return
	}
	var req struct {
		Alias string `json:"alias"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Alias) == "" {
		response.Error(c, http.StatusBadRequest, "alias is required")
		return
	}
	if err := h.svc.AddOperatorAlias(c.Request.Context(), id, req.Alias); err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *DictsHandler) RemoveOperatorAlias(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "operator id is required")
		return
	}
	alias := strings.TrimSpace(c.Query("alias"))
	if alias == "" {
		response.Error(c, http.StatusBadRequest, "alias is required")
		return
	}
	if err := h.svc.RemoveOperatorAlias(c.Request.Context(), id, alias); err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}
