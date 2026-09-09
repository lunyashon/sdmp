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

type LeadAPI interface {
	Filter(ctx context.Context, q usecase.LeadFilterQuery) (usecase.LeadFilterResult, error)
	ExportXLSX(ctx context.Context, q usecase.LeadFilterQuery, columns []string) (usecase.LeadExportFile, error)
	IncomingStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadSourceCount, error)
	EnrichmentStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadEnrichmentStat, error)
	ZaryaStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadZaryaStat, error)
	SkorozvonBitrixStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadSkorozvonBitrixStat, error)
	Get(ctx context.Context, id int64) (entity.LeadDetail, error)
	GetByUUID(ctx context.Context, uuid string) (entity.LeadDetail, error)
	Raw(ctx context.Context, id int64, uuid string) (usecase.LeadRaw, error)
	ResendPartner(ctx context.Context, id int64, uuid string, partnerID int) (usecase.PartnerResendResult, error)
}

type LeadsHandler struct {
	svc LeadAPI
	log *slog.Logger
}

func NewLeadsHandler(svc LeadAPI, log *slog.Logger) *LeadsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &LeadsHandler{svc: svc, log: log}
}

// List
// @Summary      Список лидов
// @Tags         leads
// @Produce      json
// @Param        date_from      query     string   true  "Дата с, dd.mm.yyyy HH:MM:SS"
// @Param        date_to        query     string   true  "Дата по"
// @Param        sources        query     []int    false "ID источников" collectionFormat(multi)
// @Param        child_sources  query     []int    false "ID подисточников" collectionFormat(multi)
// @Param        cold_label     query     []string false "Метка базы" collectionFormat(multi)
// @Param        region         query     []int    false "ID регионов" collectionFormat(multi)
// @Param        operator       query     []int    false "ID операторов" collectionFormat(multi)
// @Param        status         query     []int    false "ID стадий" collectionFormat(multi)
// @Param        zarya          query     []string false "Заря" collectionFormat(multi)
// @Param        phone          query     string   false "Телефон"
// @Param        city           query     string   false "Город"
// @Param        tag            query     string   false "Тег"
// @Param        call_result    query     int      false "ID результата звонка"
// @Param        via_us         query     bool     false "false=создалась из СК, true=обновилась"
// @Param        ids            query     []int    false "lead_row.id" collectionFormat(multi)
// @Param        limit          query     int      false "Лимит" default(10)
// @Param        offset         query     int      false "Смещение" default(0)
// @Success      200 {object} response.SuccessEnvelope{data=dto.LeadsResponse}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/leads [get]
func (h *LeadsHandler) List(c *gin.Context) {
	q, err := dto.ParseLeadsQuery(c.Request.URL.Query())
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	res, err := h.svc.Filter(c.Request.Context(), usecase.LeadFilterQuery{
		DateFrom:     q.DateFrom,
		DateTo:       q.DateTo,
		Sources:      q.Sources,
		ChildSources: q.ChildSources,
		ColdLabels:   q.ColdLabels,
		Regions:      q.Regions,
		Operators:    q.Operators,
		Statuses:     q.Statuses,
		Zarya:        q.Zarya,
		Phone:        q.Phone,
		City:         q.City,
		Tag:          q.Tag,
		CallResult:   q.CallResult,
		ViaUs:        q.ViaUs,
		IDs:          q.IDs,
		Limit:        q.Limit,
		Offset:       q.Offset,
	})
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}

	leads := make([]dto.LeadDTO, 0, len(res.Leads))
	for _, lead := range res.Leads {
		leads = append(leads, dto.NewLeadDTO(lead))
	}

	response.Success(c, dto.LeadsResponse{
		Count:  len(leads),
		Total:  res.Total,
		Limit:  res.Limit,
		Offset: res.Offset,
		Leads:  leads,
	})
}

