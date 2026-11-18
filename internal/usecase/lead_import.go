package usecase

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/lib/id"
	"github.com/lunyashon/sdmp/internal/lib/phone"
	"github.com/lunyashon/sdmp/internal/lib/xlsx"
)

const (
	ImportMaxBytes   = 8 << 20
	importMaxBytes   = ImportMaxBytes
	importMaxColumns = 80
	importMaxRows    = 20000
	importPreviewN   = 8
	ImportBatchSize  = 500
	importWorkers    = 8
)

type LeadImportField struct {
	ID     string `json:"id"`
	Column string `json:"column"`
	Label  string `json:"label"`
}

type LeadImportColumn struct {
	Index   int      `json:"index"`
	Name    string   `json:"name"`
	Samples []string `json:"samples"`
}

type LeadImportPreview struct {
	Filename  string             `json:"filename"`
	RowCount  int                `json:"row_count"`
	Truncated bool               `json:"truncated"`
	Columns   []LeadImportColumn `json:"columns"`
	Rows      [][]string         `json:"rows"`
	Fields    []LeadImportField  `json:"fields"`
	Suggested map[string]string  `json:"suggested"`
}

var leadRowImportFields = []LeadImportField{
	{ID: "uuid", Column: "uuid", Label: "UUID"},
	{ID: "phone", Column: "phone", Label: "Телефон"},
	{ID: "phone_valid", Column: "phone_valid", Label: "Телефон валиден"},
	{ID: "source_id", Column: "source_id", Label: "Источник (source_id)"},
	{ID: "child_source_id", Column: "child_source_id", Label: "Подисточник (child_source_id)"},
	{ID: "region_id", Column: "region_id", Label: "Регион (region_id)"},
	{ID: "region_source", Column: "region_source", Label: "Источник региона"},
	{ID: "city", Column: "city", Label: "Город"},
	{ID: "operator_id", Column: "operator_id", Label: "Оператор (operator_id)"},
	{ID: "operator_source", Column: "operator_source", Label: "Источник оператора"},
	{ID: "operator_raw", Column: "operator_raw", Label: "Оператор (сырой)"},
	{ID: "web_id", Column: "web_id", Label: "Web ID"},
	{ID: "tag", Column: "tag", Label: "Тег"},
	{ID: "call_result", Column: "call_result", Label: "Результат звонка (id)"},
	{ID: "call_result_name", Column: "call_result_name", Label: "Результат звонка"},
	{ID: "call_id", Column: "call_id", Label: "ID звонка"},
	{ID: "call_url", Column: "call_url", Label: "URL записи"},
	{ID: "call_last_at", Column: "call_last_at", Label: "Дата звонка"},
	{ID: "project_num", Column: "project_num", Label: "№ проекта"},
	{ID: "call_form", Column: "call_form", Label: "Анкета (сырьё)"},
	{ID: "status_id", Column: "status_id", Label: "Стадия (status_id)"},
	{ID: "s3_key", Column: "s3_key", Label: "s3_key"},
	{ID: "archived_at", Column: "archived_at", Label: "Дата архивации"},
	{ID: "created_at", Column: "created_at", Label: "Дата создания"},
	{ID: "updated_at", Column: "updated_at", Label: "Дата обновления"},
	{ID: "legacy_fields", Column: "legacy_fields", Label: "legacy_fields"},
	{ID: "bitrix_lead_id", Column: "bitrix_lead_id", Label: "ID лида Б24"},
	{ID: "cold_label", Column: "cold_label", Label: "Холодная база"},
	{ID: "base_label", Column: "base_label", Label: "Какая база"},
	{ID: "source_description", Column: "source_description", Label: "Дополнительно об источнике"},
	{ID: "skorozvon_lead_id", Column: "skorozvon_lead_id", Label: "ID лида Скорозвон"},
	{ID: "skorozvon_anketa_id", Column: "skorozvon_anketa_id", Label: "ID анкеты Скорозвон"},
	{ID: "via_us", Column: "via_us", Label: "Из Скорозвона строка"},
}

