package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type GoogleSheetAPI interface {
	List(ctx context.Context) ([]entity.GoogleSheet, error)
	Accounts(ctx context.Context) ([]entity.GoogleAccount, error)
	Create(ctx context.Context, in usecase.GoogleSheetInput) (entity.GoogleSheet, error)
	Update(ctx context.Context, id int, in usecase.GoogleSheetInput) (entity.GoogleSheet, error)
	Delete(ctx context.Context, id int) error
	SendTest(ctx context.Context, id int) (entity.GoogleSheetCheck, error)
	Refresh(ctx context.Context) ([]entity.GoogleSheetCheck, error)
	Preview(ctx context.Context, in usecase.GoogleSheetInput) (entity.GoogleSheetPreview, error)
}

type GoogleSheetsHandler struct {
	svc GoogleSheetAPI
	log *slog.Logger
}

func NewGoogleSheetsHandler(svc GoogleSheetAPI, log *slog.Logger) *GoogleSheetsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &GoogleSheetsHandler{svc: svc, log: log}
}

// List
// @Summary      Инстансы Google Sheets
// @Tags         google
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=dto.GoogleSheetList}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/google/sheets [get]
func (h *GoogleSheetsHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// Accounts
// @Summary      Сохранённые сервисные аккаунты Google
// @Tags         google
// @Produce      json
// @Success      200 {object} response.SuccessEnvelope{data=dto.GoogleAccountList}
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/google/accounts [get]
func (h *GoogleSheetsHandler) Accounts(c *gin.Context) {
	items, err := h.svc.Accounts(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// Create
// @Summary      Создать инстанс Google Sheets
// @Tags         google
// @Accept       json
// @Produce      json
// @Param        body body dto.GoogleSheetRequest true "Инстанс"
// @Success      201 {object} response.SuccessEnvelope{data=entity.GoogleSheet}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/google/sheets [post]
func (h *GoogleSheetsHandler) Create(c *gin.Context) {
	in, ok := bindGoogleSheet(c)
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
// @Summary      Обновить инстанс Google Sheets
// @Tags         google
// @Accept       json
// @Produce      json
// @Param        id   path int true "ID инстанса"
// @Param        body body dto.GoogleSheetRequest true "Инстанс"
// @Success      200 {object} response.SuccessEnvelope{data=entity.GoogleSheet}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/google/sheets/{id} [put]
func (h *GoogleSheetsHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "sheet id is required")
		return
	}
	in, ok := bindGoogleSheet(c)
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

// Delete
// @Summary      Удалить инстанс Google Sheets
// @Tags         google
// @Param        id path int true "ID инстанса"
// @Success      204
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/google/sheets/{id} [delete]
func (h *GoogleSheetsHandler) Preview(c *gin.Context) {
	in, ok := bindGoogleSheet(c)
	if !ok {
		return
	}
	item, err := h.svc.Preview(c.Request.Context(), in)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, item)
}

func (h *GoogleSheetsHandler) SendTest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "sheet id is required")
		return
	}
	item, err := h.svc.SendTest(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, item)
}

func (h *GoogleSheetsHandler) Refresh(c *gin.Context) {
	items, err := h.svc.Refresh(c.Request.Context())
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

func (h *GoogleSheetsHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "sheet id is required")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func bindGoogleSheet(c *gin.Context) (usecase.GoogleSheetInput, bool) {
	var req dto.GoogleSheetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid json")
		return usecase.GoogleSheetInput{}, false
	}
	testMode := false
	if req.TestMode != nil {
		testMode = *req.TestMode
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	return usecase.GoogleSheetInput{
		Name:              req.Name,
		SpreadsheetID:     req.SpreadsheetID,
		SheetName:         req.SheetName,
		IsActive:          isActive,
		TestMode:          testMode,
		ZaryaResults:      req.ZaryaResults,
		RegionIDs:         req.RegionIDs,
		StopSourceIDs:     req.StopSourceIDs,
		Credentials:       json.RawMessage(req.Credentials),
		CredentialsFromID: req.CredentialsFromID,
		Columns:           req.Columns,
	}, true
}
