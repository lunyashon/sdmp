package handler

import (
	_ "embed"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
)

//go:embed crm_lead_add.docx
var crmLeadAddDocx []byte

const (
	crmLeadAddDownloadName = "crm-lead-add.docx"
	crmLeadAddMIME         = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

type DocsHandler struct {
	log *slog.Logger
}

func NewDocsHandler(log *slog.Logger) *DocsHandler {
	if log == nil {
		log = slog.Default()
	}
	return &DocsHandler{log: log}
}

// CRMLeadAddDocx
// @Summary      Документация crm.lead.add
// @Tags         docs
// @Produce      application/vnd.openxmlformats-officedocument.wordprocessingml.document
// @Success      200 {file} file
// @Failure      500 {object} response.ErrorEnvelope
// @Router       /rest/docs/crm.lead.add.docx [get]
func (h *DocsHandler) CRMLeadAddDocx(c *gin.Context) {
	if len(crmLeadAddDocx) < 4 || string(crmLeadAddDocx[:2]) != "PK" {
		h.log.Error("embedded crm.lead.add.docx is missing or invalid")
		response.Error(c, http.StatusInternalServerError, "documentation file not found")
		return
	}
	c.Header("Content-Type", crmLeadAddMIME)
	c.Header("Content-Disposition", `attachment; filename="`+crmLeadAddDownloadName+`"`)
	c.Header("Content-Length", strconv.Itoa(len(crmLeadAddDocx)))
	c.Data(http.StatusOK, crmLeadAddMIME, crmLeadAddDocx)
}
