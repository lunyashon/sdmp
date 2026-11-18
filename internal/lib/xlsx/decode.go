package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func Decode(raw []byte) ([][]string, error) {
	if len(raw) < 4 || string(raw[:2]) != "PK" {
		return nil, fmt.Errorf("not an xlsx zip")
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[strings.ToLower(strings.ReplaceAll(f.Name, "\\", "/"))] = f
	}
	shared, err := decodeSharedStrings(files["xl/sharedstrings.xml"])
	if err != nil {
		return nil, err
	}
	sheet := firstSheet(files)
	if sheet == nil {
		return nil, fmt.Errorf("xlsx sheet not found")
	}
	return decodeSheet(sheet, shared)
}

func firstSheet(files map[string]*zip.File) *zip.File {
	for _, name := range []string{
		"xl/worksheets/sheet1.xml",
		"xl/worksheets/sheet.xml",
	} {
		if f := files[name]; f != nil {
			return f
		}
	}
	var first *zip.File
	for name, f := range files {
		if strings.HasPrefix(name, "xl/worksheets/sheet") && strings.HasSuffix(name, ".xml") {
			if first == nil || name < strings.ToLower(strings.ReplaceAll(first.Name, "\\", "/")) {
				first = f
			}
		}
	}
	return first
}

func decodeSharedStrings(f *zip.File) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open shared strings: %w", err)
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var out []string
	var cur strings.Builder
	inSI, inT := false, false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("shared strings: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "si":
				inSI = true
				cur.Reset()
			case "t":
				if inSI {
					inT = true
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inT = false
			case "si":
				out = append(out, cur.String())
				inSI = false
			}
		case xml.CharData:
			if inT {
				cur.Write(t)
			}
		}
	}
	return out, nil
}

func decodeSheet(f *zip.File, shared []string) ([][]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open sheet: %w", err)
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var (
		rows      [][]string
		row       []string
		rowIndex  = -1
		cellRef   string
		cellType  string
		inValue   bool
		inInlineT bool
		value     strings.Builder
	)
	flushRow := func() {
		if rowIndex < 0 {
			return
		}
		for len(rows) <= rowIndex {
			rows = append(rows, nil)
		}
		rows[rowIndex] = row
		row = nil
		rowIndex = -1
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("sheet xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				flushRow()
				rowIndex = atoiAttr(t, "r") - 1
				if rowIndex < 0 {
					rowIndex = len(rows)
				}
				row = nil
			case "c":
				cellRef = attr(t, "r")
				cellType = attr(t, "t")
				value.Reset()
			case "v":
				inValue = true
			case "t":
				inInlineT = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v":
				inValue = false
			case "t":
				inInlineT = false
			case "c":
				col := colIndex(cellRef)
				if col < 0 {
					col = len(row)
				}
				for len(row) <= col {
					row = append(row, "")
				}
				row[col] = cellText(cellType, strings.TrimSpace(value.String()), shared)
				cellRef, cellType = "", ""
			case "row":
				flushRow()
			}
		case xml.CharData:
			if inValue || inInlineT {
				value.Write(t)
			}
		}
	}
	flushRow()
	return compactRows(rows), nil
}

func cellText(cellType, raw string, shared []string) string {
	switch cellType {
	case "s":
		i, err := strconv.Atoi(raw)
		if err != nil || i < 0 || i >= len(shared) {
			return ""
		}
		return strings.TrimSpace(shared[i])
	case "inlineStr", "str":
		return raw
	case "b":
		if raw == "1" || strings.EqualFold(raw, "true") {
			return "true"
		}
		return "false"
	default:
		return raw
	}
}

func compactRows(rows [][]string) [][]string {
	width := 0
	for _, row := range rows {
		if n := lastNonEmpty(row); n > width {
			width = n
		}
	}
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		if lastNonEmpty(row) == 0 {
			continue
		}
		line := make([]string, width)
		copy(line, row)
		out = append(out, line)
	}
	return out
}

func lastNonEmpty(row []string) int {
	for i := len(row) - 1; i >= 0; i-- {
		if strings.TrimSpace(row[i]) != "" {
			return i + 1
		}
	}
	return 0
}

func colIndex(ref string) int {
	n := 0
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			if r >= 'a' && r <= 'z' {
				r = r - 'a' + 'A'
			} else {
				break
			}
		}
		n = n*26 + int(r-'A') + 1
	}
	return n - 1
}

func attr(el xml.StartElement, name string) string {
	for _, a := range el.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func atoiAttr(el xml.StartElement, name string) int {
	n, _ := strconv.Atoi(attr(el, name))
	return n
}
