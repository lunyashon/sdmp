package port

import "context"

type SkorozvonLead struct {
	ID           int64             `json:"id"`
	Phone        string            `json:"phone"`
	ProjectID    int64             `json:"project_id"`
	City         string            `json:"city"`
	Region       string            `json:"region"`
	ExternalID   string            `json:"external_id"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

type SkorozvonSendResult struct {
	LeadID int
	Status int
	Body   []byte
}

type SkorozvonCustomFieldOption struct {
	ID    int64
	Label string
}

type SkorozvonCustomField struct {
	ID        int64
	Name      string
	FieldType string
	LeadType  string
	Options   []SkorozvonCustomFieldOption
}

type SkorozvonCallProject struct {
	ID    int64
	Title string
}

type Skorozvon interface {
	SendLead(ctx context.Context, lead SkorozvonLead) (SkorozvonSendResult, error)
	DeleteLead(ctx context.Context, id int) error
	CheckProject(ctx context.Context, projectID int64) bool
	ListCustomFields(ctx context.Context) ([]SkorozvonCustomField, error)
	ListCallProjects(ctx context.Context) ([]SkorozvonCallProject, error)
	DownloadCallRecording(ctx context.Context, callID string) ([]byte, string, error)
}
