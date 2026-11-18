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

func TestDecodeRoundtrip(t *testing.T) {
	raw, err := Encode("Лиды", [][]string{
		{"Телефон", "Город"},
		{"79001234567", "Москва"},
		{"", "Казань"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0][0] != "Телефон" || got[1][0] != "79001234567" || got[2][1] != "Казань" {
		t.Fatalf("%#v", got)
	}
}

func TestDecodeSharedStrings(t *testing.T) {
	raw := xlsxWithSharedStrings(t)
	got, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0][0] != "Телефон" || got[1][0] != "79001234567" {
		t.Fatalf("%#v", got)
	}
}

func xlsxWithSharedStrings(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml": contentTypes + `<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>`,
		"_rels/.rels":         rels,
		"xl/workbook.xml":     workbookXML("Sheet1"),
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
</Relationships>`,
		"xl/sharedStrings.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="3" uniqueCount="3">
  <si><t>Телефон</t></si>
  <si><t>79001234567</t></si>
  <si><r><t>Мо</t></r><r><t>сква</t></r></si>
</sst>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="str"><v>Город</v></c></row>
    <row r="2"><c r="A2" t="s"><v>1</v></c><c r="B2" t="s"><v>2</v></c></row>
  </sheetData>
</worksheet>`,
	}
	for path, body := range files {
		w, err := zw.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
