package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type stubLeadList struct {
	got   port.LeadListFilter
	items []entity.LeadListItem
	total int
	err   error
}

func (s *stubLeadList) InsertAccepted(context.Context, entity.LeadRow, entity.OutboxMessage) (int64, error) {
	return 0, nil
}

func (s *stubLeadList) List(_ context.Context, f port.LeadListFilter) ([]entity.LeadListItem, int, error) {
	s.got = f
	return s.items, s.total, s.err
}

func (s *stubLeadList) CountBySource(context.Context, time.Time, time.Time) ([]entity.LeadSourceCount, error) {
	return []entity.LeadSourceCount{{SourceID: 7, Count: 21}}, s.err
}

func (s *stubLeadList) CountEnrichmentBySource(context.Context, time.Time, time.Time) ([]entity.LeadEnrichmentStat, error) {
	return []entity.LeadEnrichmentStat{{
		SourceID: 7, Total: 20,
		Region:   entity.LeadGeoShare{Supplier: 10, Provider: 6, Unknown: 4},
		Operator: entity.LeadGeoShare{Supplier: 8, Provider: 9, Unknown: 3},
	}}, s.err
}

func (s *stubLeadList) CountZaryaBySource(context.Context, time.Time, time.Time) ([]entity.LeadZaryaStat, error) {
	return []entity.LeadZaryaStat{{SourceID: 7, Total: 15, Unique: 9, Dup: 6}}, s.err
}

func (s *stubLeadList) Get(context.Context, int64) (entity.LeadDetail, error) {
	return entity.LeadDetail{}, nil
}

func (s *stubLeadList) UpdateEnrichment(context.Context, int64, entity.LeadEnrichment) error {
	return nil
}

func (s *stubLeadList) InsertDispatch(context.Context, entity.Dispatch) error {
	return nil
}

func (s *stubLeadList) SaveZaryaResult(context.Context, int64, bool) error {
	return nil
}

func (s *stubLeadList) BindSkorozvon(context.Context, int64, string, string) error {
	return nil
}

func (s *stubLeadList) ApplyCallResult(context.Context, string, entity.LeadCallResult) error {
	return nil
}

func TestFilterLeads(t *testing.T) {
	created := time.Date(2026, 8, 27, 16, 27, 56, 0, time.Local)
	repo := &stubLeadList{
		total: 2,
		items: []entity.LeadListItem{{
			DateCreate: created,
			Source:     "GDMP",
			ColdLabel:  "",
			Region:     "Москва",
			Operator:   "БИЛАЙН",
			Stage:      "в Б24",
			Phone:      "79009876539",
		}},
	}
	svc := NewLeadService(repo, nil)
	res, err := svc.Filter(context.Background(), LeadFilterQuery{
		DateFrom:   "01.08.2026 00:00:00",
		DateTo:     "30.08.2026 23:59:59",
		Sources:    []int{1, 12},
		ColdLabels: []string{"ВР"},
		Regions:    []int{1, 5},
		Operators:  []int{2, 3},
		Zarya:      []string{"Чистый", "Дубль"},
		Phone:      "8 (900) 987-65-39",
		Limit:      0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 || res.Limit != 10 || len(res.Leads) != 1 {
		t.Fatalf("%+v", res)
	}
	if repo.got.Phone != "79009876539" {
		t.Fatalf("phone %q", repo.got.Phone)
	}
	if len(repo.got.Sources) != 2 || repo.got.ColdLabels[0] != "ВР" {
		t.Fatalf("filter %+v", repo.got)
	}
	if len(repo.got.Regions) != 2 || len(repo.got.Operators) != 2 || len(repo.got.Zarya) != 2 || repo.got.Zarya[0] != "Чистый" {
		t.Fatalf("filter %+v", repo.got)
	}
}

func TestFilterInvalidRange(t *testing.T) {
	svc := NewLeadService(&stubLeadList{}, nil)
	_, err := svc.Filter(context.Background(), LeadFilterQuery{
		DateFrom: "30.08.2026 23:59:59",
		DateTo:   "01.08.2026 00:00:00",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestFilterRequiresDates(t *testing.T) {
	svc := NewLeadService(&stubLeadList{}, nil)
	_, err := svc.Filter(context.Background(), LeadFilterQuery{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestIncomingStats(t *testing.T) {
	got, err := NewLeadService(&stubLeadList{}, nil).IncomingStats(context.Background(), "2026-08-14", "2026-08-20")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SourceID != 7 || got[0].Count != 21 {
		t.Fatalf("%+v", got)
	}
}

func TestEnrichmentStats(t *testing.T) {
	got, err := NewLeadService(&stubLeadList{}, nil).EnrichmentStats(context.Background(), "2026-08-14", "2026-08-20")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Region.Supplier != 10 || got[0].Operator.Unknown != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestZaryaStats(t *testing.T) {
	got, err := NewLeadService(&stubLeadList{}, nil).ZaryaStats(context.Background(), "2026-08-14", "2026-08-20")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Unique != 9 || got[0].Dup != 6 {
		t.Fatalf("%+v", got)
	}
}

func TestGetLead(t *testing.T) {
	created := time.Date(2026, 8, 27, 16, 27, 56, 0, time.Local)
	repo := &stubLeadGet{
		detail: entity.LeadDetail{ID: 7, Phone: "79001234567", CreatedAt: created, Stage: "принята"},
	}
	got, err := NewLeadService(repo, nil).Get(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 || repo.gotID != 7 {
		t.Fatalf("%+v id=%d", got, repo.gotID)
	}
}

func TestGetLeadInvalidID(t *testing.T) {
	_, err := NewLeadService(&stubLeadList{}, nil).Get(context.Background(), 0)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

type stubCallResultRepo struct {
	stubLeadList
	gotID string
	got   entity.LeadCallResult
	err   error
}

func (s *stubCallResultRepo) ApplyCallResult(_ context.Context, skorozvonLeadID string, in entity.LeadCallResult) error {
	s.gotID = skorozvonLeadID
	s.got = in
	return s.err
}

func TestApplyCallResult(t *testing.T) {
	ended := time.Date(2019, 1, 30, 10, 6, 8, 0, time.UTC)
	result := int32(54321)
	repo := &stubCallResultRepo{}
	err := NewLeadService(repo, nil).ApplyCallResult(context.Background(), " 123456788 ", entity.LeadCallResult{
		CallID:     "987654321",
		CallURL:    "https://api.skorozvon.ru/api/calls/987654321.mp3",
		CallLastAt: &ended,
		CallResult: &result,
		ProjectNum: "12345678",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.gotID != "123456788" {
		t.Fatalf("id %q", repo.gotID)
	}
	if repo.got.CallID != "987654321" || repo.got.ProjectNum != "12345678" || repo.got.CallResult == nil || *repo.got.CallResult != 54321 {
		t.Fatalf("%+v", repo.got)
	}
}

func TestApplyCallResultRequiresLeadID(t *testing.T) {
	err := NewLeadService(&stubLeadList{}, nil).ApplyCallResult(context.Background(), "  ", entity.LeadCallResult{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

type stubLeadGet struct {
	stubLeadList
	detail entity.LeadDetail
	gotID  int64
}

func (s *stubLeadGet) Get(_ context.Context, id int64) (entity.LeadDetail, error) {
	s.gotID = id
	return s.detail, nil
}
