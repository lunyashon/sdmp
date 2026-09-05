package entity

import "time"

type SkorozvonProject struct {
	UUID            string     `json:"uuid"`
	ProjectID       int64      `json:"project_id"`
	Name            string     `json:"name"`
	CreatedAt       time.Time  `json:"created_at"`
	LastDateRequest *time.Time `json:"last_date_request"`
	LastRequest     *bool      `json:"last_request"`
}
