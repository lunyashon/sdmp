package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type SkorozvonProjectAPI interface {
	List(ctx context.Context) ([]entity.SkorozvonProject, error)
	Create(ctx context.Context, in usecase.SkorozvonProjectInput) (entity.SkorozvonProject, error)
	Refresh(ctx context.Context) ([]entity.SkorozvonProject, error)
	SendTest(ctx context.Context, uuid string) (int, error)
}

type SkorozvonProjectsHandler struct {
	svc SkorozvonProjectAPI
	log *slog.Logger
}

func NewSkorozvonProjectsHandler(svc SkorozvonProjectAPI, log *slog.Logger) *SkorozvonProjectsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &SkorozvonProjectsHandler{svc: svc, log: log}
}

type skorozvonProjectBody struct {
	Name      string `json:"name"`
	ProjectID int64  `json:"project_id"`
}

func (h *SkorozvonProjectsHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	if items == nil {
		items = []entity.SkorozvonProject{}
	}
	c.JSON(http.StatusOK, gin.H{"count": len(items), "items": items})
}

func (h *SkorozvonProjectsHandler) Create(c *gin.Context) {
	var req skorozvonProjectBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := h.svc.Create(c.Request.Context(), usecase.SkorozvonProjectInput{
		Name:      req.Name,
		ProjectID: req.ProjectID,
	})
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *SkorozvonProjectsHandler) Refresh(c *gin.Context) {
	items, err := h.svc.Refresh(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	if items == nil {
		items = []entity.SkorozvonProject{}
	}
	c.JSON(http.StatusOK, gin.H{"count": len(items), "items": items})
}

func (h *SkorozvonProjectsHandler) SendTest(c *gin.Context) {
	leadID, err := h.svc.SendTest(c.Request.Context(), c.Param("uuid"))
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"lead_id": leadID})
}
