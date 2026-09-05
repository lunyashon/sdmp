package entity

import "time"

type Partner struct {
	ID              int        `json:"id"`
	Name            string     `json:"name"`
	WebhookURL      string     `json:"webhook_url"`
	IsActive        bool       `json:"is_active"`
	ZaryaResults    []string   `json:"zarya_results"`
	RegionIDs       []int      `json:"region_ids"`
	StopSourceIDs   []int      `json:"stop_source_ids"`
	LastDateRequest *time.Time `json:"last_date_request,omitempty"`
	LastRequest     *bool      `json:"last_request,omitempty"`
	LastHTTPStatus  *int       `json:"last_http_status,omitempty"`
	LastResponse    string     `json:"last_response,omitempty"`
}

type PartnerCheck struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	OK         bool      `json:"ok"`
	HTTPStatus int       `json:"http_status"`
	Response   string    `json:"response"`
	CheckedAt  time.Time `json:"checked_at"`
}
