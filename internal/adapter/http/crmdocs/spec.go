package crmdocs

import (
	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
)

// swaggerCRMLeadAdd — общая информация отдельной спеки.
//
// @title          SDMP crm.lead.add
// @version        0.1
// @description    Приём лида в формате Битрикс. JSON, form-urlencoded, multipart и query. user_id в пути должен быть 1. token — токен источника из настроек, источник берётся только из него. PHONE обязателен. SOURCE_ID не используется. SOURCE_DESCRIPTION — дополнительно об источнике, пишется в lead_row. Остальные fields сохраняются в сырьё; worker читает оператор, регион и тег.
// @BasePath       /
func swaggerCRMLeadAdd() {}

// Add документирует параметры приёма. Реальный хендлер — handler.CRMLeadHandler.Add.
//
// @Summary      crm.lead.add
// @Description  user_id=1. token — токен источника, источник берётся только из него. PHONE обязателен: строка "79001234567" или массив {VALUE,VALUE_TYPE}. SOURCE_ID игнорируется. SOURCE_DESCRIPTION — дополнительно об источнике. UF_CRM_1752577998838 — web_id (идемпотентность). UF_CRM_1734523005552 — оператор. UF_CRM_1747216350206 — регион. UF_CRM_1766149282 — тег. UF_CRM_1699340770806 — номер проекта. TITLE и NAME опциональны. Те же ключи можно передать query/form: fields[PHONE][0][VALUE]=79001234567
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
// @Failure      429 {object} response.BitrixErrorBody
// @Failure      500 {object} response.BitrixErrorBody
// @Router       /rest/{user_id}/{token}/crm.lead.add.json [post]
// @Router       /rest/{user_id}/{token}/crm.lead.add.json [get]
// @Router       /rest/{user_id}/{token}/crm.lead.add [post]
// @Router       /rest/{user_id}/{token}/crm.lead.add [get]
func Add(req dto.CRMLeadAddRequest) response.BitrixOK { return response.BitrixOK{} }

// Scope документирует список прав входящего вебхука. Реальный хендлер — handler.CRMLeadHandler.Scope.
//
// @Summary      scope
// @Description  Как в Битрикс: без параметров — права этого вебхука (crm, user, task, tasks_extended). full=true — полный список кодов. user_id в пути должен быть 1.
// @Tags         scope
// @Produce      json
// @Param        user_id path string true "Всегда 1"
// @Param        token   path string true "Токен источника"
// @Param        full    query bool false "true — вернуть все коды scope"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} response.BitrixErrorBody
// @Router       /rest/{user_id}/{token}/scope [get]
// @Router       /rest/{user_id}/{token}/scope [post]
// @Router       /rest/{user_id}/{token}/scope.json [get]
// @Router       /rest/{user_id}/{token}/scope.json [post]
func Scope() {}

// Profile документирует текущего пользователя входящего вебхука.
//
// @Summary      profile
// @Description  Пустышка Битрикс: текущий пользователь вебхука (ID=1, ADMIN=true).
// @Tags         bitrix.mimic
// @Produce      json
// @Param        user_id path string true "Всегда 1"
// @Param        token   path string true "Токен источника"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} response.BitrixErrorBody
// @Router       /rest/{user_id}/{token}/profile [get]
// @Router       /rest/{user_id}/{token}/profile [post]
// @Router       /rest/{user_id}/{token}/profile.json [get]
// @Router       /rest/{user_id}/{token}/profile.json [post]
func Profile() {}

// UserGet документирует список пользователей.
//
// @Summary      user.get
// @Description  Пустышка Битрикс: три пользователя (1 администратор, 2 менеджер, 3 оператор). filter[ID] сужает список.
// @Tags         bitrix.mimic
// @Produce      json
// @Param        user_id path string true "Всегда 1"
// @Param        token   path string true "Токен источника"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} response.BitrixErrorBody
// @Router       /rest/{user_id}/{token}/user.get [get]
// @Router       /rest/{user_id}/{token}/user.get [post]
// @Router       /rest/{user_id}/{token}/user.get.json [get]
// @Router       /rest/{user_id}/{token}/user.get.json [post]
func UserGet() {}

// StatusList документирует справочник статусов и источников.
//
// @Summary      crm.status.list
// @Description  Пустышка Битрикс. filter[ENTITY_ID]=SOURCE — источники (CALL, WEB, UC_BIGDATA…). filter[ENTITY_ID]=STATUS — стадии лида. Без фильтра — оба справочника.
// @Tags         bitrix.mimic
// @Produce      json
// @Param        user_id path string true "Всегда 1"
// @Param        token   path string true "Токен источника"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} response.BitrixErrorBody
// @Router       /rest/{user_id}/{token}/crm.status.list [get]
// @Router       /rest/{user_id}/{token}/crm.status.list [post]
// @Router       /rest/{user_id}/{token}/crm.status.list.json [get]
// @Router       /rest/{user_id}/{token}/crm.status.list.json [post]
func StatusList() {}

// TaskAdd документирует создание задачи.
//
// @Summary      tasks.task.add
// @Description  Пустышка Битрикс: возвращает фиктивный task.id, лид не создаёт задачу в системе.
// @Tags         bitrix.mimic
// @Accept       json
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        user_id path string true "Всегда 1"
// @Param        token   path string true "Токен источника"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} response.BitrixErrorBody
// @Router       /rest/{user_id}/{token}/tasks.task.add [post]
// @Router       /rest/{user_id}/{token}/tasks.task.add [get]
// @Router       /rest/{user_id}/{token}/tasks.task.add.json [post]
// @Router       /rest/{user_id}/{token}/tasks.task.add.json [get]
func TaskAdd() {}

// LeadFields документирует поля лида для мастера BigData-IT.
//
// @Summary      crm.lead.fields
// @Description  Поля из документации crm.lead.add: TITLE, NAME, PHONE, SOURCE_ID, SOURCE_DESCRIPTION и UF_CRM_* (web_id, оператор, регион, тег, номер проекта), плюс COMMENTS, EMAIL, ASSIGNED_BY_ID. Тот же каталог отдают crm.deal.fields и crm.contact.fields.
// @Tags         bitrix.mimic
// @Produce      json
// @Param        user_id path string true "Всегда 1"
// @Param        token   path string true "Токен источника"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} response.BitrixErrorBody
// @Router       /rest/{user_id}/{token}/crm.lead.fields [get]
// @Router       /rest/{user_id}/{token}/crm.lead.fields [post]
// @Router       /rest/{user_id}/{token}/crm.lead.fields.json [get]
// @Router       /rest/{user_id}/{token}/crm.lead.fields.json [post]
func LeadFields() {}

// UserfieldList документирует пользовательские поля лида.
//
// @Summary      crm.userfield.list
// @Description  Пустышка Битрикс: UF_CRM_* из документации crm.lead.add (web_id, оператор, регион, тег, номер проекта).
// @Tags         bitrix.mimic
// @Produce      json
// @Param        user_id path string true "Всегда 1"
// @Param        token   path string true "Токен источника"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} response.BitrixErrorBody
// @Router       /rest/{user_id}/{token}/crm.userfield.list [get]
// @Router       /rest/{user_id}/{token}/crm.userfield.list [post]
// @Router       /rest/{user_id}/{token}/crm.userfield.list.json [get]
// @Router       /rest/{user_id}/{token}/crm.userfield.list.json [post]
func UserfieldList() {}
