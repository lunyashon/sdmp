package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/lib/crm"
	"github.com/lunyashon/sdmp/internal/port"
)

var skorozvonMapsHours = []int{12, 19}

func bitrixCodeForSkorozvonName(name string) string {
	return crm.BitrixCodeForSkorozvonName(name)
}

func (s *WorkerService) WithSkorozvonMaps(maps port.SkorozvonMapRepository) *WorkerService {
	s.maps = maps
	return s
}

func (s *WorkerService) loopSkorozvonMaps(ctx context.Context) {
	if err := s.SyncSkorozvonMaps(ctx); err != nil {
		s.log.ErrorContext(ctx, "skorozvon maps sync failed", "error", err)
	}
	for {
		waitFor := time.Until(nextSkorozvonMapsAt(clock.Now()))
		if waitFor < 0 {
			waitFor = 0
		}
		if err := wait(ctx, waitFor); err != nil {
			return
		}
		if err := s.SyncSkorozvonMaps(ctx); err != nil {
			s.log.ErrorContext(ctx, "skorozvon maps sync failed", "error", err)
		}
	}
}

func (s *WorkerService) SyncSkorozvonMaps(ctx context.Context) error {
	if s.skorozvon == nil || s.maps == nil {
		return nil
	}
	fields, err := s.skorozvon.ListCustomFields(ctx)
	if err != nil {
		return err
	}
	rows := flattenSkorozvonMaps(fields, clock.Now())
	if len(rows) == 0 {
		return fmt.Errorf("skorozvon custom_fields empty")
	}
	if err := s.maps.ReplaceAll(ctx, rows); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "skorozvon maps synced", "fields", len(fields), "rows", len(rows))
	return nil
}

func flattenSkorozvonMaps(fields []port.SkorozvonCustomField, now time.Time) []entity.SkorozvonMap {
	out := make([]entity.SkorozvonMap, 0, len(fields))
	for _, f := range fields {
		if f.ID == 0 {
			continue
		}
		name := strings.TrimSpace(f.Name)
		if name == "" {
			name = fmt.Sprintf("FIELD_%d", f.ID)
		}
		out = append(out, entity.SkorozvonMap{
			FieldID:    f.ID,
			OptionID:   0,
			Name:       name,
			BitrixCode: bitrixCodeForSkorozvonName(name),
			FieldType:  strings.TrimSpace(f.FieldType),
			LeadType:   strings.TrimSpace(f.LeadType),
			UpdatedAt:  now,
		})
		for _, opt := range f.Options {
			if opt.ID == 0 {
				continue
			}
			label := strings.TrimSpace(opt.Label)
			if label == "" {
				label = fmt.Sprintf("%d", opt.ID)
			}
			out = append(out, entity.SkorozvonMap{
				FieldID:   f.ID,
				OptionID:  opt.ID,
				Name:      label,
				FieldType: strings.TrimSpace(f.FieldType),
				LeadType:  strings.TrimSpace(f.LeadType),
				UpdatedAt: now,
			})
		}
	}
	return out
}

func nextSkorozvonMapsAt(now time.Time) time.Time {
	now = now.In(clock.Location)
	for _, hour := range skorozvonMapsHours {
		at := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, clock.Location)
		if at.After(now) {
			return at
		}
	}
	next := now.AddDate(0, 0, 1)
	return time.Date(next.Year(), next.Month(), next.Day(), skorozvonMapsHours[0], 0, 0, 0, clock.Location)
}
