package crm

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Scalar string

func (s Scalar) String() string { return string(s) }

type Payload struct {
	Phone    Scalar
	RegionUF Scalar
	Operator Scalar
	Tag      Scalar
}

func ParsePayload(raw string) (Payload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Payload{}, fmt.Errorf("empty payload")
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return Payload{}, err
	}
	fields, _ := root["fields"].(map[string]any)
	if fields == nil {
		fields = root
	}
	return Payload{
		Phone:    Scalar(scalar(fields["PHONE"])),
		RegionUF: Scalar(scalar(fields["UF_CRM_1747216350206"])),
		Operator: Scalar(scalar(fields["UF_CRM_1734523005552"])),
		Tag:      Scalar(scalar(fields["UF_CRM_1766149282"])),
	}, nil
}

func scalar(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		if val, ok := t["VALUE"]; ok {
			return scalar(val)
		}
		if val, ok := t["value"]; ok {
			return scalar(val)
		}
	case []any:
		if len(t) > 0 {
			return scalar(t[0])
		}
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return strings.Trim(string(b), `"`)
	}
	return ""
}
