package usecase

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

func (s *LeadService) enrichSnapGeo(ctx context.Context, snap *entity.LeadFromSkorozvon) {
	if snap == nil {
		return
	}
	cityClient, cityOther, regionClient, regionOther := s.geoFromCustomFields(ctx, snap.CustomFields)
	city := firstNonEmptyTrim(cityClient, cityOther, snap.City)
	regionName := firstNonEmptyTrim(regionClient, regionOther, snap.Region)
	snap.City = city
	snap.Region = regionName
	if regionName == "" || s.dicts == nil {
		return
	}
	id, ok, err := s.dicts.ResolveRegion(ctx, regionName)
	if err != nil {
		s.log.Warn("resolve skorozvon region failed", "region", regionName, "error", err)
		return
	}
	if !ok {
		s.log.Info("skorozvon region not mapped", "region", regionName)
		return
	}
	snap.RegionID = &id
	snap.RegionSource = entity.GeoSourceSupplier
}

func (s *LeadService) geoFromCustomFields(ctx context.Context, fields []entity.SkorozvonCustomValue) (cityClient, cityOther, regionClient, regionOther string) {
	if s.maps == nil {
		return "", "", "", ""
	}
	for _, field := range fields {
		if field.FieldID == 0 {
			continue
		}
		meta, ok, err := s.maps.Get(ctx, field.FieldID, 0)
		if err != nil {
			s.log.Warn("skorozvon_maps lookup failed", "field_id", field.FieldID, "error", err)
			continue
		}
		if !ok {
			continue
		}
		kind, client := classifySkorozvonGeo(meta)
		if kind == "" {
			continue
		}
		label := s.firstCustomFieldLabel(ctx, field)
		if label == "" {
			continue
		}
		switch {
		case kind == "city" && client && cityClient == "":
			cityClient = label
		case kind == "city" && cityOther == "":
			cityOther = label
		case kind == "region" && client && regionClient == "":
			regionClient = label
		case kind == "region" && regionOther == "":
			regionOther = label
		}
	}
	return cityClient, cityOther, regionClient, regionOther
}

func (s *LeadService) firstCustomFieldLabel(ctx context.Context, field entity.SkorozvonCustomValue) string {
	for _, part := range decodeCustomFieldParts(field.Raw) {
		if part.text != "" {
			return part.text
		}
		if part.optionID == 0 {
			continue
		}
		opt, ok, err := s.maps.Get(ctx, field.FieldID, part.optionID)
		if err != nil || !ok {
			continue
		}
		if name := strings.TrimSpace(opt.Name); name != "" {
			return name
		}
	}
	return ""
}

func classifySkorozvonGeo(meta entity.SkorozvonMap) (kind string, client bool) {
	name := strings.ToLower(strings.TrimSpace(meta.Name))
	ft := strings.ToLower(strings.TrimSpace(meta.FieldType))
	client = strings.Contains(name, "клиент")
	switch {
	case ft == "region" || strings.Contains(name, "регион"):
		return "region", client
	case ft == "city" || strings.Contains(name, "город"):
		return "city", client
	default:
		return "", false
	}
}

type customFieldPart struct {
	optionID int64
	text     string
}

func decodeCustomFieldParts(raw []byte) []customFieldPart {
	raw = bytesTrim(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var parts []customFieldPart
	if raw[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil
		}
		for _, item := range items {
			parts = append(parts, decodeCustomFieldPart(item)...)
		}
		return parts
	}
	return decodeCustomFieldPart(raw)
}

func decodeCustomFieldPart(raw json.RawMessage) []customFieldPart {
	raw = bytesTrim(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			return []customFieldPart{{optionID: id}}
		}
		return []customFieldPart{{text: s}}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		id := int64(n)
		if id > 0 {
			return []customFieldPart{{optionID: id}}
		}
	case json.Number:
		id, err := n.Int64()
		if err == nil && id > 0 {
			return []customFieldPart{{optionID: id}}
		}
	}
	return nil
}

func anketaFieldsFromCustom(fields []entity.SkorozvonCustomValue) []entity.SkorozvonAnketaField {
	seen := map[[2]int64]struct{}{}
	out := make([]entity.SkorozvonAnketaField, 0)
	for _, f := range fields {
		if f.FieldID <= 0 {
			continue
		}
		for _, part := range decodeCustomFieldParts(f.Raw) {
			if part.optionID <= 0 {
				continue
			}
			key := [2]int64{f.FieldID, part.optionID}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, entity.SkorozvonAnketaField{FieldID: f.FieldID, EnumID: part.optionID})
		}
	}
	return out
}

func bytesTrim(raw []byte) []byte {
	return []byte(strings.TrimSpace(string(raw)))
}

func firstNonEmptyTrim(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
