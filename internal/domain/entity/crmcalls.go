package entity

import "time"

type CrmcallsLink struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	ProjectID      string `json:"project_id"`
	Provider       string `json:"provider"`
	ChildSourceIDs []int  `json:"child_source_ids"`
	RegionIDs      []int  `json:"region_ids"`
}

type CrmcallsProject struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Status          string     `json:"status,omitempty"`
	LastDateRequest *time.Time `json:"last_date_request,omitempty"`
	LastRequest     *bool      `json:"last_request,omitempty"`
}

type CrmcallsTestResult struct {
	ContactID  string `json:"contact_id"`
	HTTPStatus int    `json:"http_status"`
}

type CrmcallsSettings struct {
	APIKey  string
	BaseURL string
}

type CrmcallsConnection struct {
	HasKey       bool   `json:"has_key"`
	BaseURL      string `json:"base_url"`
	APIKeyMasked string `json:"api_key_masked"`
	ProjectCount int    `json:"project_count"`
}