func (s *LeadService) PreviewImport(filename string, body []byte) (LeadImportPreview, error) {
	return PreviewLeadImport(filename, body)
}

func PreviewLeadImport(filename string, body []byte) (LeadImportPreview, error) {
	if len(body) == 0 {
		return LeadImportPreview{}, fmt.Errorf("%w: empty file", domain.ErrInvalidInput)
	}
	if len(body) > importMaxBytes {
		return LeadImportPreview{}, fmt.Errorf("%w: file too large, max %d bytes", domain.ErrInvalidInput, importMaxBytes)
	}

	kind := importKind(filename, body)
	var table [][]string
	var err error
	switch kind {
	case "xlsx":
		table, err = xlsx.Decode(body)
	case "csv":
		table, err = decodeCSV(body)
	default:
		return LeadImportPreview{}, fmt.Errorf("%w: ожидается xlsx или csv", domain.ErrInvalidInput)
	}
	if err != nil {
		return LeadImportPreview{}, fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	if len(table) == 0 {
		return LeadImportPreview{}, fmt.Errorf("%w: в файле нет строк", domain.ErrInvalidInput)
	}

	header := append([]string(nil), table[0]...)
	data := append([][]string(nil), table[1:]...)
	truncated := false
	if len(data) > importMaxRows {
		data = data[:importMaxRows]
		truncated = true
	}
	if len(header) > importMaxColumns {
		header = header[:importMaxColumns]
		for i, row := range data {
			if len(row) > importMaxColumns {
				data[i] = row[:importMaxColumns]
			}
		}
	}

	width := len(header)
	for i := range header {
		header[i] = strings.TrimSpace(header[i])
		if header[i] == "" {
			header[i] = fmt.Sprintf("Колонка %d", i+1)
		}
	}
	for i, row := range data {
		line := make([]string, width)
		for j := 0; j < width && j < len(row); j++ {
			line[j] = strings.TrimSpace(row[j])
		}
		data[i] = line
	}

	previewN := importPreviewN
	if len(data) < previewN {
		previewN = len(data)
	}
	preview := make([][]string, previewN)
	copy(preview, data[:previewN])

	cols := make([]LeadImportColumn, width)
	for i, name := range header {
		samples := make([]string, 0, 3)
		for _, row := range preview {
			if i >= len(row) {
				continue
			}
			v := strings.TrimSpace(row[i])
			if v == "" {
				continue
			}
			samples = append(samples, v)
			if len(samples) == 3 {
				break
			}
		}
		cols[i] = LeadImportColumn{Index: i, Name: name, Samples: samples}
	}

	name := strings.TrimSpace(path.Base(filename))
	if name == "" || name == "." {
		name = "import." + kind
	}

	return LeadImportPreview{
		Filename:  name,
		RowCount:  len(data),
		Truncated: truncated,
		Columns:   cols,
		Rows:      data,
		Fields:    append([]LeadImportField(nil), leadRowImportFields...),
		Suggested: suggestImportMapping(header),
	}, nil
}

func importKind(filename string, body []byte) string {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".xlsx":
		return "xlsx"
	case ".csv", ".txt":
		return "csv"
	case ".xls":
		return ""
	}
	if len(body) >= 2 && body[0] == 'P' && body[1] == 'K' {
		return "xlsx"
	}
	return "csv"
}

func decodeCSV(raw []byte) ([][]string, error) {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(raw) {
		raw = decodeCP1251(raw)
	}
	r := csv.NewReader(bytes.NewReader(raw))
	r.Comma = csvDelimiter(raw)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = false
	return r.ReadAll()
}

