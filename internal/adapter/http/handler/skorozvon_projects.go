package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type SkorozvonProjectAPI interface {
	ListPage(ctx context.Context, offset int, name string) ([]entity.SkorozvonProject, int, error)
	Sync(ctx context.Context) ([]entity.SkorozvonProject, error)
	Refresh(ctx context.Context, uuids []string) ([]entity.SkorozvonProject, error)
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
// @Param        offset query int    false "Смещение" default(0)
// @Param        name   query string false "Поиск по названию"
// @Success      200 {object} response.SuccessEnvelope{data=dto.SkorozvonProjectList}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/skorozvon/projects/list [get]
func (h *SkorozvonProjectsHandler) List(c *gin.Context) {
	offset := 0
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			response.Error(c, http.StatusBadRequest, "offset must be a non-negative int")
			return
		}
		offset = n
	}
	items, total, err := h.svc.ListPage(c.Request.Context(), offset, strings.TrimSpace(c.Query("name")))
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	if items == nil {
		items = []entity.SkorozvonProject{}
	}
	response.Success(c, dto.SkorozvonProjectList{
		Count:  len(items),
		Total:  total,
		Limit:  usecase.SkorozvonProjectListLimit,
		Offset: offset,
		Items:  items,
	})
}

// Sync
// @Summary      Синхронизировать проекты Скорозвона
// @Tags         skorozvon
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=dto.SkorozvonProjectList}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/skorozvon/projects/sync [post]
func (h *SkorozvonProjectsHandler) Sync(c *gin.Context) {
	items, err := h.svc.Sync(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// Refresh
// @Summary      Обновить проекты из Скорозвона
// @Tags         skorozvon
// @Accept       json
// @Produce      json
// @Param        body body dto.SkorozvonProjectRefreshRequest false "UUID проектов; пусто — все"
// @Success      200 {object} response.SuccessEnvelope{data=dto.SkorozvonProjectList}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/skorozvon/projects/refresh [post]
func (h *SkorozvonProjectsHandler) Refresh(c *gin.Context) {
	var req dto.SkorozvonProjectRefreshRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, "invalid json")
			return
		}
	}
	items, err := h.svc.Refresh(c.Request.Context(), req.UUIDs)
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
