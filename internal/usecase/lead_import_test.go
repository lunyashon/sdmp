package usecase

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/xlsx"
)

func TestPreviewLeadImportCSV(t *testing.T) {
	raw := []byte("Телефон;Город\n79001234567;Москва\n79007654321;Казань\n")
	got, err := PreviewLeadImport("leads.csv", raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.RowCount != 2 || got.Filename != "leads.csv" {
		t.Fatalf("%+v", got)
	}
	if len(got.Columns) != 2 || got.Columns[0].Name != "Телефон" || got.Suggested["0"] != "phone" || got.Suggested["1"] != "city" {
		t.Fatalf("cols %+v suggested %+v", got.Columns, got.Suggested)
	}
	if len(got.Rows) != 2 || got.Rows[0][0] != "79001234567" {
		t.Fatalf("rows %+v", got.Rows)
	}
	if len(got.Fields) == 0 || got.Fields[1].Column != "phone" {
		t.Fatalf("fields %+v", got.Fields)
	}
}

func TestPreviewLeadImportXLSX(t *testing.T) {
	body, err := xlsx.Encode("Лиды", [][]string{
		{"UUID", "phone"},
		{"abc-1", "79001234567"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := PreviewLeadImport("import.xlsx", body)
	if err != nil {
		t.Fatal(err)
	}
	if got.Suggested["0"] != "uuid" || got.Suggested["1"] != "phone" {
		t.Fatalf("suggested %+v", got.Suggested)
	}
	if got.RowCount != 1 || got.Rows[0][1] != "79001234567" {
		t.Fatalf("%+v", got)
	}
}

func TestPreviewLeadImportRejectsEmpty(t *testing.T) {
	_, err := PreviewLeadImport("a.csv", nil)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("%v", err)
	}
}

type stubImportRepo struct {
	stubLeadList
	mu   sync.Mutex
	rows []entity.LeadImportRow
}

func (s *stubImportRepo) InsertImport(_ context.Context, row entity.LeadImportRow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, row)
	return nil
}

func TestImportQueuesRows(t *testing.T) {
	repo := &stubImportRepo{}
	svc := NewLeadService(repo, nil).
		WithSources(stubSources{byID: map[int]entity.Source{
			10: {ID: 10, Name: "GDMP"},
		}})
	got, err := svc.Import(context.Background(), LeadImportInput{
		SourceID: 10,
		Mapping:  map[string]string{"0": "phone", "1": "city"},
		Rows:     [][]string{{"79001234567", "Москва"}, {"", "Казань"}, {"79007654321", "Сочи"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Queued != 2 || got.Failed != 1 || len(repo.rows) != 2 {
		t.Fatalf("%+v n=%d", got, len(repo.rows))
	}
	phones := map[string]bool{}
	for _, row := range repo.rows {
		phones[row.Phone] = true
		if row.SourceID != 10 || row.Payload == "" {
			t.Fatalf("%+v", row)
		}
		if row.Values["source_id"] != 10 {
			t.Fatalf("source %+v", row.Values)
		}
		if _, ok := row.Values["child_source_id"]; ok {
			t.Fatalf("child source must be empty: %+v", row.Values)
		}
		if !strings.Contains(row.Payload, "7900") {
			t.Fatalf("payload %s", row.Payload)
		}
	}
	if !phones["79001234567"] || !phones["79007654321"] {
		t.Fatalf("%+v", repo.rows)
	}
}

func TestImportRequiresPhoneMapping(t *testing.T) {
	svc := NewLeadService(&stubImportRepo{}, nil).
		WithSources(stubSources{byID: map[int]entity.Source{
			10: {ID: 10, Name: "GDMP"},
		}})
	_, err := svc.Import(context.Background(), LeadImportInput{
		SourceID: 10,
		Mapping:  map[string]string{"0": "city"},
		Rows:     [][]string{{"Москва"}},
	})
	if err == nil || !strings.Contains(err.Error(), "phone") {
		t.Fatalf("%v", err)
	}
}

func TestImportRequiresSource(t *testing.T) {
	svc := NewLeadService(&stubImportRepo{}, nil).
		WithSources(stubSources{byID: map[int]entity.Source{10: {ID: 10, Name: "GDMP"}}})
	_, err := svc.Import(context.Background(), LeadImportInput{
		Mapping: map[string]string{"0": "phone"},
		Rows:    [][]string{{"79001234567"}},
	})
	if err == nil || !strings.Contains(err.Error(), "source_id") {
		t.Fatalf("%v", err)
	}
}

func TestImportRejectsChildAsSource(t *testing.T) {
	parentID := 10
	svc := NewLeadService(&stubImportRepo{}, nil).
		WithSources(stubSources{byID: map[int]entity.Source{
			10: {ID: 10, Name: "GDMP"},
			20: {ID: 20, ParentID: &parentID, Name: "МТС"},
		}})
	_, err := svc.Import(context.Background(), LeadImportInput{
		SourceID: 20,
		Mapping:  map[string]string{"0": "phone"},
		Rows:     [][]string{{"79001234567"}},
	})
	if err == nil || !strings.Contains(err.Error(), "parent source") {
		t.Fatalf("%v", err)
	}
}
