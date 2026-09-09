package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/lib/xlsx"
	"github.com/lunyashon/sdmp/internal/port"
)

const (
	exportMaxRows = 50000
	exportMaxIDs  = 10000
	xlsxType      = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

type LeadExportFile struct {
	Filename    string
	ContentType string
	Body        []byte
}

type exportColumn struct {
	id    string
	label string
	value func(entity.LeadListItem) string
}

var exportColumnCatalog = []exportColumn{
	{id: "id", label: "ID", value: func(r entity.LeadListItem) string { return strconv.FormatInt(r.ID, 10) }},
	{id: "uuid", label: "UUID", value: func(r entity.LeadListItem) string { return r.UUID }},
	{id: "date", label: "Дата создания", value: func(r entity.LeadListItem) string { return formatExportTime(r.DateCreate) }},
	{id: "source", label: "Источник", value: func(r entity.LeadListItem) string { return r.Source }},
	{id: "childSource", label: "Подисточник", value: func(r entity.LeadListItem) string { return r.ChildSource }},
	{id: "baza", label: "Холодная база", value: func(r entity.LeadListItem) string { return r.ColdLabel }},
	{id: "kakaya", label: "Какая база", value: func(r entity.LeadListItem) string { return r.BaseLabel }},
	{id: "sourceDesc", label: "Source description", value: func(r entity.LeadListItem) string { return r.SourceDescription }},
	{id: "region", label: "Регион", value: func(r entity.LeadListItem) string { return r.Region }},
	{id: "city", label: "Город", value: func(r entity.LeadListItem) string { return r.City }},
	{id: "operator", label: "Оператор", value: func(r entity.LeadListItem) string { return r.Operator }},
	{id: "stage", label: "Стадия", value: func(r entity.LeadListItem) string { return r.Stage }},
	{id: "viaUs", label: "Из скорозвона строка", value: func(r entity.LeadListItem) string {
		if !r.ViaUs {
			return "Создалась"
		}
		return "Обновилось"
	}},
	{id: "zarya", label: "Проверка Зари", value: func(r entity.LeadListItem) string { return r.Zarya }},
	{id: "callResult", label: "Результат звонка", value: func(r entity.LeadListItem) string {
		if strings.TrimSpace(r.CallResultName) != "" {
			return r.CallResultName
		}
		if r.CallResult != nil {
			return strconv.FormatInt(*r.CallResult, 10)
		}
		return ""
	}},
	{id: "upk", label: "ЮПК", value: func(r entity.LeadListItem) string { return r.UPK }},
	{id: "phone", label: "Телефон", value: func(r entity.LeadListItem) string { return r.Phone }},
	{id: "teg", label: "Тег (вход)", value: func(r entity.LeadListItem) string { return r.Tag }},
	{id: "webId", label: "Web ID", value: func(r entity.LeadListItem) string { return r.WebID }},
	{id: "projectNum", label: "№ проекта", value: func(r entity.LeadListItem) string { return r.ProjectNum }},
}

func (s *LeadService) ExportXLSX(ctx context.Context, q LeadFilterQuery, columns []string) (LeadExportFile, error) {
	cols, err := resolveExportColumns(columns)
	if err != nil {
		return LeadExportFile{}, err
	}
	ids, err := normalizeExportIDs(q.IDs)
	if err != nil {
		return LeadExportFile{}, err
	}

	from, err := parseLeadDate(q.DateFrom)
	if err != nil {
		return LeadExportFile{}, fmt.Errorf("%w: date_from: %v", domain.ErrInvalidInput, err)
	}
	to, err := parseLeadDate(q.DateTo)
	if err != nil {
		return LeadExportFile{}, fmt.Errorf("%w: date_to: %v", domain.ErrInvalidInput, err)
	}
	if to.Before(from) {
		return LeadExportFile{}, fmt.Errorf("%w: date_to must be >= date_from", domain.ErrInvalidInput)
	}

	filter := port.LeadListFilter{
		DateFrom:     from,
		DateTo:       to,
		Sources:      q.Sources,
		ChildSources: q.ChildSources,
		ColdLabels:   q.ColdLabels,
		Regions:      q.Regions,
		Operators:    q.Operators,
		Statuses:     q.Statuses,
		Zarya:        q.Zarya,
		Phone:        normalizeListPhone(q.Phone),
		City:         strings.TrimSpace(q.City),
		Tag:          strings.TrimSpace(q.Tag),
		CallResult:   q.CallResult,
		ViaUs:        q.ViaUs,
		IDs:          ids,
		Limit:        exportMaxRows,
		Offset:       0,
	}
	if n := len(ids); n > 0 && n < filter.Limit {
		filter.Limit = n
	}

	items, total, err := s.leads.List(ctx, filter)
	if err != nil {
		return LeadExportFile{}, err
	}
	if len(ids) == 0 && total > exportMaxRows {
		return LeadExportFile{}, fmt.Errorf("%w: too many rows (%d), max %d", domain.ErrInvalidInput, total, exportMaxRows)
	}

	rows := make([][]string, 0, len(items)+1)
	header := make([]string, len(cols))
	for i, c := range cols {
		header[i] = c.label
	}
	rows = append(rows, header)
	for _, item := range items {
		line := make([]string, len(cols))
		for i, c := range cols {
			line[i] = c.value(item)
		}
		rows = append(rows, line)
	}

	body, err := xlsx.Encode("Лиды", rows)
	if err != nil {
		return LeadExportFile{}, err
	}
	return LeadExportFile{
		Filename:    "leads_" + clock.Now().Format("2006-01-02") + ".xlsx",
		ContentType: xlsxType,
		Body:        body,
	}, nil
}

func resolveExportColumns(ids []string) ([]exportColumn, error) {
	byID := make(map[string]exportColumn, len(exportColumnCatalog))
	for _, c := range exportColumnCatalog {
		byID[c.id] = c
	}
	out := make([]exportColumn, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		c, ok := byID[id]
		if !ok {
			continue
		}
		seen[id] = true
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: columns are required", domain.ErrInvalidInput)
	}
	return out, nil
}

func normalizeExportIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	out := make([]int64, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: ids are required", domain.ErrInvalidInput)
	}
	if len(out) > exportMaxIDs {
		return nil, fmt.Errorf("%w: too many ids, max %d", domain.ErrInvalidInput, exportMaxIDs)
	}
	return out, nil
}

func formatExportTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, clock.Location).Format(LeadDateLayout)
}
