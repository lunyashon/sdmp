package crmdocs

import (
	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
)

// swaggerCRMLeadAdd — общая информация отдельной спеки.
//
// @title          SDMP crm.lead.add
// @version        0.1
// @description    Приём лида в формате Битрикс. JSON, form-urlencoded, multipart и query. user_id в пути должен быть 1. token — токен источника из настроек. PHONE обязателен. SOURCE_ID, если передан, должен совпасть с bitrix_code источника. Остальные fields сохраняются в сырьё; worker читает оператор, регион и тег.
// @BasePath       /
func swaggerCRMLeadAdd() {}

// Add документирует параметры приёма. Реальный хендлер — handler.CRMLeadHandler.Add.
//
// @Summary      crm.lead.add
// @Description  user_id=1. token — токен источника. PHONE обязателен: строка "79001234567" или массив {VALUE,VALUE_TYPE}. SOURCE_ID должен совпасть с bitrix_code, если оба заданы. UF_CRM_1752577998838 — web_id (идемпотентность). UF_CRM_1734523005552 — оператор. UF_CRM_1747216350206 — регион. UF_CRM_1766149282 — тег. UF_CRM_1699340770806 — номер проекта. TITLE и NAME опциональны. Те же ключи можно передать query/form: fields[PHONE][0][VALUE]=79001234567
// @Tags         crm.lead.add
// @Accept       json
// @Accept       x-www-form-urlencoded
// @Accept       mpfd
// @Produce      json
// @Param        user_id path string true "Всегда 1"
// @Param        token   path string true "Токен источника"
// @Param        body    body dto.CRMLeadAddRequest false "JSON. Либо form/query с теми же ключами"
// @Success      200 {object} response.BitrixOK
// @Failure      400 {object} response.BitrixErrorBody
// @Failure      401 {object} response.BitrixErrorBody
// @Failure      403 {object} response.BitrixErrorBody
// @Failure      500 {object} response.BitrixErrorBody
// @Router       /rest/{user_id}/{token}/crm.lead.add.json [post]
// @Router       /rest/{user_id}/{token}/crm.lead.add.json [get]
// @Router       /rest/{user_id}/{token}/crm.lead.add [post]
// @Router       /rest/{user_id}/{token}/crm.lead.add [get]
func Add(req dto.CRMLeadAddRequest) response.BitrixOK { return response.BitrixOK{} }
