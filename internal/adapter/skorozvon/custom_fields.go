package skorozvon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/lunyashon/sdmp/internal/port"
)

const customFieldsPageSize = 100

func (c *Client) ListCustomFields(ctx context.Context) ([]port.SkorozvonCustomField, error) {
	byID := map[int64]port.SkorozvonCustomField{}
	for page := 1; page < 1000; page++ {
		path := fmt.Sprintf("/api/v2/custom_fields?page=%d&length=%d", page, customFieldsPageSize)
		raw, code, err := c.doJSON(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("list custom fields: %w", err)
		}
		if code != http.StatusOK {
			return nil, c.statusError(code, raw)
		}
		items, err := decodeCustomFields(raw)
		if err != nil {
			return nil, fmt.Errorf("decode custom fields: %w", err)
		}
		for _, item := range items {
			if item.ID == 0 {
				continue
			}
			byID[item.ID] = item
		}
		if len(items) < customFieldsPageSize {
			break
		}
	}

	out := make([]port.SkorozvonCustomField, 0, len(byID))
	for _, item := range byID {
		out = append(out, item)
	}
	return out, nil
}

func decodeCustomFields(raw []byte) ([]port.SkorozvonCustomField, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var rows []json.RawMessage
	switch raw[0] {
	case '[':
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, err
		}
	case '{':
		var wrap struct {
			Data  []json.RawMessage `json:"data"`
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(raw, &wrap); err != nil {
			return nil, err
		}
		rows = wrap.Data
		if len(rows) == 0 {
			rows = wrap.Items
		}
	default:
		return nil, fmt.Errorf("unexpected custom_fields payload")
	}

	out := make([]port.SkorozvonCustomField, 0, len(rows))
	for _, row := range rows {
		item, ok := decodeCustomField(row)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func decodeCustomField(raw json.RawMessage) (port.SkorozvonCustomField, bool) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return port.SkorozvonCustomField{}, false
	}
	id := jsonInt64(obj["id"])
	if id == 0 {
		return port.SkorozvonCustomField{}, false
	}
	name := jsonString(obj["name"])
	if name == "" {
		name = jsonString(obj["title"])
	}
	field := port.SkorozvonCustomField{
		ID:        id,
		Name:      name,
		FieldType: firstJSONString(obj, "field_type", "type"),
		LeadType:  jsonString(obj["lead_type"]),
	}
	if opts, ok := obj["options"].([]any); ok {
		for _, opt := range opts {
			m, ok := opt.(map[string]any)
			if !ok {
				continue
			}
			oid := jsonInt64(m["id"])
			label := firstJSONString(m, "label", "name", "title")
			if oid == 0 && label == "" {
				continue
			}
			field.Options = append(field.Options, port.SkorozvonCustomFieldOption{ID: oid, Label: label})
		}
	}
	return field, true
}

func firstJSONString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := jsonString(obj[key]); s != "" {
			return s
		}
	}
	return ""
}

func jsonString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func jsonInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	case int64:
		return n
	case int:
		return int64(n)
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}
