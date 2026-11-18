package bitrix

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	maxBodyBytes    = 2 << 20
	WebIDField      = "UF_CRM_1752577998838"
	OperatorField   = "UF_CRM_1734523005552"
	ProjectNumField = "UF_CRM_1699340770806"
	TagField        = "UF_CRM_1766149282"
	RegionField     = "UF_CRM_1747216350206"
)

// ParseRequest собирает тело хука Bitrix из JSON, form-urlencoded и form-data.
// При конфликте ключей побеждает JSON, затем urlencoded (включая query), затем form-data.
func ParseRequest(r *http.Request) (map[string]any, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBodyBytes {
		return nil, errBodyTooLarge
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))

	jsonBody := parseJSONMap(raw)

	formData := url.Values{}
	urlencoded := cloneValues(r.URL.Query())

	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch mediaType {
	case "multipart/form-data":
		if err := r.ParseMultipartForm(maxBodyBytes); err == nil && r.MultipartForm != nil {
			formData = url.Values(r.MultipartForm.Value)
		}
	case "application/x-www-form-urlencoded":
		r.Body = io.NopCloser(bytes.NewReader(raw))
		if err := r.ParseForm(); err == nil {
			for k, vs := range r.PostForm {
				urlencoded[k] = vs
			}
		}
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))

	out := map[string]any{}
	deepMerge(out, valuesToNested(formData))
	deepMerge(out, valuesToNested(urlencoded))
	deepMerge(out, jsonBody)
	return out, nil
}

func parseJSONMap(raw []byte) map[string]any {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(trimmed, &out); err != nil {
		return nil
	}
	return out
}

func cloneValues(in url.Values) url.Values {
	out := make(url.Values, len(in))
	for k, vs := range in {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}

func valuesToNested(values url.Values) map[string]any {
	root := map[string]any{}
	if len(values) == 0 {
		return root
	}
	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		setPath(root, parsePHPKey(key), vals[len(vals)-1])
	}
	coerced := coerceArrays(root)
	out, _ := coerced.(map[string]any)
	if out == nil {
		return map[string]any{}
	}
	return out
}

func parsePHPKey(key string) []string {
	parts := make([]string, 0, 4)
	i := 0
	for i < len(key) {
		if key[i] == '[' {
			end := strings.IndexByte(key[i:], ']')
			if end < 0 {
				parts = append(parts, key[i:])
				break
			}
			parts = append(parts, key[i+1:i+end])
			i += end + 1
			continue
		}
		j := i
		for j < len(key) && key[j] != '[' {
			j++
		}
		parts = append(parts, key[i:j])
		i = j
	}
	return parts
}

func setPath(root map[string]any, path []string, val string) {
	if len(path) == 0 {
		return
	}
	cur := root
	for i, p := range path {
		if p == "" {
			p = "0"
		}
		if i == len(path)-1 {
			cur[p] = val
			return
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
}

func coerceArrays(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	for k, child := range m {
		m[k] = coerceArrays(child)
	}

	idxs := make([]int, 0, len(m))
	for k := range m {
		n, err := strconv.Atoi(k)
		if err != nil || n < 0 {
			return m
		}
		idxs = append(idxs, n)
	}
	if len(idxs) == 0 {
		return m
	}

	max := 0
	seen := make(map[int]struct{}, len(idxs))
	for _, n := range idxs {
		if n > max {
			max = n
		}
		seen[n] = struct{}{}
	}
	if len(seen) != max+1 {
		return m
	}
	out := make([]any, max+1)
	for i := 0; i <= max; i++ {
		out[i] = m[strconv.Itoa(i)]
	}
	return out
}

func deepMerge(dst, src map[string]any) {
	if src == nil {
		return
	}
	for k, v := range src {
		if vMap, ok := v.(map[string]any); ok {
			if dMap, ok := dst[k].(map[string]any); ok {
				deepMerge(dMap, vMap)
				continue
			}
		}
		dst[k] = v
	}
}

func Fields(root map[string]any) map[string]any {
	if root == nil {
		return map[string]any{}
	}
	fields := map[string]any{}
	if raw, ok := root["fields"].(map[string]any); ok {
		for k, v := range raw {
			fields[k] = v
		}
	}
	for _, key := range []string{"PHONE", "SOURCE_ID", "SOURCE_DESCRIPTION", WebIDField} {
		if _, exists := fields[key]; exists {
			continue
		}
		if v, ok := root[key]; ok {
			fields[key] = v
		}
	}
	return fields
}

func Phone(fields map[string]any) string {
	if fields == nil {
		return ""
	}
	return contactValue(fields["PHONE"])
}

func String(fields map[string]any, key string) string {
	if fields == nil {
		return ""
	}
	return stringify(fields[key])
}

type IncomingLead struct {
	Phone             string
	SourceID          string
	SourceDescription string
	WebID             string
}

func MapLead(root map[string]any) IncomingLead {
	fields := Fields(root)
	return IncomingLead{
		Phone:             Phone(fields),
		SourceID:          String(fields, "SOURCE_ID"),
		SourceDescription: String(fields, "SOURCE_DESCRIPTION"),
		WebID:             String(fields, WebIDField),
	}
}

func contactValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case []any:
		if len(t) == 0 {
			return ""
		}
		return contactValue(t[0])
	case map[string]any:
		if val, ok := t["VALUE"]; ok {
			return stringify(val)
		}
		if val, ok := t["value"]; ok {
			return stringify(val)
		}
		if val, ok := t["0"]; ok {
			return contactValue(val)
		}
	default:
		return stringify(t)
	}
	return ""
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(toString(t))
	}
}

func toString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s := string(b)
	return strings.Trim(s, `"`)
}

type tooLargeError struct{}

func (tooLargeError) Error() string { return "request body too large" }

var errBodyTooLarge tooLargeError
