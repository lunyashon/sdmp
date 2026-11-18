package skorozvon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/lunyashon/sdmp/internal/port"
)

const callProjectsPageSize = 100

func (c *Client) ListCallProjects(ctx context.Context) ([]port.SkorozvonCallProject, error) {
	byID := map[int64]port.SkorozvonCallProject{}
	for page := 1; page < 1000; page++ {
		path := fmt.Sprintf("/api/v2/call_projects?type=call&page=%d&length=%d", page, callProjectsPageSize)
		raw, code, err := c.doJSON(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("list call projects: %w", err)
		}
		if code != http.StatusOK {
			return nil, c.statusError(code, raw)
		}
		items, err := decodeCallProjects(raw)
		if err != nil {
			return nil, fmt.Errorf("decode call projects: %w", err)
		}
		for _, item := range items {
			if item.ID == 0 {
				continue
			}
			byID[item.ID] = item
		}
		if len(items) < callProjectsPageSize {
			break
		}
	}

	out := make([]port.SkorozvonCallProject, 0, len(byID))
	for _, item := range byID {
		out = append(out, item)
	}
	return out, nil
}

func decodeCallProjects(raw []byte) ([]port.SkorozvonCallProject, error) {
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
		return nil, fmt.Errorf("unexpected call_projects payload")
	}

	out := make([]port.SkorozvonCallProject, 0, len(rows))
	for _, row := range rows {
		item, ok := decodeCallProject(row)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func decodeCallProject(raw json.RawMessage) (port.SkorozvonCallProject, bool) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return port.SkorozvonCallProject{}, false
	}
	id := jsonInt64(obj["id"])
	if id == 0 {
		return port.SkorozvonCallProject{}, false
	}
	title := firstJSONString(obj, "title", "name")
	return port.SkorozvonCallProject{ID: id, Title: strings.TrimSpace(title)}, true
}
