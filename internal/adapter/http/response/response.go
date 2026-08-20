package response

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/domain"
)

type ErrorBody struct {
	Error string `json:"error"`
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorBody{Error: message})
}

func HandleError(c *gin.Context, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		log.Warn("invalid input", "error", err, "path", c.Request.URL.Path)
		c.JSON(http.StatusBadRequest, ErrorBody{Error: err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		log.Warn("not found", "error", err, "path", c.Request.URL.Path)
		c.JSON(http.StatusNotFound, ErrorBody{Error: err.Error()})
	default:
		log.Error("internal error", "error", err, "path", c.Request.URL.Path)
		c.JSON(http.StatusInternalServerError, ErrorBody{Error: "internal server error"})
	}
}
