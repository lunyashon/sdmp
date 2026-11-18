package handler

import (
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/bitrix"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
)

var (
	dummyTaskSeq   atomic.Int64
	dummyEntitySeq atomic.Int64
)

// Mimic — пустые методы входящего вебхука Битрикс для мастера sales.bigdata-it.ru.
func (h *CRMLeadHandler) Mimic(c *gin.Context) {
	start := time.Now()
	if !h.bitrixAuth(c) {
		return
	}
	method := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(c.Param("method")), ".json"))
	payload, err := bitrix.ParseRequest(c.Request)
	if err != nil {
		response.BitrixError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if strings.Contains(method, ".add") {
		h.logBitrixBody(c, payload)
	}

	switch method {
	case "profile":
		response.BitrixData(c, start, dummyBitrixProfile())
	case "user.current":
		response.BitrixData(c, start, dummyBitrixUser("1"))
	case "user.get", "user.search":
		users := filterDummyUsers(bitrixLookupID(payload))
		response.BitrixList(c, start, users, len(users))
	case "crm.status.list":
		items := dummyBitrixStatuses(bitrixLookupEntity(payload))
		response.BitrixList(c, start, items, len(items))
	case "crm.source.list":
		items := dummyBitrixStatuses("SOURCE")
		response.BitrixList(c, start, items, len(items))
	case "crm.lead.fields", "crm.deal.fields", "crm.contact.fields":
		response.BitrixData(c, start, dummyBitrixLeadFields())
	case "crm.userfield.list", "crm.lead.userfield.list", "crm.deal.userfield.list", "crm.contact.userfield.list":
		items := dummyBitrixUserfields()
		response.BitrixList(c, start, items, len(items))
	case "crm.category.list":
		response.BitrixData(c, start, dummyBitrixCategories())
	case "crm.contact.add", "crm.deal.add":
		response.BitrixData(c, start, dummyBitrixEntityID())
	case "tasks.task.add":
		response.BitrixData(c, start, map[string]any{
			"task": dummyBitrixTask(bitrix.Fields(payload)),
		})
	case "task.item.add":
		task := dummyBitrixTask(bitrix.Fields(payload))
		id, _ := strconv.ParseInt(stringifyAny(task["id"]), 10, 64)
		response.BitrixData(c, start, id)
	default:
		response.BitrixError(c, http.StatusBadRequest, "ERROR_METHOD_NOT_FOUND", "Method not found!")
	}
}

func dummyBitrixProfile() map[string]any {
	return map[string]any{
		"ID":              "1",
		"ADMIN":           true,
		"NAME":            "Администратор",
		"LAST_NAME":       "Портала",
		"SECOND_NAME":     "",
		"EMAIL":           "admin@cdp-dmp.ru",
		"PERSONAL_GENDER": "",
		"TIME_ZONE":       "Europe/Moscow",
	}
}

func dummyBitrixUsers() []map[string]any {
	return []map[string]any{
		dummyBitrixUser("1"),
		dummyBitrixUser("2"),
		dummyBitrixUser("3"),
	}
}

func dummyBitrixUser(id string) map[string]any {
	name, last, email, position := "Администратор", "Портала", "admin@cdp-dmp.ru", "Администратор"
	switch id {
	case "2":
		name, last, email, position = "Менеджер", "Идентификаций", "manager@cdp-dmp.ru", "Менеджер"
	case "3":
		name, last, email, position = "Оператор", "Колл-центра", "operator@cdp-dmp.ru", "Оператор"
	}
	return map[string]any{
		"ID":            id,
		"XML_ID":        "",
		"ACTIVE":        true,
		"NAME":          name,
		"LAST_NAME":     last,
		"SECOND_NAME":   "",
		"EMAIL":         email,
		"TIME_ZONE":     "Europe/Moscow",
		"IS_ONLINE":     "Y",
		"WORK_POSITION": position,
		"UF_DEPARTMENT": []int{1},
	}
}

func filterDummyUsers(id string) []map[string]any {
	all := dummyBitrixUsers()
	id = strings.TrimSpace(id)
	if id == "" {
		return all
	}
	for _, user := range all {
		if user["ID"] == id {
			return []map[string]any{user}
		}
	}
	return []map[string]any{}
}