// ExportXLSX
// @Summary      Выгрузка лидов в XLSX
// @Tags         leads
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param        date_from      query     string   true  "Дата с"
// @Param        date_to        query     string   true  "Дата по"
// @Param        sources        query     []int    false "ID источников" collectionFormat(multi)
// @Param        child_sources  query     []int    false "ID подисточников" collectionFormat(multi)
// @Param        cold_label     query     []string false "Метка базы" collectionFormat(multi)
// @Param        region         query     []int    false "ID регионов" collectionFormat(multi)
// @Param        operator       query     []int    false "ID операторов" collectionFormat(multi)
// @Param        status         query     []int    false "ID стадий" collectionFormat(multi)
// @Param        zarya          query     []string false "Заря" collectionFormat(multi)
// @Param        phone          query     string   false "Телефон"
// @Param        city           query     string   false "Город"
// @Param        tag            query     string   false "Тег"
// @Param        call_result    query     int      false "ID результата звонка"
// @Param        via_us         query     bool     false "false=создалась из СК, true=обновилась"
// @Param        ids            query     []int    false "Только эти lead_row.id" collectionFormat(multi)
// @Param        columns        query     []string false "Колонки файла" collectionFormat(multi)
// @Success      200 {file} file
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/leads/export.xlsx [get]
func (h *LeadsHandler) ExportXLSX(c *gin.Context) {
	q, err := dto.ParseLeadsQuery(c.Request.URL.Query())
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	file, err := h.svc.ExportXLSX(c.Request.Context(), usecase.LeadFilterQuery{
		DateFrom:     q.DateFrom,
		DateTo:       q.DateTo,
		Sources:      q.Sources,
		ChildSources: q.ChildSources,
		ColdLabels:   q.ColdLabels,
		Regions:      q.Regions,
		Operators:    q.Operators,
		Statuses:     q.Statuses,
		Zarya:        q.Zarya,
		Phone:        q.Phone,
		City:         q.City,
		Tag:          q.Tag,
		CallResult:   q.CallResult,
		ViaUs:        q.ViaUs,
		IDs:          q.IDs,
	}, q.Columns)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	name := strings.ReplaceAll(file.Filename, `"`, "")
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(http.StatusOK, file.ContentType, file.Body)
}

