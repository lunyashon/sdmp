package port

import (
	"context"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type LeadListFilter struct {
	DateFrom   time.Time
	DateTo     time.Time
	Sources      []int
	ChildSources []int
	ColdLabels   []string
	Regions    []int
	Operators  []int
	Statuses   []int
	Zarya      []string
	Phone      string
	City       string
	Tag        string
	CallResult *int32
	Limit      int
	Offset     int
}

type LeadRepository interface {
	InsertAccepted(ctx context.Context, row entity.LeadRow, outbox entity.OutboxMessage) (int64, error)
	List(ctx context.Context, f LeadListFilter) ([]entity.LeadListItem, int, error)
	CountBySource(ctx context.Context, from, to time.Time) ([]entity.LeadSourceCount, error)
	CountEnrichmentBySource(ctx context.Context, from, to time.Time) ([]entity.LeadEnrichmentStat, error)
	CountZaryaBySource(ctx context.Context, from, to time.Time) ([]entity.LeadZaryaStat, error)
	Get(ctx context.Context, id int64) (entity.LeadDetail, error)
	GetByUUID(ctx context.Context, uuid string) (entity.LeadDetail, error)
	UpdateEnrichment(ctx context.Context, leadID int64, in entity.LeadEnrichment) error
	BindSkorozvon(ctx context.Context, leadID int64, skorozvonLeadID, projectID string) error
	BindBitrix(ctx context.Context, leadUUID, bitrixLeadID string) error
	ApplyCallResult(ctx context.Context, skorozvonLeadID string, in entity.LeadCallResult) error
	ApplyFormResponse(ctx context.Context, skorozvonLeadID string, in entity.LeadFormResponse) error
	InsertFromSkorozvon(ctx context.Context, in entity.LeadFromSkorozvon) error
	InsertDispatch(ctx context.Context, in entity.Dispatch) error
	SaveZaryaResult(ctx context.Context, leadID int64, duplicate bool) error
}
