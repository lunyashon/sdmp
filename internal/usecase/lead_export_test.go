package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
)

func TestExportXLSXSelectedColumnsAndIDs(t *testing.T) {
	created := time.Date(2026, 8, 27, 16, 27, 56, 0, time.UTC)
	repo := &stubLeadList{
		total: 1,
		items: []entity.LeadListItem{{
			ID:         42,
			UUID:       "abc-uuid",
			DateCreate: created,
			Phone:      "79001234567",
			Source:     "ТЕСТ",
			ViaUs:      false,
		}},
	}
	file, err := NewLeadService(repo, nil).ExportXLSX(context.Background(), LeadFilterQuery{
		DateFrom: "01.08.2026 00:00:00",
		DateTo:   "30.08.2026 23:59:59",
		IDs:      []int64{42, 42, 0},
	}, []string{"id", "phone", "nope", "uuid", "viaUs"})
	if err != nil {
		t.Fatal(err)
	}
	if file.ContentType == "" || !strings.HasPrefix(file.Filename, "leads_") || string(file.Body[:2]) != "PK" {
		t.Fatalf("%+v", file)
	}
	if len(repo.got.IDs) != 1 || repo.got.IDs[0] != 42 {
		t.Fatalf("ids %+v", repo.got.IDs)
	}
	sheet := xlsxSheet(t, file.Body)
	if !strings.Contains(sheet, "42") || !strings.Contains(sheet, "79001234567") || !strings.Contains(sheet, "abc-uuid") || !strings.Contains(sheet, "Создалась") {
		t.Fatalf("sheet %s", sheet)
	}
	if strings.Contains(sheet, "ТЕСТ") {
		t.Fatalf("dropped source column still present")
	}
}

func TestExportXLSXRequiresColumns(t *testing.T) {
	_, err := NewLeadService(&stubLeadList{}, nil).ExportXLSX(context.Background(), LeadFilterQuery{
		DateFrom: "01.08.2026 00:00:00",
		DateTo:   "30.08.2026 23:59:59",
	}, nil)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestExportXLSXRejectsTooMany(t *testing.T) {
	repo := &stubLeadList{total: exportMaxRows + 1}
	_, err := NewLeadService(repo, nil).ExportXLSX(context.Background(), LeadFilterQuery{
		DateFrom: "01.08.2026 00:00:00",
		DateTo:   "30.08.2026 23:59:59",
	}, []string{"uuid"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func xlsxSheet(t *testing.T, raw []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	t.Fatal("no sheet")
	return ""
}
