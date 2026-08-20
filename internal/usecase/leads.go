package usecase

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

const maxLeadMonths = 24

type LeadFilterQuery struct {
	SourceID string
	DateFrom string
	DateTo   string
}

type LeadFilterResult struct {
	SourceID      string
	DateFrom      string
	DateTo        string
	MonthsLoaded  []string
	MonthsMissing []string
	Leads         []entity.LeadRecord
}

type LeadService struct {
	storage port.S3
	sources port.SourceRepository
	log     *slog.Logger
}

func NewLeadService(storage port.S3, sources port.SourceRepository, log *slog.Logger) *LeadService {
	if log == nil {
		log = slog.Default()
	}
	return &LeadService{storage: storage, sources: sources, log: log}
}

func (s *LeadService) Filter(ctx context.Context, q LeadFilterQuery) (LeadFilterResult, error) {
	from, err := parseQueryDate(q.DateFrom, false)
	if err != nil {
		return LeadFilterResult{}, fmt.Errorf("%w: date_from: %v", domain.ErrInvalidInput, err)
	}
	to, err := parseQueryDate(q.DateTo, true)
	if err != nil {
		return LeadFilterResult{}, fmt.Errorf("%w: date_to: %v", domain.ErrInvalidInput, err)
	}
	if to.Before(from) {
		return LeadFilterResult{}, fmt.Errorf("%w: date_to must be >= date_from", domain.ErrInvalidInput)
	}

	code, err := s.resolveSourceCode(ctx, q.SourceID)
	if err != nil {
		return LeadFilterResult{}, err
	}

	months, err := monthsInRange(from, to)
	if err != nil {
		return LeadFilterResult{}, err
	}

	result := LeadFilterResult{
		SourceID:      code,
		DateFrom:      q.DateFrom,
		DateTo:        q.DateTo,
		MonthsLoaded:  make([]string, 0, len(months)),
		MonthsMissing: make([]string, 0),
		Leads:         make([]entity.LeadRecord, 0),
	}

	for _, m := range months {
		key := monthCSVKey(code, m)
		leads, err := s.readMonth(ctx, key, from, to)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				result.MonthsMissing = append(result.MonthsMissing, m.label())
				continue
			}
			return LeadFilterResult{}, err
		}
		result.MonthsLoaded = append(result.MonthsLoaded, m.label())
		result.Leads = append(result.Leads, leads...)
	}

	return result, nil
}

func (s *LeadService) resolveSourceCode(_ context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w: source_id is required", domain.ErrInvalidInput)
	}
	// source_id в query — код Bitrix / папка в S3 (UC_* или 141/143/144), не PK таблицы source.
	return raw, nil
}

func (s *LeadService) readMonth(ctx context.Context, key string, from, to time.Time) ([]entity.LeadRecord, error) {
	obj, err := s.storage.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer obj.Body.Close()

	reader := csv.NewReader(obj.Body)
	reader.ReuseRecord = true
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return []entity.LeadRecord{}, nil
		}
		return nil, fmt.Errorf("read csv header %s: %w", key, err)
	}
	idx := csvIndex(header)
	if idx.dateCreate < 0 {
		return nil, fmt.Errorf("%w: csv %s has no date_create column", domain.ErrInvalidInput, key)
	}

	out := make([]entity.LeadRecord, 0, 1024)
	for {
		rec, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv %s: %w", key, err)
		}
		rawDate := field(rec, idx.dateCreate)
		created, err := parseCSVDate(rawDate)
		if err != nil {
			continue
		}
		if created.Before(from) || created.After(to) {
			continue
		}
		out = append(out, entity.LeadRecord{
			Phone:      field(rec, idx.phone),
			SourceID:   field(rec, idx.sourceID),
			Region:     field(rec, idx.region),
			Operator:   field(rec, idx.operator),
			City:       field(rec, idx.city),
			Name:       strings.TrimSpace(field(rec, idx.name)),
			OwnerID:    field(rec, idx.ownerID),
			DateCreate: rawDate,
		})
	}
	return out, nil
}

type yearMonth struct {
	year  int
	month time.Month
}

func (m yearMonth) label() string {
	return fmt.Sprintf("%04d-%02d", m.year, m.month)
}

func monthsInRange(from, to time.Time) ([]yearMonth, error) {
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location())
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, to.Location())
	out := make([]yearMonth, 0, 8)
	for !cur.After(end) {
		out = append(out, yearMonth{year: cur.Year(), month: cur.Month()})
		if len(out) > maxLeadMonths {
			return nil, fmt.Errorf("%w: range exceeds %d months", domain.ErrInvalidInput, maxLeadMonths)
		}
		cur = cur.AddDate(0, 1, 0)
	}
	return out, nil
}

func monthCSVKey(code string, m yearMonth) string {
	file := fmt.Sprintf("%s-%04d%02d.csv", code, m.year, m.month)
	return port.JoinKey(code, fmt.Sprintf("%04d", m.year), fmt.Sprintf("%02d", m.month), "csv", file)
}

type csvCols struct {
	phone, sourceID, region, operator, city, name, ownerID, dateCreate int
}

func csvIndex(header []string) csvCols {
	idx := csvCols{-1, -1, -1, -1, -1, -1, -1, -1}
	for i, col := range header {
		switch strings.ToLower(strings.TrimSpace(col)) {
		case "phone":
			idx.phone = i
		case "source_id":
			idx.sourceID = i
		case "region":
			idx.region = i
		case "operator":
			idx.operator = i
		case "city":
			idx.city = i
		case "name":
			idx.name = i
		case "owner_id":
			idx.ownerID = i
		case "date_create":
			idx.dateCreate = i
		}
	}
	return idx
}

func field(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}
	return rec[i]
}

func parseQueryDate(raw string, endOfDay bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	var parsed time.Time
	var err error
	for _, layout := range layouts {
		parsed, err = time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			if layout == "2006-01-02" && endOfDay {
				return parsed.Add(24*time.Hour - time.Nanosecond), nil
			}
			return parsed, nil
		}
	}
	return time.Time{}, err
}

func parseCSVDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q", raw)
}
