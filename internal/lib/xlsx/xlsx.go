package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

func Encode(sheet string, rows [][]string) ([]byte, error) {
	name := strings.TrimSpace(sheet)
	if name == "" {
		name = "Sheet1"
	}
	name = strings.NewReplacer("\\", " ", "/", " ", "?", " ", "*", " ", "[", " ", "]", " ").Replace(name)
	if len(name) > 31 {
		name = name[:31]
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := []struct {
		path, body string
	}{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rels},
		{"xl/workbook.xml", workbookXML(name)},
		{"xl/_rels/workbook.xml.rels", workbookRels},
		{"xl/worksheets/sheet1.xml", sheetXML(rows)},
	}
	for _, f := range files {
		w, err := zw.Create(f.path)
		if err != nil {
			return nil, fmt.Errorf("xlsx create %s: %w", f.path, err)
		}
		if _, err := w.Write([]byte(f.body)); err != nil {
			return nil, fmt.Errorf("xlsx write %s: %w", f.path, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("xlsx close: %w", err)
	}
	return buf.Bytes(), nil
}

const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`

const rels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const workbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`

func workbookXML(name string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="` + xmlEscape(name) + `" sheetId="1" r:id="rId1"/></sheets>
</workbook>`
}

func sheetXML(rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for i, row := range rows {
		fmt.Fprintf(&b, `<row r="%d">`, i+1)
		for c, val := range row {
			fmt.Fprintf(&b, `<c r="%s%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, colName(c+1), i+1, xmlEscape(sanitize(val)))
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func colName(n int) string {
	if n < 1 {
		n = 1
	}
	var s []byte
	for n > 0 {
		n--
		s = append([]byte{byte('A' + n%26)}, s...)
		n /= 26
	}
	return string(s)
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, s)
}
