package entity

import "encoding/json"

type GoogleSheet struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	SpreadsheetID string   `json:"spreadsheet_id"`
	SheetName     string   `json:"sheet_name"`
	IsActive      bool     `json:"is_active"`
	TestMode      bool     `json:"test_mode"`
	ZaryaResults  []string `json:"zarya_results"`
	RegionIDs     []int    `json:"region_ids"`
	StopSourceIDs []int    `json:"stop_source_ids"`
	ClientEmail   string   `json:"client_email"`
	ProjectID     string   `json:"project_id"`
	HasKey        bool     `json:"has_key"`
}

// GoogleSheetTarget — инстанс для отправки строки в таблицу. Credentials не отдаются в API.
type GoogleSheetTarget struct {
	ID            int
	Name          string
	SpreadsheetID string
	SheetName     string
	Credentials   json.RawMessage
}
