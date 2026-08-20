package usecase

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type stubS3 struct {
	files map[string]string
}

func (s stubS3) Put(context.Context, string, io.Reader, port.PutOptions) error {
	return nil
}
func (s stubS3) Delete(context.Context, string) error { return nil }
func (s stubS3) Exists(context.Context, string) (bool, error) {
	return false, nil
}
func (s stubS3) List(context.Context, string) ([]port.ObjectInfo, error) {
	return nil, nil
}
func (s stubS3) Ping(context.Context) error { return nil }
func (s stubS3) Get(_ context.Context, key string) (*port.Object, error) {
	body, ok := s.files[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &port.Object{
		Key:  key,
		Body: io.NopCloser(strings.NewReader(body)),
	}, nil
}

type stubSources struct {
	byID map[int]entity.Source
}

func (s stubSources) List(context.Context) ([]entity.Source, error) { return nil, nil }
func (s stubSources) GetByBitrixCode(context.Context, string) (entity.Source, error) {
	return entity.Source{}, domain.ErrNotFound
}
func (s stubSources) GetByID(_ context.Context, id int) (entity.Source, error) {
	src, ok := s.byID[id]
	if !ok {
		return entity.Source{}, domain.ErrNotFound
	}
	return src, nil
}

func TestMonthsInRange(t *testing.T) {
	from := time.Date(2026, 7, 15, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 8, 2, 0, 0, 0, 0, time.Local)
	got, err := monthsInRange(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].label() != "2026-07" || got[1].label() != "2026-08" {
		t.Fatalf("unexpected months: %+v", got)
	}
}

func TestMonthCSVKey(t *testing.T) {
	key := monthCSVKey("UC_L26K09", yearMonth{2023, time.October})
	want := "UC_L26K09/2023/10/csv/UC_L26K09-202310.csv"
	if key != want {
		t.Fatalf("got %s want %s", key, want)
	}
	key = monthCSVKey("143", yearMonth{2026, time.February})
	want = "143/2026/02/csv/143-202602.csv"
	if key != want {
		t.Fatalf("got %s want %s", key, want)
	}
}

func TestFilterLeadsFromCSV(t *testing.T) {
	csvBody := "phone,source_id,region,operator,city,name,owner_id,date_create\n" +
		"+79009876539,UC_L26K09,\"\",\"\",Липецк,Евгений,784338,2023-10-27 09:27:10\n" +
		"+79011079331,UC_L26K09,\"\",\"\",Армавир,Роман,784378,2023-10-28 10:02:16\n"

	svc := NewLeadService(stubS3{files: map[string]string{
		"UC_L26K09/2023/10/csv/UC_L26K09-202310.csv": csvBody,
	}}, stubSources{}, nil)

	res, err := svc.Filter(context.Background(), LeadFilterQuery{
		SourceID: "UC_L26K09",
		DateFrom: "2023-10-27",
		DateTo:   "2023-10-27",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Leads) != 1 || res.Leads[0].Phone != "+79009876539" {
		t.Fatalf("leads: %+v", res.Leads)
	}
	if len(res.MonthsLoaded) != 1 {
		t.Fatalf("loaded: %v", res.MonthsLoaded)
	}
}

func TestFilterInvalidRange(t *testing.T) {
	svc := NewLeadService(stubS3{files: map[string]string{}}, stubSources{}, nil)
	_, err := svc.Filter(context.Background(), LeadFilterQuery{
		SourceID: "UC_L26K09",
		DateFrom: "2023-10-28",
		DateTo:   "2023-10-27",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestNumericSourceIDUsesS3Folder(t *testing.T) {
	csvBody := "phone,source_id,region,operator,city,name,owner_id,date_create\n" +
		"+79001234567,143,\"\",\"\",Москва,Иван,1,2026-02-10 12:00:00\n"

	svc := NewLeadService(stubS3{files: map[string]string{
		"143/2026/02/csv/143-202602.csv": csvBody,
	}}, stubSources{byID: map[int]entity.Source{
		143: {ID: 143, BitrixCode: "UC_WBLW0N", Name: "Робот Карл"},
	}}, nil)

	res, err := svc.Filter(context.Background(), LeadFilterQuery{
		SourceID: "143",
		DateFrom: "2026-02-01",
		DateTo:   "2026-02-28",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.SourceID != "143" {
		t.Fatalf("source %s", res.SourceID)
	}
	if len(res.Leads) != 1 {
		t.Fatalf("leads: %+v", res.Leads)
	}
}
