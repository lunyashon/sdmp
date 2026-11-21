package entity

import (
	"encoding/json"
	"time"
)

type GoogleSheet struct {
	ID              int                 `json:"id"`
	Name            string              `json:"name"`
	SpreadsheetID   string              `json:"spreadsheet_id"`
	SheetName       string              `json:"sheet_name"`
	IsActive        bool                `json:"is_active"`
	TestMode        bool                `json:"test_mode"`
	ZaryaResults    []string            `json:"zarya_results"`
	RegionIDs       []int               `json:"region_ids"`
	StopSourceIDs   []int               `json:"stop_source_ids"`
	ClientEmail     string              `json:"client_email"`
	ProjectID       string              `json:"project_id"`
	HasKey          bool                `json:"has_key"`
	LastDateRequest *time.Time          `json:"last_date_request,omitempty"`
	LastRequest     *bool               `json:"last_request,omitempty"`
	LastHTTPStatus  *int                `json:"last_http_status,omitempty"`
	LastResponse    string              `json:"last_response,omitempty"`
	Columns         []GoogleSheetColumn `json:"columns"`
}

// GoogleSheetColumn — заголовок из первой строки листа и значение, которое туда писать.
// Value — плейсхолдер ({{phone}}, {{region}}, {{city}}, {{phone_valid}}) или свой текст.
type GoogleSheetColumn struct {
	Header string `json:"header"`
	Value  string `json:"value"`
}

type GoogleSheetPreview struct {
	SheetName string   `json:"sheet_name"`
	Sheets    []string `json:"sheets"`
	Columns   []string `json:"columns"`
}

type GoogleSheetCheck struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	OK         bool      `json:"ok"`
	HTTPStatus int       `json:"http_status"`
	Response   string    `json:"response"`
	CheckedAt  time.Time `json:"checked_at"`
}

// GoogleAccount — сохранённый сервисный аккаунт без ключа. ID указывает строку, из которой ключ копируется.
type GoogleAccount struct {
	ID          int    `json:"id"`
	ClientEmail string `json:"client_email"`
}

// GoogleSheetTarget — инстанс для отправки строки в таблицу. Credentials не отдаются в API.
type GoogleSheetTarget struct {
	ID            int
	Name          string
	SpreadsheetID string
	SheetName     string
	Credentials   json.RawMessage
	Columns       []GoogleSheetColumn
}
