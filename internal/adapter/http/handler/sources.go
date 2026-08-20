package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type SourceLister interface {
	List(ctx context.Context) ([]entity.Source, error)
}

type SourcesHandler struct {
	svc SourceLister
	log *slog.Logger
}

func NewSourcesHandler(svc SourceLister, log *slog.Logger) *SourcesHandler {
	return &SourcesHandler{svc: svc, log: log}
}

func (h *SourcesHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}

	out := make([]dto.SourceDTO, 0, len(items))
	for _, src := range items {
		out = append(out, dto.NewSourceDTO(src))
	}

	c.JSON(http.StatusOK, dto.SourcesResponse{
		Count:   len(out),
		Sources: out,
	})
}
