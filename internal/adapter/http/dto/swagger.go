package dto

import "github.com/lunyashon/sdmp/internal/domain/entity"

// OperatorAliasRequest тело POST /rest/operators/{id}/aliases.
type OperatorAliasRequest struct {
	Alias string `json:"alias" example:"Билайн"`
}

// PartnerRequest тело создания и обновления партнёра.
type PartnerRequest struct {
	Name          string            `json:"name"`
	WebhookURL    string            `json:"webhook_url"`
	IsActive      *bool             `json:"is_active"`
	ZaryaResults  []string          `json:"zarya_results"`
	RegionIDs     []int             `json:"region_ids"`
	StopSourceIDs []int             `json:"stop_source_ids"`
	Body          map[string]string `json:"body"`
	ContentType   string            `json:"content_type"`
}

// SkorozvonProjectRequest тело POST /rest/skorozvon/projects.
type SkorozvonProjectRequest struct {
	Name      string `json:"name"`
	ProjectID int64  `json:"project_id"`
}

// SkorozvonLinkRequest тело создания и обновления связки Скорозвона.
type SkorozvonLinkRequest struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	ProjectID      int64  `json:"project_id"`
	ChildSourceIDs []int  `json:"child_source_ids"`
	RegionIDs      []int  `json:"region_ids"`
}

// SkorozvonEventRequest входящий webhook Скорозвона (call_result / form_response).
type SkorozvonEventRequest struct {
	Type string `json:"type" example:"call_result"`
}

// CRMLeadPhone телефон в формате Битрикса. Также принимается строка PHONE: "79001234567".
type CRMLeadPhone struct {
	Value     string `json:"VALUE" example:"79001234567"`
	ValueType string `json:"VALUE_TYPE" example:"WORK"`
}

// CRMLeadAddFields поля crm.lead.add. Неизвестные ключи тоже сохраняются в сырьё.
type CRMLeadAddFields struct {
	Phone      []CRMLeadPhone `json:"PHONE"`
	SourceID   string         `json:"SOURCE_ID" example:"GDMP"`
	WebID      string         `json:"UF_CRM_1752577998838" example:"web-123"`
	Operator   string         `json:"UF_CRM_1734523005552" example:"МТС"`
	Region     string         `json:"UF_CRM_1747216350206" example:"Москва"`
	Tag        string         `json:"UF_CRM_1766149282" example:"подтверждён"`
	ProjectNum string         `json:"UF_CRM_1699340770806" example:"12345"`
	Title      string         `json:"TITLE" example:"Лид"`
	Name       string         `json:"NAME" example:"Иван"`
}

// CRMLeadAddRequest тело приёма лида. Те же ключи можно передать query или form: fields[PHONE][0][VALUE].
type CRMLeadAddRequest struct {
	Fields CRMLeadAddFields `json:"fields"`
	// PHONE на корне, если нет fields.PHONE.
	Phone string `json:"PHONE,omitempty" example:"79001234567"`
	// SOURCE_ID на корне, если нет fields.SOURCE_ID.
	SourceID string `json:"SOURCE_ID,omitempty" example:"GDMP"`
	// WebID на корне, если нет fields.UF_CRM_1752577998838.
	WebID string `json:"UF_CRM_1752577998838,omitempty"`
}

type NamedRefList struct {
	Count int               `json:"count"`
	Items []entity.NamedRef `json:"items"`
}

type OperatorGroupList struct {
	Count int                    `json:"count"`
	Items []entity.OperatorGroup `json:"items"`
}

type PartnerList struct {
	Count int              `json:"count"`
	Items []entity.Partner `json:"items"`
}

type PartnerCheckList struct {
	Count int                   `json:"count"`
	Items []entity.PartnerCheck `json:"items"`
}

type SkorozvonProjectList struct {
	Count int                       `json:"count"`
	Items []entity.SkorozvonProject `json:"items"`
}

type SkorozvonLinkList struct {
	Count int                    `json:"count"`
	Items []entity.SkorozvonLink `json:"items"`
}

type LeadSourceCountList struct {
	Count int                      `json:"count"`
	Items []entity.LeadSourceCount `json:"items"`
}

type LeadEnrichmentStatList struct {
	Count int                         `json:"count"`
	Items []entity.LeadEnrichmentStat `json:"items"`
}

type LeadZaryaStatList struct {
	Count int                    `json:"count"`
	Items []entity.LeadZaryaStat `json:"items"`
}

type LeadSkorozvonBitrixStatList struct {
	Count int                              `json:"count"`
	Items []entity.LeadSkorozvonBitrixStat `json:"items"`
}

type SourceSplitItems struct {
	Items []entity.SourceSplit `json:"items"`
}

type PartnerResendResult struct {
	PartnerID   int    `json:"partner_id"`
	PartnerName string `json:"partner_name"`
	SentOK      bool   `json:"sent_ok"`
	HTTPStatus  int    `json:"http_status"`
	RawResponse string `json:"raw_response"`
	Attempts    int16  `json:"attempts"`
}
