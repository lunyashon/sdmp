package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/port"
)

type stubSkorozvonMapsAPI struct {
	fields []port.SkorozvonCustomField
	err    error
}

func (s stubSkorozvonMapsAPI) SendLead(context.Context, port.SkorozvonLead) (port.SkorozvonSendResult, error) {
	return port.SkorozvonSendResult{}, nil
}
func (s stubSkorozvonMapsAPI) DeleteLead(context.Context, int) error { return nil }
func (s stubSkorozvonMapsAPI) CheckProject(context.Context, int64) bool {
	return true
}
func (s stubSkorozvonMapsAPI) ListCustomFields(context.Context) ([]port.SkorozvonCustomField, error) {
	return s.fields, s.err
}

type stubSkorozvonMapsRepo struct {
	got []entity.SkorozvonMap
	err error
}

func (s *stubSkorozvonMapsRepo) ReplaceAll(_ context.Context, rows []entity.SkorozvonMap) error {
	s.got = append([]entity.SkorozvonMap(nil), rows...)
	return s.err
}

func (s *stubSkorozvonMapsRepo) Get(context.Context, int64, int64) (entity.SkorozvonMap, bool, error) {
	return entity.SkorozvonMap{}, false, nil
}

func TestSyncSkorozvonMapsStoresFieldsAndOptions(t *testing.T) {
	repo := &stubSkorozvonMapsRepo{}
	svc := NewWorkerService(nil, nil, nil, nil, nil, nil, nil, 1).
		WithSkorozvon(nil, stubSkorozvonMapsAPI{fields: []port.SkorozvonCustomField{{
			ID:        50000011777,
			Name:      "Источник",
			FieldType: "select",
			LeadType:  "lead",
			Options: []port.SkorozvonCustomFieldOption{
				{ID: 50000004560, Label: "ВР"},
			},
		}, {
			ID:        50000011780,
			Name:      "test info",
			FieldType: "text",
		}, {
			ID:        50000011790,
			Name:      "Регион клиента",
			FieldType: "multiselect",
		}}}).
		WithSkorozvonMaps(repo)

	if err := svc.SyncSkorozvonMaps(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.got) != 4 {
		t.Fatalf("rows %+v", repo.got)
	}
	if repo.got[0].FieldID != 50000011777 || repo.got[0].OptionID != 0 || repo.got[0].Name != "Источник" || repo.got[0].BitrixCode != "" {
		t.Fatalf("field %+v", repo.got[0])
	}
	if repo.got[1].OptionID != 50000004560 || repo.got[1].Name != "ВР" {
		t.Fatalf("option %+v", repo.got[1])
	}
	if repo.got[2].FieldID != 50000011780 || repo.got[2].OptionID != 0 {
		t.Fatalf("text %+v", repo.got[2])
	}
	if repo.got[3].Name != "Регион клиента" || repo.got[3].BitrixCode != "UF_CRM_1747216350206" {
		t.Fatalf("bitrix %+v", repo.got[3])
	}
}

func TestBitrixCodeForSkorozvonName(t *testing.T) {
	if got := bitrixCodeForSkorozvonName("Регион клиента"); got != "UF_CRM_1747216350206" {
		t.Fatalf("region %q", got)
	}
	if got := bitrixCodeForSkorozvonName("Телефоны"); got != "PHONE[0][VALUE]" {
		t.Fatalf("phone %q", got)
	}
	if got := bitrixCodeForSkorozvonName("Название компании"); got != "" {
		t.Fatalf("company %q", got)
	}
}

func TestSyncSkorozvonMapsRejectsEmpty(t *testing.T) {
	repo := &stubSkorozvonMapsRepo{}
	svc := NewWorkerService(nil, nil, nil, nil, nil, nil, nil, 1).
		WithSkorozvon(nil, stubSkorozvonMapsAPI{}).
		WithSkorozvonMaps(repo)
	if err := svc.SyncSkorozvonMaps(context.Background()); err == nil {
		t.Fatal("expected empty error")
	}
	if repo.got != nil {
		t.Fatalf("must not replace: %+v", repo.got)
	}
}

func TestNextSkorozvonMapsAt(t *testing.T) {
	cases := []struct {
		hour, min int
		wantHour  int
		wantDay   int
	}{
		{11, 59, 12, 8},
		{12, 0, 19, 8},
		{12, 1, 19, 8},
		{18, 59, 19, 8},
		{19, 0, 12, 9},
		{19, 1, 12, 9},
	}
	for _, tc := range cases {
		now := time.Date(2026, 9, 8, tc.hour, tc.min, 0, 0, clock.Location)
		got := nextSkorozvonMapsAt(now)
		if got.Hour() != tc.wantHour || got.Day() != tc.wantDay {
			t.Fatalf("%s -> %s, want %02d:00 day %d", now.Format("15:04"), got.Format("02 15:04"), tc.wantHour, tc.wantDay)
		}
		if !got.After(now) {
			t.Fatalf("%s not after %s", got, now)
		}
	}
}