func dummyBitrixStatuses(entityID string) []map[string]any {
	entityID = strings.ToUpper(strings.TrimSpace(entityID))
	sources := []map[string]any{
		dummyStatus("1", "SOURCE", "CALL", "Звонок", 10, true),
		dummyStatus("2", "SOURCE", "EMAIL", "Email", 20, true),
		dummyStatus("3", "SOURCE", "WEB", "Веб-сайт", 30, true),
		dummyStatus("4", "SOURCE", "ADVERTISING", "Реклама", 40, true),
		dummyStatus("5", "SOURCE", "PARTNER", "Существующий клиент", 50, true),
		dummyStatus("6", "SOURCE", "RECOMMENDATION", "По рекомендации", 60, true),
		dummyStatus("7", "SOURCE", "TRADE_SHOW", "Выставка", 70, true),
		dummyStatus("8", "SOURCE", "WEBFORM", "CRM-форма", 80, true),
		dummyStatus("9", "SOURCE", "CALLBACK", "Обратный звонок", 90, true),
		dummyStatus("10", "SOURCE", "RC_GENERATOR", "Генератор продаж", 100, true),
		dummyStatus("11", "SOURCE", "STORE", "Интернет-магазин", 110, true),
		dummyStatus("12", "SOURCE", "OTHER", "Другое", 120, true),
		dummyStatus("13", "SOURCE", "UC_BIGDATA", "BigData-IT", 130, false),
	}
	statuses := []map[string]any{
		dummyStatus("100", "STATUS", "NEW", "Не обработан", 10, true),
		dummyStatus("101", "STATUS", "IN_PROCESS", "В работе", 20, true),
		dummyStatus("102", "STATUS", "PROCESSED", "Обработан", 30, true),
		dummyStatus("103", "STATUS", "CONVERTED", "Качественный лид", 40, true),
		dummyStatus("104", "STATUS", "JUNK", "Некачественный лид", 50, true),
	}
	switch entityID {
	case "SOURCE":
		return sources
	case "STATUS":
		return statuses
	case "":
		out := make([]map[string]any, 0, len(sources)+len(statuses))
		out = append(out, sources...)
		out = append(out, statuses...)
		return out
	default:
		return []map[string]any{}
	}
}

func dummyStatus(id, entityID, statusID, name string, sort int, system bool) map[string]any {
	sys := "N"
	if system {
		sys = "Y"
	}
	return map[string]any{
		"ID":        id,
		"ENTITY_ID": entityID,
		"STATUS_ID": statusID,
		"NAME":      name,
		"NAME_INIT": name,
		"SORT":      sort,
		"SYSTEM":    sys,
	}
}

func dummyBitrixEntityID() int64 {
	return dummyEntitySeq.Add(1) + 10000
}

func dummyBitrixTask(fields map[string]any) map[string]any {
	id := dummyTaskSeq.Add(1) + 1000
	responsible := bitrix.String(fields, "RESPONSIBLE_ID")
	if responsible == "" {
		responsible = "1"
	}
	return map[string]any{
		"id":            strconv.FormatInt(id, 10),
		"title":         firstNonEmpty(bitrix.String(fields, "TITLE"), "Задача"),
		"description":   bitrix.String(fields, "DESCRIPTION"),
		"responsibleId": responsible,
	}
}

func bitrixLookupEntity(payload map[string]any) string {
	return strings.ToUpper(bitrix.String(bitrixLookupMap(payload, "filter"), "ENTITY_ID"))
}

func bitrixLookupID(payload map[string]any) string {
	if id := bitrix.String(payload, "ID"); id != "" {
		return id
	}
	if id := bitrix.String(payload, "id"); id != "" {
		return id
	}
	filter := bitrixLookupMap(payload, "filter")
	if id := bitrix.String(filter, "ID"); id != "" {
		return id
	}
	return bitrix.String(filter, "id")
}

func bitrixLookupMap(root map[string]any, key string) map[string]any {
	if root == nil {
		return nil
	}
	for _, k := range []string{key, strings.ToUpper(key), strings.ToLower(key)} {
		if m, ok := root[k].(map[string]any); ok {
			return m
		}
	}
	return nil
}