func csvDelimiter(raw []byte) rune {
	line := raw
	if i := bytes.IndexByte(raw, '\n'); i >= 0 {
		line = raw[:i]
	}
	line = bytes.TrimRight(line, "\r")
	commas := bytes.Count(line, []byte{','})
	semis := bytes.Count(line, []byte{';'})
	tabs := bytes.Count(line, []byte{'\t'})
	switch {
	case tabs > commas && tabs > semis:
		return '\t'
	case semis > commas:
		return ';'
	default:
		return ','
	}
}

func decodeCP1251(raw []byte) []byte {
	var b strings.Builder
	b.Grow(len(raw))
	for _, c := range raw {
		if c < 0x80 {
			b.WriteByte(c)
			continue
		}
		if r, ok := cp1251[c]; ok {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('?')
	}
	return []byte(b.String())
}

var cp1251 = map[byte]rune{
	0xA8: 'Ё', 0xB8: 'ё',
	0xC0: 'А', 0xC1: 'Б', 0xC2: 'В', 0xC3: 'Г', 0xC4: 'Д', 0xC5: 'Е', 0xC6: 'Ж', 0xC7: 'З',
	0xC8: 'И', 0xC9: 'Й', 0xCA: 'К', 0xCB: 'Л', 0xCC: 'М', 0xCD: 'Н', 0xCE: 'О', 0xCF: 'П',
	0xD0: 'Р', 0xD1: 'С', 0xD2: 'Т', 0xD3: 'У', 0xD4: 'Ф', 0xD5: 'Х', 0xD6: 'Ц', 0xD7: 'Ч',
	0xD8: 'Ш', 0xD9: 'Щ', 0xDA: 'Ъ', 0xDB: 'Ы', 0xDC: 'Ь', 0xDD: 'Э', 0xDE: 'Ю', 0xDF: 'Я',
	0xE0: 'а', 0xE1: 'б', 0xE2: 'в', 0xE3: 'г', 0xE4: 'д', 0xE5: 'е', 0xE6: 'ж', 0xE7: 'з',
	0xE8: 'и', 0xE9: 'й', 0xEA: 'к', 0xEB: 'л', 0xEC: 'м', 0xED: 'н', 0xEE: 'о', 0xEF: 'п',
	0xF0: 'р', 0xF1: 'с', 0xF2: 'т', 0xF3: 'у', 0xF4: 'ф', 0xF5: 'х', 0xF6: 'ц', 0xF7: 'ч',
	0xF8: 'ш', 0xF9: 'щ', 0xFA: 'ъ', 0xFB: 'ы', 0xFC: 'ь', 0xFD: 'э', 0xFE: 'ю', 0xFF: 'я',
}

func suggestImportMapping(headers []string) map[string]string {
	used := map[string]bool{}
	out := map[string]string{}
	for i, h := range headers {
		key := normalizeImportName(h)
		if key == "" {
			continue
		}
		for _, f := range leadRowImportFields {
			if used[f.ID] {
				continue
			}
			if key == normalizeImportName(f.ID) || key == normalizeImportName(f.Column) || key == normalizeImportName(f.Label) || aliasMatch(key, f.ID) {
				used[f.ID] = true
				out[fmt.Sprintf("%d", i)] = f.ID
				break
			}
		}
	}
	return out
}

func aliasMatch(key, fieldID string) bool {
	aliases := map[string][]string{
		"phone":              {"тел", "телефон", "мобильный", "номер"},
		"city":               {"город", "наспункт", "населенныйпункт"},
		"uuid":               {"uuid", "uid", "externalid", "внешнийid"},
		"tag":                {"тег", "teg", "метка"},
		"cold_label":         {"холоднаябаза", "база", "baza"},
		"base_label":         {"какаябаза", "kakaya"},
		"source_description": {"sourcedescription", "описаниеисточника", "дополнительнообисточнике"},
		"bitrix_lead_id":     {"идлидаб24", "bitrixid", "b24id"},
		"skorozvon_lead_id":  {"идлидаскорозвон", "skorozvonid"},
		"created_at":         {"датасоздания", "created", "datecreate", "дата"},
		"via_us":             {"изскорозвонастрока", "viaus"},
		"status_id":          {"стадия", "status", "статус"},
		"web_id":             {"webid", "вебid"},
		"project_num":        {"проект", "номерпроекта"},
		"call_result_name":   {"результатзвонка", "callresult"},
		"source_id":          {"источник", "source"},
		"child_source_id":    {"подисточник", "childsource"},
		"region_id":          {"регион", "region"},
		"operator_id":        {"оператор", "operator"},
		"operator_raw":       {"операторсырой"},
	}
	for _, a := range aliases[fieldID] {
		if key == a {
			return true
		}
	}
	return false
}

func normalizeImportName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type LeadImportInput struct {
	SourceID int
	Mapping  map[string]string
	Rows     [][]string
}

type LeadImportRowError struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

type LeadImportResult struct {
	Queued int                  `json:"queued"`
	Failed int                  `json:"failed"`
	Errors []LeadImportRowError `json:"errors"`
}

func (s *LeadService) Import(ctx context.Context, in LeadImportInput) (LeadImportResult, error) {
	if s.sources == nil {
		return LeadImportResult{}, fmt.Errorf("import sources not configured")
	}
	if err := s.resolveImportSource(ctx, in.SourceID); err != nil {
		return LeadImportResult{}, err
	}
	if len(in.Rows) == 0 {
		return LeadImportResult{}, fmt.Errorf("%w: rows required", domain.ErrInvalidInput)
	}
	if len(in.Rows) > ImportBatchSize {
		return LeadImportResult{}, fmt.Errorf("%w: too many rows, max %d", domain.ErrInvalidInput, ImportBatchSize)
	}
	colMap, err := parseImportMapping(in.Mapping)
	if err != nil {
		return LeadImportResult{}, err
	}

	jobs := make(chan int)
	var queued atomic.Int32
	var failed atomic.Int32
	var mu sync.Mutex
	errs := make([]LeadImportRowError, 0)

	workers := importWorkers
	if n := len(in.Rows); n < workers {
		workers = n
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if ctx.Err() != nil {
					failed.Add(1)
					continue
				}
				row, err := buildImportRow(colMap, in.Rows[idx], in.SourceID)
				if err != nil {
					failed.Add(1)
					mu.Lock()
					if len(errs) < 20 {
						errs = append(errs, LeadImportRowError{Index: idx, Error: err.Error()})
					}
					mu.Unlock()
					continue
				}
				if err := s.leads.InsertImport(ctx, row); err != nil {
					failed.Add(1)
					mu.Lock()
					if len(errs) < 20 {
						errs = append(errs, LeadImportRowError{Index: idx, Error: err.Error()})
					}
					mu.Unlock()
					continue
				}
				queued.Add(1)
			}
		}()
	}
	for i := range in.Rows {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return LeadImportResult{}, ctx.Err()
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()

	return LeadImportResult{
		Queued: int(queued.Load()),
		Failed: int(failed.Load()),
		Errors: errs,
	}, nil
}

