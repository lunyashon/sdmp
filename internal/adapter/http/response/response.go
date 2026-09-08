package response

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/domain"
)

type Envelope[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
}

type ErrorData struct {
	Error string `json:"error"`
}

type Empty []any

type HealthBody struct {
	Status string `json:"status"`
}

type ListBody[T any] struct {
	Count int `json:"count"`
	Items []T `json:"items"`
}

type ItemsBody[T any] struct {
	Items []T `json:"items"`
}

type LeadIDBody struct {
	LeadID int `json:"lead_id"`
}

func write[T any](c *gin.Context, status int, success bool, data T) {
	c.JSON(status, Envelope[T]{Success: success, Data: data})
}

func JSON[T any](c *gin.Context, status int, data T) {
	write(c, status, true, data)
}

func Success[T any](c *gin.Context, data T) {
	write(c, http.StatusOK, true, data)
}

func OK(c *gin.Context) {
	Success(c, make(Empty, 0))
}

func Health(c *gin.Context) {
	Success(c, HealthBody{Status: "ok"})
}

func Error(c *gin.Context, status int, message string) {
	write(c, status, false, ErrorData{Error: message})
}

func List[T any](c *gin.Context, items []T) {
	if items == nil {
		items = []T{}
	}
	Success(c, ListBody[T]{Count: len(items), Items: items})
}

func Items[T any](c *gin.Context, items []T) {
	if items == nil {
		items = []T{}
	}
	Success(c, ItemsBody[T]{Items: items})
}

func LeadID(c *gin.Context, id int) {
	Success(c, LeadIDBody{LeadID: id})
}

func HandleError(c *gin.Context, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		log.Warn("invalid input", "error", err, "path", c.Request.URL.Path)
		Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		log.Warn("unauthorized", "error", err, "path", c.Request.URL.Path)
		Error(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		log.Warn("forbidden", "error", err, "path", c.Request.URL.Path)
		Error(c, http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		log.Warn("not found", "error", err, "path", c.Request.URL.Path)
		Error(c, http.StatusNotFound, err.Error())
	default:
		log.Error("internal error", "error", err, "path", c.Request.URL.Path)
		Error(c, http.StatusInternalServerError, "internal server error")
	}
}
