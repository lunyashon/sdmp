package entity

import "time"

type BitrixLeadField struct {
	Code      string    `json:"code"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	Writable  bool      `json:"writable"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BitrixStatus struct {
	EntityID  string    `json:"entity_id"`
	StatusID  string    `json:"status_id"`
	Name      string    `json:"name"`
	Sort      int       `json:"sort"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BitrixLeadMap struct {
	SourceKey  string    `json:"source_key"`
	BitrixCode string    `json:"bitrix_code"`
	FixedValue string    `json:"fixed_value"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type BitrixCatalog struct {
	Fields   []BitrixLeadField `json:"fields"`
	Statuses []BitrixStatus    `json:"statuses"`
	Sources  []BitrixStatus    `json:"sources"`
}