func parseImportMapping(raw map[string]string) (map[int]string, error) {
	allowed := map[string]bool{}
	for _, f := range leadRowImportFields {
		allowed[f.ID] = true
	}
	out := map[int]string{}
	used := map[string]bool{}
	for k, fieldID := range raw {
		fieldID = strings.TrimSpace(fieldID)
		if fieldID == "" {
			continue
		}
		if !allowed[fieldID] {
			return nil, fmt.Errorf("%w: unknown field %s", domain.ErrInvalidInput, fieldID)
		}
		if used[fieldID] {
			return nil, fmt.Errorf("%w: field %s mapped twice", domain.ErrInvalidInput, fieldID)
		}
		idx, err := strconv.Atoi(strings.TrimSpace(k))
		if err != nil || idx < 0 {
			return nil, fmt.Errorf("%w: bad mapping column %s", domain.ErrInvalidInput, k)
		}
		used[fieldID] = true
		out[idx] = fieldID
	}
	if _, ok := used["phone"]; !ok {
		return nil, fmt.Errorf("%w: нужно сопоставить колонку с полем phone", domain.ErrInvalidInput)
	}
	return out, nil
}

func (s *LeadService) resolveImportSource(ctx context.Context, sourceID int) error {
	if sourceID <= 0 {
		return fmt.Errorf("%w: source_id required", domain.ErrInvalidInput)
	}
	src, err := s.sources.GetByID(ctx, sourceID)
	if err != nil {
		return err
	}
	if src.ParentID != nil {
		return fmt.Errorf("%w: source_id must be a parent source", domain.ErrInvalidInput)
	}
	return nil
}

