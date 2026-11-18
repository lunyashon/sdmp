package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/lib/auth"
	"github.com/lunyashon/sdmp/internal/lib/config"
)

type AuthHandler struct {
	cfg *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var in loginRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, http.StatusBadRequest, "bad request")
		return
	}
	login := strings.TrimSpace(in.Login)
	password := in.Password
	if h.cfg == nil || !auth.CheckPassword(login, password, h.cfg.AuthLogin, h.cfg.AuthPassword) {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	token, err := auth.Issue(h.cfg.AuthPassword, time.Now())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "auth is not configured")
		return
	}
	response.Success(c, gin.H{"token": token})
}
