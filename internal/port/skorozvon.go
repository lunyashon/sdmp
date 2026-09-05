package port

import "context"

type SkorozvonLead struct {
	ID        int64  `json:"id"`
	Phone     string `json:"phone"`
	ProjectID int64  `json:"project_id"`
	City      string `json:"city"`
	Region    string `json:"region"`
	ExternalID string `json:"external_id"`
}

type SkorozvonSendResult struct {
	LeadID int
	Status int
	Body   []byte
}

type Skorozvon interface {
	SendLead(ctx context.Context, lead SkorozvonLead) (SkorozvonSendResult, error)
	DeleteLead(ctx context.Context, id int) error
	CheckProject(ctx context.Context, projectID int64) bool
}