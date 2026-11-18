package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/crm"
	"github.com/lunyashon/sdmp/internal/port"
)

var sdmpBitrixFields = []BitrixSourceField{
	{Key: "phone", Title: "Телефон", Group: "CDP"},
	{Key: "city", Title: "Город", Group: "CDP"},
	{Key: "region", Title: "Регион", Group: "CDP"},
	{Key: "uuid", Title: "UUID лида", Group: "CDP"},
	{Key: "web_id", Title: "Web ID", Group: "CDP"},
	{Key: "comments", Title: "Комментарий (анкета)", Group: "CDP"},
	{Key: "status_id", Title: "Стадия в Битрикс", Group: "CDP"},
	{Key: "name", Title: "Имя клиента", Group: "CDP"},
	{Key: "title", Title: "Название лида", Group: "CDP"},
	{Key: "base_label", Title: "Какая база", Group: "CDP"},
	{Key: "cold_label", Title: "Холодная база", Group: "CDP"},
	{Key: "operator", Title: "Оператор", Group: "CDP"},
	{Key: "tag", Title: "Тег", Group: "CDP"},
	{Key: "zarya", Title: "Проверка по Заре", Group: "CDP"},
	{Key: "zarya_old", Title: "Старая проверка на дубль", Group: "CDP"},
	{Key: "source_description", Title: "Дополнительно об источнике", Group: "CDP"},
	{Key: "call_url", Title: "Ссылка на запись звонка", Group: "CDP"},
}

type BitrixSourceField struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Group string `json:"group"`
}

type BitrixLeadMapView struct {
	SDMPFields      []BitrixSourceField      `json:"sdmp_fields"`
	SkorozvonFields []entity.SkorozvonMap    `json:"skorozvon_fields"`
	BitrixFields    []entity.BitrixLeadField `json:"bitrix_fields"`
	Statuses        []entity.BitrixStatus    `json:"statuses"`
	Sources         []entity.BitrixStatus    `json:"sources"`
	Maps            []entity.BitrixLeadMap   `json:"maps"`
}

type BitrixLeadMapSave struct {
	Maps            []entity.BitrixLeadMap `json:"maps"`
	SkorozvonFields []entity.SkorozvonMap  `json:"skorozvon_fields"`
}

type BitrixCatalogAPI interface {
	LeadFields(ctx context.Context) ([]crm.LeadFieldInfo, error)
	StatusList(ctx context.Context) ([]crm.StatusItem, error)
	SourceList(ctx context.Context) ([]crm.StatusItem, error)
}

type BitrixMapService struct {
	maps port.BitrixMapRepository
	sk   port.SkorozvonMapRepository
	bx24 BitrixCatalogAPI
	log  *slog.Logger
}

func NewBitrixMapService(maps port.BitrixMapRepository, sk port.SkorozvonMapRepository, bx24 BitrixCatalogAPI, log *slog.Logger) *BitrixMapService {
	if log == nil {
		log = slog.Default()
	}
	return &BitrixMapService{maps: maps, sk: sk, bx24: bx24, log: log}
}

func (s *BitrixMapService) Get(ctx context.Context) (BitrixLeadMapView, error) {
	fields, err := s.maps.ListFields(ctx)
	if err != nil {
		return BitrixLeadMapView{}, err
	}
	statuses, err := s.maps.ListStatuses(ctx, "STATUS")
	if err != nil {
		return BitrixLeadMapView{}, err
	}
	sources, err := s.maps.ListStatuses(ctx, "SOURCE")
	if err != nil {
		return BitrixLeadMapView{}, err
	}
	maps, err := s.maps.ListMaps(ctx)
	if err != nil {
		return BitrixLeadMapView{}, err
	}
	skFields := []entity.SkorozvonMap{}
	if s.sk != nil {
		skFields, err = s.sk.ListFields(ctx)
		if err != nil {
			return BitrixLeadMapView{}, err
		}
	}
	return BitrixLeadMapView{
		SDMPFields:      sdmpBitrixFields,
		SkorozvonFields: skFields,
		BitrixFields:    fields,
		Statuses:        statuses,
		Sources:         sources,
		Maps:            maps,
	}, nil
}

func (s *BitrixMapService) Save(ctx context.Context, in BitrixLeadMapSave) error {
	if err := s.maps.ReplaceMaps(ctx, in.Maps); err != nil {
		return err
	}
	if s.sk != nil && len(in.SkorozvonFields) > 0 {
		if err := s.sk.SetBitrixCodes(ctx, in.SkorozvonFields); err != nil {
			return err
		}
	}
	return nil
}

func (s *BitrixMapService) Refresh(ctx context.Context) (BitrixLeadMapView, error) {
	if s.bx24 == nil {
		return BitrixLeadMapView{}, fmt.Errorf("%w: bitrix webhook is empty", domain.ErrInvalidInput)
	}
	now := time.Now()
	fields, err := s.bx24.LeadFields(ctx)
	if err != nil {
		return BitrixLeadMapView{}, err
	}
	stored := make([]entity.BitrixLeadField, 0, len(fields))
	for _, f := range fields {
		stored = append(stored, entity.BitrixLeadField{
			Code:      f.Code,
			Title:     f.Title,
			Type:      f.Type,
			Writable:  f.Writable,
			UpdatedAt: now,
		})
	}
	if err := s.maps.ReplaceFields(ctx, stored); err != nil {
		return BitrixLeadMapView{}, err
	}

	statuses, err := s.bx24.StatusList(ctx)
	if err != nil {
		return BitrixLeadMapView{}, err
	}
	if err := s.maps.ReplaceStatuses(ctx, "STATUS", toBitrixStatuses("STATUS", statuses, now)); err != nil {
		return BitrixLeadMapView{}, err
	}

	sources, err := s.bx24.SourceList(ctx)
	if err != nil {
		return BitrixLeadMapView{}, err
	}
	if err := s.maps.ReplaceStatuses(ctx, "SOURCE", toBitrixStatuses("SOURCE", sources, now)); err != nil {
		return BitrixLeadMapView{}, err
	}

	s.log.InfoContext(ctx, "bitrix catalogs refreshed",
		"fields", len(stored),
		"statuses", len(statuses),
		"sources", len(sources),
	)
	return s.Get(ctx)
}

func toBitrixStatuses(entityID string, items []crm.StatusItem, now time.Time) []entity.BitrixStatus {
	out := make([]entity.BitrixStatus, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.StatusID)
		if id == "" {
			id = strings.TrimSpace(item.ID)
		}
		if id == "" {
			continue
		}
		sort := 0
		switch t := item.Sort.(type) {
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
				sort = n
			}
		case float64:
			sort = int(t)
		case json.Number:
			if n, err := t.Int64(); err == nil {
				sort = int(n)
			}
		}
		out = append(out, entity.BitrixStatus{
			EntityID:  entityID,
			StatusID:  id,
			Name:      strings.TrimSpace(item.Name),
			Sort:      sort,
			UpdatedAt: now,
		})
	}
	return out
}
