package entity

type Source struct {
	ID                 int    `json:"id"`
	ParentID           *int   `json:"parent_id"`
	Name               string `json:"name"`
	BitrixCode         string `json:"bitrix_code"`
	Token              string `json:"token"`
	ColdLabel          string `json:"cold_label"`
	BaseLabel          string `json:"base_label"`
	IsActive           bool   `json:"is_active"`
	IdempotencyActive  bool   `json:"idempotency_active"`
}