// IncomingStats
// @Summary      Входящий трафик по источникам
// @Tags         leads
// @Produce      json
// @Param        date_from query string true "Дата с"
// @Param        date_to   query string true "Дата по"
// @Success      200 {object} response.SuccessEnvelope{data=dto.LeadSourceCountList}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/leads/incoming-stats [get]
func (h *LeadsHandler) IncomingStats(c *gin.Context) {
	from, to, ok := statsDates(c)
	if !ok {
		return
	}
	items, err := h.svc.IncomingStats(c.Request.Context(), from, to)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// EnrichmentStats
// @Summary      Статистика обогащения (регион / оператор)
// @Tags         leads
// @Produce      json
// @Param        date_from query string true "Дата с"
// @Param        date_to   query string true "Дата по"
// @Success      200 {object} response.SuccessEnvelope{data=dto.LeadEnrichmentStatList}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/leads/enrichment-stats [get]
func (h *LeadsHandler) EnrichmentStats(c *gin.Context) {
	from, to, ok := statsDates(c)
	if !ok {
		return
	}
	items, err := h.svc.EnrichmentStats(c.Request.Context(), from, to)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// ZaryaStats
// @Summary      Статистика Зари
// @Tags         leads
// @Produce      json
// @Param        date_from query string true "Дата с"
// @Param        date_to   query string true "Дата по"
// @Success      200 {object} response.SuccessEnvelope{data=dto.LeadZaryaStatList}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/leads/zarya-stats [get]
func (h *LeadsHandler) ZaryaStats(c *gin.Context) {
	from, to, ok := statsDates(c)
	if !ok {
		return
	}
	items, err := h.svc.ZaryaStats(c.Request.Context(), from, to)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

// SkorozvonBitrixStats
// @Summary      Скорозвон → Битрикс
// @Tags         leads
// @Produce      json
// @Param        date_from query string true "Дата с"
// @Param        date_to   query string true "Дата по"
// @Success      200 {object} response.SuccessEnvelope{data=dto.LeadSkorozvonBitrixStatList}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/leads/skorozvon-bitrix-stats [get]
func (h *LeadsHandler) SkorozvonBitrixStats(c *gin.Context) {
	from, to, ok := statsDates(c)
	if !ok {
		return
	}
	items, err := h.svc.SkorozvonBitrixStats(c.Request.Context(), from, to)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.List(c, items)
}

func statsDates(c *gin.Context) (string, string, bool) {
	from := c.Query("date_from")
	to := c.Query("date_to")
	if from == "" || to == "" {
		response.Error(c, http.StatusBadRequest, "date_from and date_to are required")
		return "", "", false
	}
	return from, to, true
}

// Get
// @Summary      Карточка лида
// @Tags         leads
// @Produce      json
// @Param        id path string true "lead_row.id или uuid"
// @Success      200 {object} response.SuccessEnvelope{data=dto.LeadDetailResponse}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/leads/{id} [get]
func (h *LeadsHandler) Get(c *gin.Context) {
	raw := strings.TrimSpace(c.Param("id"))
	id, uuid, ok := parseLeadRef(raw)
	if !ok {
		response.Error(c, http.StatusBadRequest, "lead id is required")
		return
	}
	var (
		lead entity.LeadDetail
		err  error
	)
	if id > 0 {
		lead, err = h.svc.Get(c.Request.Context(), id)
	} else {
		lead, err = h.svc.GetByUUID(c.Request.Context(), uuid)
	}
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}

	response.Success(c, dto.LeadDetailResponse{Lead: dto.NewLeadDetailDTO(lead)})
}

// ResendPartner
// @Summary      Повторная отправка партнёру
// @Tags         leads
// @Produce      json
// @Param        id        path int true "lead_row.id или uuid"
// @Param        partnerId path int true "ID партнёра"
// @Success      200 {object} response.SuccessEnvelope{data=dto.PartnerResendResult}
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/leads/{id}/partners/{partnerId}/resend [post]
func (h *LeadsHandler) ResendPartner(c *gin.Context) {
	raw := strings.TrimSpace(c.Param("id"))
	id, uuid, ok := parseLeadRef(raw)
	if !ok {
		response.Error(c, http.StatusBadRequest, "lead id is required")
		return
	}
	partnerID, err := strconv.Atoi(c.Param("partnerId"))
	if err != nil || partnerID <= 0 {
		response.Error(c, http.StatusBadRequest, "partner id is required")
		return
	}
	got, err := h.svc.ResendPartner(c.Request.Context(), id, uuid, partnerID)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	response.Success(c, got)
}

// Raw
// @Summary      Сырой payload лида (S3)
// @Tags         leads
// @Produce      application/json
// @Param        id path string true "lead_row.id или uuid"
// @Success      200 {file} file
// @Failure      400 {object} response.ErrorEnvelope
// @Failure      404 {object} response.ErrorEnvelope
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/leads/{id}/raw [get]
func (h *LeadsHandler) Raw(c *gin.Context) {
	raw := strings.TrimSpace(c.Param("id"))
	id, uuid, ok := parseLeadRef(raw)
	if !ok {
		response.Error(c, http.StatusBadRequest, "lead id is required")
		return
	}
	file, err := h.svc.Raw(c.Request.Context(), id, uuid)
	if err != nil {
		response.HandleError(c, h.log, err)
		return
	}
	name := strings.ReplaceAll(file.Filename, `"`, "")
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(http.StatusOK, file.ContentType, file.Body)
}

func parseLeadRef(raw string) (id int64, uuid string, ok bool) {
	if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
		return id, "", true
	}
	if isLeadUUID(raw) {
		return 0, raw, true
	}
	return 0, "", false
}

func isLeadUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
