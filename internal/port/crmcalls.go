package port

import (
	"context"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type CrmcallsLead struct {
	Phone      string
	ExternalID string
	Name       string
	ProjectID  string
	Region     string
}

type CrmcallsSendResult struct {
	ContactID string
	Status    int
	Body      []byte
}

type Crmcalls interface {
	SendLead(ctx context.Context, lead CrmcallsLead) (CrmcallsSendResult, error)
	ListProjects(ctx context.Context) ([]entity.CrmcallsProject, error)
	// DownloadRecording — call_id или полный record_url из постбэка.
	DownloadRecording(ctx context.Context, callIDOrURL string) (body []byte, contentType string, err error)
}

type CrmcallsLinkRepository interface {
	List(ctx context.Context) ([]entity.CrmcallsLink, error)
	Get(ctx context.Context, id int) (entity.CrmcallsLink, error)
	Create(ctx context.Context, in entity.CrmcallsLink) (entity.CrmcallsLink, error)
	Update(ctx context.Context, in entity.CrmcallsLink) (entity.CrmcallsLink, error)
	Delete(ctx context.Context, id int) error
	ListRoutable(ctx context.Context, kind string, childSourceID int, regionID *int) ([]entity.CrmcallsLink, error)
}

type CrmcallsProjectRepository interface {
	List(ctx context.Context) ([]entity.CrmcallsProject, error)
	Get(ctx context.Context, id string) (entity.CrmcallsProject, error)
	Upsert(ctx context.Context, items []entity.CrmcallsProject) error
	Ensure(ctx context.Context, id, name string) error
	UpdateCheck(ctx context.Context, id string, at time.Time, ok bool, status, name string) (entity.CrmcallsProject, error)
}

type CrmcallsSettingsRepository interface {
	Get(ctx context.Context) (entity.CrmcallsSettings, error)
	Save(ctx context.Context, in entity.CrmcallsSettings) error
}
