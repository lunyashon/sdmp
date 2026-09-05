package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/port"
)

const (
	LeadDateLayout = "02.01.2006 15:04:05"
	defaultLimit   = 10
	maxLimit       = 100
)

type LeadFilterQuery struct {
	DateFrom   string
	DateTo     string
	Sources    []int
	ColdLabels []string
	Regions    []int
	Operators  []int
	Statuses   []int
	Zarya      []string
	Phone      string
	City       string
	Tag        string
	CallResult *int32
	Limit      int
	Offset     int
}

type LeadFilterResult struct {
	Total  int
	Limit  int
	Offset int
	Leads  []entity.LeadListItem
}

type LeadService struct {
	leads port.LeadRepository
	log   *slog.Logger
}

func NewLeadService(leads port.LeadRepository, log *slog.Logger) *LeadService {
	if log == nil {
		log = slog.Default()
	}
	return &LeadService{leads: leads, log: log}
}

func (s *LeadService) Filter(ctx context.Context, q LeadFilterQuery) (LeadFilterResult, error) {
	from, err := parseLeadDate(q.DateFrom)
	if err != nil {
		return LeadFilterResult{}, fmt.Errorf("%w: date_from: %v", domain.ErrInvalidInput, err)
	}
	to, err := parseLeadDate(q.DateTo)
	if err != nil {
		return LeadFilterResult{}, fmt.Errorf("%w: date_to: %v", domain.ErrInvalidInput, err)
	}
	if to.Before(from) {
		return LeadFilterResult{}, fmt.Errorf("%w: date_to must be >= date_from", domain.ErrInvalidInput)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	filter := port.LeadListFilter{
		DateFrom:   from,
		DateTo:     to,
		Sources:    q.Sources,
		ColdLabels: q.ColdLabels,
		Regions:    q.Regions,
		Operators:  q.Operators,
		Statuses:   q.Statuses,
		Zarya:      q.Zarya,
		Phone:      normalizeListPhone(q.Phone),
		City:       strings.TrimSpace(q.City),
		Tag:        strings.TrimSpace(q.Tag),
		CallResult: q.CallResult,
		Limit:      limit,
		Offset:     offset,
	}

	items, total, err := s.leads.List(ctx, filter)
	if err != nil {
		return LeadFilterResult{}, err
	}
	return LeadFilterResult{Total: total, Limit: limit, Offset: offset, Leads: items}, nil
}

func (s *LeadService) IncomingStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadSourceCount, error) {
	from, to, err := parseStatsRange(dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	items, err := s.leads.CountBySource(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []entity.LeadSourceCount{}
	}
	return items, nil
}

func (s *LeadService) EnrichmentStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadEnrichmentStat, error) {
	from, to, err := parseStatsRange(dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	items, err := s.leads.CountEnrichmentBySource(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []entity.LeadEnrichmentStat{}
	}
	return items, nil
}

func (s *LeadService) ZaryaStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadZaryaStat, error) {
	from, to, err := parseStatsRange(dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	items, err := s.leads.CountZaryaBySource(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []entity.LeadZaryaStat{}
	}
	return items, nil
}

func (s *LeadService) Get(ctx context.Context, id int64) (entity.LeadDetail, error) {
	if id <= 0 {
		return entity.LeadDetail{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	return s.leads.Get(ctx, id)
}

func (s *LeadService) ApplyCallResult(ctx context.Context, skorozvonLeadID string, in entity.LeadCallResult) error {
	skorozvonLeadID = strings.TrimSpace(skorozvonLeadID)
	if skorozvonLeadID == "" {
		return fmt.Errorf("%w: lead.id is required", domain.ErrInvalidInput)
	}
	return s.leads.ApplyCallResult(ctx, skorozvonLeadID, in)
}

func parseLeadDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("required, format %s", LeadDateLayout)
	}
	t, err := time.ParseInLocation(LeadDateLayout, raw, clock.Location)
	if err != nil {
		return time.Time{}, fmt.Errorf("want %s", LeadDateLayout)
	}
	return t, nil
}

func parseStatsRange(dateFrom, dateTo string) (time.Time, time.Time, error) {
	from, err := parseIncomingBound(dateFrom, false)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_from: %v", domain.ErrInvalidInput, err)
	}
	to, err := parseIncomingBound(dateTo, true)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_to: %v", domain.ErrInvalidInput, err)
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_to must be >= date_from", domain.ErrInvalidInput)
	}
	return from, to, nil
}

func parseIncomingBound(raw string, endOfDay bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("required, format %s or 2006-01-02", LeadDateLayout)
	}
	for _, layout := range []string{LeadDateLayout, "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, raw, clock.Location); err == nil {
			return t, nil
		}
	}
	for _, layout := range []string{"2006-01-02", "02.01.2006"} {
		t, err := time.ParseInLocation(layout, raw, clock.Location)
		if err != nil {
			continue
		}
		if endOfDay {
			t = t.Add(24*time.Hour - time.Second)
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("want %s or 2006-01-02", LeadDateLayout)
}

func normalizeListPhone(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	switch {
	case len(digits) == 11 && digits[0] == '8':
		return "7" + digits[1:]
	case len(digits) == 10:
		return "7" + digits
	default:
		return digits
	}
}
