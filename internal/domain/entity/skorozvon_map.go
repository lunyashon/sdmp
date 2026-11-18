package entity

import "time"

type SkorozvonMap struct {
	FieldID    int64     `json:"field_id"`
	OptionID   int64     `json:"option_id"`
	Name       string    `json:"name"`
	BitrixCode string    `json:"bitrix_code"`
	SourceKey  string    `json:"source_key,omitempty"`
	FieldType  string    `json:"field_type"`
	LeadType   string    `json:"lead_type"`
	UpdatedAt  time.Time `json:"updated_at"`
}
