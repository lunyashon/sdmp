package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
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

// List
// @Summary      Проекты Скорозвона
// @Tags         skorozvon
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=dto.SkorozvonProjectList}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/skorozvon/projects [get]
func (h *SkorozvonProjectsHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// Create
// @Summary      Добавить проект Скорозвона
// @Tags         skorozvon
// @Accept       json
// @Produce      json
// @Param        body body dto.SkorozvonProjectRequest true "Проект"
// @Success      201 {object} response.SuccessEnvelope{data=entity.SkorozvonProject}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/skorozvon/projects [post]
func (h *SkorozvonProjectsHandler) Create(c *gin.Context) {
	var req dto.SkorozvonProjectRequest
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
	response.JSON(c, http.StatusCreated, item)
}

// Refresh
// @Summary      Обновить проекты из Скорозвона
// @Tags         skorozvon
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=dto.SkorozvonProjectList}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/skorozvon/projects/refresh [post]
func (h *SkorozvonProjectsHandler) Refresh(c *gin.Context) {
	items, err := h.svc.Refresh(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// SendTest
// @Summary      Тестовая отправка в проект Скорозвона
// @Tags         skorozvon
// @Produce      json
// @Param        uuid path string true "UUID проекта"
// @Success      200 {object} response.SuccessEnvelope{data=response.LeadIDBody}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/skorozvon/projects/{uuid}/test [post]
func (h *SkorozvonProjectsHandler) SendTest(c *gin.Context) {
	leadID, err := h.svc.SendTest(c.Request.Context(), c.Param("uuid"))
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.LeadID(c, leadID)
}