func buildImportRow(mapping map[int]string, raw []string, sourceID int) (entity.LeadImportRow, error) {
	values := map[string]any{}
	for idx, fieldID := range mapping {
		if idx < 0 || idx >= len(raw) {
			continue
		}
		v := strings.TrimSpace(raw[idx])
		if v == "" {
			continue
		}
		parsed, err := parseImportValue(fieldID, v)
		if err != nil {
			return entity.LeadImportRow{}, err
		}
		if parsed != nil {
			values[fieldID] = parsed
		}
	}
	phoneRaw, _ := values["phone"].(string)
	phoneRaw = strings.TrimSpace(phoneRaw)
	if phoneRaw == "" {
		return entity.LeadImportRow{}, fmt.Errorf("пустой телефон")
	}
	phoneVal := phoneRaw
	phoneValid := false
	if norm, ok := phone.Normalize(phoneRaw); ok {
		phoneVal = norm
		phoneValid = true
	}
	uuidVal, _ := values["uuid"].(string)
	uuidVal = strings.TrimSpace(uuidVal)
	if uuidVal == "" {
		u, err := id.NewUUID()
		if err != nil {
			return entity.LeadImportRow{}, err
		}
		uuidVal = u
	}
	if _, ok := values["phone_valid"]; !ok {
		values["phone_valid"] = phoneValid
	}
	delete(values, "child_source_id")
	values["source_id"] = sourceID
	fields := map[string]any{"PHONE": phoneVal}
	if tag, ok := values["tag"].(string); ok && strings.TrimSpace(tag) != "" {
		fields["UF_CRM_1766149282"] = strings.TrimSpace(tag)
	}
	rawPayload, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return entity.LeadImportRow{}, err
	}
	return entity.LeadImportRow{
		UUID:       uuidVal,
		Phone:      phoneVal,
		PhoneValid: phoneValid,
		SourceID:   sourceID,
		Payload:    string(rawPayload),
		Values:     values,
	}, nil
}

func parseImportValue(fieldID, raw string) (any, error) {
	switch fieldID {
	case "phone_valid", "via_us":
		return parseImportBool(raw)
	case "source_id", "child_source_id", "region_id", "operator_id":
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: не число", fieldID)
		}
		return n, nil
	case "region_source", "operator_source", "status_id":
		n, err := strconv.ParseInt(raw, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("%s: не число", fieldID)
		}
		return int16(n), nil
	case "call_result", "skorozvon_anketa_id":
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: не число", fieldID)
		}
		return n, nil
	case "call_last_at", "archived_at", "created_at", "updated_at":
		t, err := parseImportTime(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", fieldID, err)
		}
		return t, nil
	default:
		return raw, nil
	}
}

func parseImportBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "t", "yes", "да", "истина":
		return true, nil
	case "0", "false", "f", "no", "нет", "ложь":
		return false, nil
	default:
		return false, fmt.Errorf("ожидалось да/нет")
	}
}

func parseImportTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	layouts := []string{
		LeadDateLayout,
		"02.01.2006",
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, clock.Location); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("не дата")
}