type dummyLeadField struct {
	Code       string
	Type       string
	Title      string
	Multiple   bool
	Required   bool
	ReadOnly   bool
	Dynamic    bool
	StatusType string
}

// Поля из документации crm.lead.add плюс стандартные, которые мапит BigData-IT.
func dummyCRMLeadFieldList() []dummyLeadField {
	return []dummyLeadField{
		{Code: "TITLE", Type: "string", Title: "Название"},
		{Code: "NAME", Type: "string", Title: "Имя"},
		{Code: "LAST_NAME", Type: "string", Title: "Фамилия"},
		{Code: "PHONE", Type: "string", Title: "Телефон", Required: true},
		{Code: "EMAIL", Type: "string", Title: "Email"},
		{Code: "COMMENTS", Type: "string", Title: "Комментарий"},
		{Code: "SOURCE_ID", Type: "crm_status", Title: "Источник", StatusType: "SOURCE"},
		{Code: "SOURCE_DESCRIPTION", Type: "string", Title: "Дополнительно об источнике"},
		{Code: "STATUS_ID", Type: "crm_status", Title: "Стадия", StatusType: "STATUS"},
		{Code: "ASSIGNED_BY_ID", Type: "user", Title: "Ответственный"},
		{Code: bitrix.WebIDField, Type: "string", Title: "Web ID", Dynamic: true},
		{Code: bitrix.OperatorField, Type: "string", Title: "Оператор", Dynamic: true},
		{Code: bitrix.RegionField, Type: "string", Title: "Регион", Dynamic: true},
		{Code: bitrix.TagField, Type: "string", Title: "Тег", Dynamic: true},
		{Code: bitrix.ProjectNumField, Type: "string", Title: "Номер проекта", Dynamic: true},
	}
}

func dummyBitrixLeadFields() map[string]any {
	out := make(map[string]any, 16)
	for _, f := range dummyCRMLeadFieldList() {
		out[f.Code] = f.toMap()
	}
	return out
}

func dummyBitrixUserfields() []map[string]any {
	out := make([]map[string]any, 0, 8)
	id := 1
	for _, f := range dummyCRMLeadFieldList() {
		if !f.Dynamic && !strings.HasPrefix(f.Code, "UF_") {
			continue
		}
		multiple := "N"
		if f.Multiple {
			multiple = "Y"
		}
		mandatory := "N"
		if f.Required {
			mandatory = "Y"
		}
		out = append(out, map[string]any{
			"ID":                strconv.Itoa(id),
			"ENTITY_ID":         "CRM_LEAD",
			"FIELD_NAME":        f.Code,
			"USER_TYPE_ID":      f.Type,
			"XML_ID":            f.Code,
			"SORT":              id * 100,
			"MULTIPLE":          multiple,
			"MANDATORY":         mandatory,
			"SHOW_FILTER":       "Y",
			"SHOW_IN_LIST":      "Y",
			"EDIT_IN_LIST":      "Y",
			"IS_SEARCHABLE":     "N",
			"EDIT_FORM_LABEL":   map[string]any{"ru": f.Title, "en": f.Title},
			"LIST_COLUMN_LABEL": map[string]any{"ru": f.Title, "en": f.Title},
			"LIST_FILTER_LABEL": map[string]any{"ru": f.Title, "en": f.Title},
		})
		id++
	}
	return out
}

func dummyBitrixCategories() map[string]any {
	return map[string]any{
		"categories": []map[string]any{
			{
				"id":           0,
				"name":         "Общая",
				"sort":         100,
				"entityTypeId": 1,
				"isDefault":    "Y",
			},
		},
	}
}

func (f dummyLeadField) toMap() map[string]any {
	m := map[string]any{
		"type":        f.Type,
		"isRequired":  f.Required,
		"isReadOnly":  f.ReadOnly,
		"isImmutable": f.ReadOnly,
		"isMultiple":  f.Multiple,
		"isDynamic":   f.Dynamic,
		"title":       f.Title,
		"formLabel":   f.Title,
		"listLabel":   f.Title,
		"filterLabel": f.Title,
		"TYPE":        f.Type,
		"TITLE":       f.Title,
	}
	if f.StatusType != "" {
		m["statusType"] = f.StatusType
	}
	return m
}

func stringifyAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	default:
		return ""
	}
}
