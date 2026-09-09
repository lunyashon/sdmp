package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestEncodeContainsCells(t *testing.T) {
	raw, err := Encode("Лиды", [][]string{
		{"UUID", "Телефон"},
		{"abc-1", "79001234567"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 4 || string(raw[:2]) != "PK" {
		t.Fatalf("not zip")
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var sheet string
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
		sheet = string(body)
	}
	if !strings.Contains(sheet, "79001234567") || !strings.Contains(sheet, "UUID") {
		t.Fatalf("sheet %s", sheet)
	}
}
