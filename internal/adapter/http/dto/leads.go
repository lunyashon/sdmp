package dto

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type SourceDTO struct {
	ID                int    `json:"id"`
	ParentID          *int   `json:"parent_id"`
	Name              string `json:"name"`
	BitrixCode        string `json:"bitrix_code"`
	Token             string `json:"token,omitempty"`
	ColdLabel         string `json:"cold_label,omitempty"`
	BaseLabel         string `json:"base_label,omitempty"`
	IsActive          bool   `json:"is_active"`
	IdempotencyActive bool   `json:"idempotency_active"`
}

type SourcesResponse struct {
	Count   int         `json:"count"`
	Sources []SourceDTO `json:"sources"`
}

type CreateTokenResponse struct {
	SourceID int    `json:"source_id"`
	Token    string `json:"token"`
}

type UpdateSourceActiveRequest struct {
	IsActive *bool `json:"is_active"`
}

type CreateChildSourceRequest struct {
	Name       string `json:"name"`
	BitrixCode string `json:"bitrix_code"`
	ColdLabel  string `json:"cold_label"`
	BaseLabel  string `json:"base_label"`
}

func NewSourceDTO(s entity.Source) SourceDTO {
	return SourceDTO{
		ID:                s.ID,
		ParentID:          s.ParentID,
		Name:              s.Name,
		BitrixCode:        s.BitrixCode,
		Token:             s.Token,
		ColdLabel:         s.ColdLabel,
		BaseLabel:         s.BaseLabel,
		IsActive:          s.IsActive,
		IdempotencyActive: s.IdempotencyActive,
	}
}

type LeadDTO struct {
	ID                int64  `json:"id"`
	UUID              string `json:"uuid"`
	DateCreate        string `json:"date_create"`
	Source            string `json:"source"`
	ChildSource       string `json:"child_source"`
	ColdLabel         string `json:"cold_label"`
	BaseLabel         string `json:"base_label"`
	SourceDescription string `json:"source_description"`
	Region            string `json:"region"`
	City              string `json:"city"`
	Operator          string `json:"operator"`
	StatusID          int16  `json:"status_id"`
	Stage             string `json:"stage"`
	CallResult        *int64 `json:"call_result"`
	CallResultName    string `json:"call_result_name"`
	UPK               string `json:"upk"`
	Phone             string `json:"phone"`
	Zarya             string `json:"zarya"`
	Tag               string `json:"tag"`
	WebID             string `json:"web_id"`
	ProjectNum        string `json:"project_num"`
	CallID            string `json:"call_id"`
	CallURL           string `json:"call_url"`
	ViaUs             bool   `json:"via_us"`
}

type LeadsQuery struct {
	DateFrom     string
	DateTo       string
	Sources      []int
	ChildSources []int
	ColdLabels   []string
	Regions      []int
	Operators    []int
	Statuses     []int
	Zarya        []string
	Phone        string
	City         string
	Tag          string
	CallResult   *int64
	ViaUs        *bool
	IDs          []int64
	Columns      []string
	Limit        int
	Offset       int
}

type LeadsResponse struct {
	Count  int       `json:"count"`
	Total  int       `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
	Leads  []LeadDTO `json:"leads"`
}

func NewLeadDTO(l entity.LeadListItem) LeadDTO {
	return LeadDTO{
		ID:                l.ID,
		UUID:              l.UUID,
		DateCreate:        formatLeadTime(l.DateCreate),
		Source:            l.Source,
		ChildSource:       l.ChildSource,
		ColdLabel:         l.ColdLabel,
		BaseLabel:         l.BaseLabel,
		SourceDescription: l.SourceDescription,
		Region:            l.Region,
		City:              l.City,
		Operator:          l.Operator,
		StatusID:          l.StatusID,
		Stage:             l.Stage,
		CallResult:        l.CallResult,
		CallResultName:    l.CallResultName,
		UPK:               l.UPK,
		Phone:             l.Phone,
		Zarya:             l.Zarya,
		Tag:               l.Tag,
		WebID:             l.WebID,
		ProjectNum:        l.ProjectNum,
		CallID:            l.CallID,
		CallURL:           l.CallURL,
		ViaUs:             l.ViaUs,
	}
}

func ParseLeadsQuery(q url.Values) (LeadsQuery, error) {
	out := LeadsQuery{
		DateFrom:     strings.TrimSpace(q.Get("date_from")),
		DateTo:       strings.TrimSpace(q.Get("date_to")),
		Sources:      queryInts(q, "sources"),
		ChildSources: queryInts(q, "child_sources"),
		ColdLabels:   queryStrings(q, "cold_label"),
		Regions:      queryInts(q, "region"),
		Operators:    queryInts(q, "operator"),
		Statuses:     queryInts(q, "status"),
		Zarya:        queryStrings(q, "zarya"),
		Phone:        strings.TrimSpace(q.Get("phone")),
		City:         strings.TrimSpace(q.Get("city")),
		Tag:          strings.TrimSpace(q.Get("tag")),
		IDs:          queryInt64s(q, "ids"),
		Columns:      queryStrings(q, "columns"),
		Limit:        10,
		Offset:       0,
	}
	if raw := strings.TrimSpace(q.Get("call_result")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return LeadsQuery{}, fmt.Errorf("call_result must be int")
		}
		out.CallResult = &n
	}
	viaUs, err := queryBool(q.Get("via_us"))
	if err != nil {
		return LeadsQuery{}, err
	}
	out.ViaUs = viaUs
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return LeadsQuery{}, fmt.Errorf("limit must be int")
		}
		out.Limit = n
	}
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return LeadsQuery{}, fmt.Errorf("offset must be int")
		}
		out.Offset = n
	}
	if out.DateFrom == "" || out.DateTo == "" {
		return LeadsQuery{}, fmt.Errorf("date_from and date_to are required")
	}
	return out, nil
}

func queryStrings(q url.Values, key string) []string {
	raw := append([]string{}, q[key]...)
	raw = append(raw, q[key+"[]"]...)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func queryBool(raw string) (*bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return nil, nil
	case "1", "true", "t", "yes":
		v := true
		return &v, nil
	case "0", "false", "f", "no":
		v := false
		return &v, nil
	default:
		return nil, fmt.Errorf("via_us must be bool")
	}
}

func queryInts(q url.Values, key string) []int {
	out := make([]int, 0)
	for _, s := range queryStrings(q, key) {
		n, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

func queryInt64s(q url.Values, key string) []int64 {
	out := make([]int64, 0)
	for _, s := range queryStrings(q, key) {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

func formatLeadTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, clock.Location).Format(usecase.LeadDateLayout)
}

func formatLeadTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatLeadTime(*t)
}

func parseStoredJSON(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	return v
}

type LeadEventDTO struct {
	TypeID     int16  `json:"type_id"`
	Type       string `json:"type"`
	OccurredAt string `json:"occurred_at"`
	Source     int16  `json:"source"`
	Details    string `json:"details"`
}

type LeadDispatchDTO struct {
	ChannelID   int16  `json:"channel_id"`
	Channel     string `json:"channel"`
	UPKID       *int   `json:"upk_id"`
	UPK         string `json:"upk"`
	ProjectID   string `json:"project_id"`
	SentAt      string `json:"sent_at"`
	SentOK      bool   `json:"sent_ok"`
	ExternalID  string `json:"external_id"`
	Attempts    int16  `json:"attempts"`
	HTTPStatus  *int   `json:"http_status,omitempty"`
	RawResponse string `json:"raw_response,omitempty"`
}

type LeadPhoneHistoryDTO struct {
	ID         int64  `json:"id"`
	UUID       string `json:"uuid"`
	DateCreate string `json:"date_create"`
	Source     string `json:"source"`
	StatusID   int16  `json:"status_id"`
	Stage      string `json:"stage"`
}

type LeadDetailDTO struct {
	ID                    int64                     `json:"id"`
	UUID                  string                    `json:"uuid"`
	Phone                 string                    `json:"phone"`
	PhoneValid            bool                      `json:"phone_valid"`
	ViaUs                 bool                      `json:"via_us"`
	SourceID              *int                      `json:"source_id"`
	Source                string                    `json:"source"`
	ChildSourceID         *int                      `json:"child_source_id"`
	ChildSource           string                    `json:"child_source"`
	ColdLabel             string                    `json:"cold_label"`
	BaseLabel             string                    `json:"base_label"`
	RegionID              *int                      `json:"region_id"`
	Region                string                    `json:"region"`
	RegionSource          *int16                    `json:"region_source"`
	City                  string                    `json:"city"`
	OperatorID            *int                      `json:"operator_id"`
	Operator              string                    `json:"operator"`
	OperatorSource        *int16                    `json:"operator_source"`
	WebID                 string                    `json:"web_id"`
	Tag                   string                    `json:"tag"`
	CallResult            *int64                    `json:"call_result"`
	CallResultName        string                    `json:"call_result_name"`
	CallID                string                    `json:"call_id"`
	CallURL               string                    `json:"call_url"`
	CallLastAt            string                    `json:"call_last_at,omitempty"`
	ProjectNum            string                    `json:"project_num"`
	CallForm              any                       `json:"call_form"`
	StatusID              int16                     `json:"status_id"`
	Stage                 string                    `json:"stage"`
	S3Key                 string                    `json:"s3_key"`
	LegacyFields          any                       `json:"legacy_fields"`
	BitrixLeadID          string                    `json:"bitrix_lead_id"`
	SourceDescription     string                    `json:"source_description"`
	ArchivedAt            string                    `json:"archived_at,omitempty"`
	CreatedAt             string                    `json:"created_at"`
	UpdatedAt             string                    `json:"updated_at,omitempty"`
	Zarya                 string                    `json:"zarya"`
	UPK                   string                    `json:"upk"`
	Events                []LeadEventDTO            `json:"events"`
	Dispatches            []LeadDispatchDTO         `json:"dispatches"`
	PhoneHistory          []LeadPhoneHistoryDTO     `json:"phone_history"`
	SkorozvonAnketa       *SkorozvonAnketaDTO       `json:"skorozvon_anketa"`
	SkorozvonAnketaFields []SkorozvonAnketaFieldDTO `json:"skorozvon_anketa_fields"`
}

type SkorozvonAnketaAnswerDTO struct {
	QuestionID   int64    `json:"question_id"`
	QuestionName string   `json:"question_name"`
	AnswerID     int64    `json:"answer_id"`
	AnswerValues []string `json:"answer_values"`
}

type SkorozvonAnketaDTO struct {
	ID         int64                      `json:"id"`
	UserID     *int64                     `json:"user_id"`
	FormID     *int64                     `json:"form_id"`
	ScenarioID *int64                     `json:"scenario_id"`
	FormName   string                     `json:"form_name"`
	CallID     string                     `json:"call_id"`
	UpdatedAt  string                     `json:"updated_at,omitempty"`
	Answers    []SkorozvonAnketaAnswerDTO `json:"answers"`
}

type SkorozvonAnketaFieldDTO struct {
	FieldID int64    `json:"field_id"`
	Name    string   `json:"name"`
	Values  []string `json:"values"`
}

type LeadDetailResponse struct {
	Lead LeadDetailDTO `json:"lead"`
}

func NewLeadDetailDTO(l entity.LeadDetail) LeadDetailDTO {
	events := make([]LeadEventDTO, 0, len(l.Events))
	for _, e := range l.Events {
		events = append(events, LeadEventDTO{
			TypeID:     e.TypeID,
			Type:       e.TypeName,
			OccurredAt: formatLeadTime(e.OccurredAt),
			Source:     e.Source,
			Details:    e.Details,
		})
	}
	dispatches := make([]LeadDispatchDTO, 0, len(l.Dispatches))
	for _, d := range l.Dispatches {
		dispatches = append(dispatches, LeadDispatchDTO{
			ChannelID:   d.ChannelID,
			Channel:     d.Channel,
			UPKID:       d.UPKID,
			UPK:         d.UPK,
			ProjectID:   d.ProjectID,
			SentAt:      formatLeadTime(d.SentAt),
			SentOK:      d.SentOK,
			ExternalID:  d.ExternalID,
			Attempts:    d.Attempts,
			HTTPStatus:  d.HTTPStatus,
			RawResponse: d.RawResponse,
		})
	}
	history := make([]LeadPhoneHistoryDTO, 0, len(l.PhoneHistory))
	for _, h := range l.PhoneHistory {
		history = append(history, LeadPhoneHistoryDTO{
			ID:         h.ID,
			UUID:       h.UUID,
			DateCreate: formatLeadTime(h.DateCreate),
			Source:     h.Source,
			StatusID:   h.StatusID,
			Stage:      h.Stage,
		})
	}
	return LeadDetailDTO{
		ID:                    l.ID,
		UUID:                  l.UUID,
		Phone:                 l.Phone,
		PhoneValid:            l.PhoneValid,
		ViaUs:                 l.ViaUs,
		SourceID:              l.SourceID,
		Source:                l.Source,
		ChildSourceID:         l.ChildSourceID,
		ChildSource:           l.ChildSource,
		ColdLabel:             l.ColdLabel,
		BaseLabel:             l.BaseLabel,
		RegionID:              l.RegionID,
		Region:                l.Region,
		RegionSource:          l.RegionSource,
		City:                  l.City,
		OperatorID:            l.OperatorID,
		Operator:              l.Operator,
		OperatorSource:        l.OperatorSource,
		WebID:                 l.WebID,
		Tag:                   l.Tag,
		CallResult:            l.CallResult,
		CallResultName:        l.CallResultName,
		CallID:                l.CallID,
		CallURL:               l.CallURL,
		CallLastAt:            formatLeadTimePtr(l.CallLastAt),
		ProjectNum:            l.ProjectNum,
		CallForm:              parseStoredJSON(l.CallFormRaw),
		StatusID:              l.StatusID,
		Stage:                 l.Stage,
		S3Key:                 l.S3Key,
		LegacyFields:          parseStoredJSON(l.LegacyRaw),
		BitrixLeadID:          l.BitrixLeadID,
		SourceDescription:     l.SourceDescription,
		ArchivedAt:            formatLeadTimePtr(l.ArchivedAt),
		CreatedAt:             formatLeadTime(l.CreatedAt),
		UpdatedAt:             formatLeadTimePtr(l.UpdatedAt),
		Zarya:                 l.Zarya,
		UPK:                   l.UPK,
		Events:                events,
		Dispatches:            dispatches,
		PhoneHistory:          history,
		SkorozvonAnketa:       newSkorozvonAnketaDTO(l.Anketa),
		SkorozvonAnketaFields: newSkorozvonAnketaFieldsDTO(l.AnketaFields),
	}
}

func newSkorozvonAnketaDTO(a *entity.SkorozvonAnketa) *SkorozvonAnketaDTO {
	if a == nil {
		return nil
	}
	answers := make([]SkorozvonAnketaAnswerDTO, 0, len(a.Answers))
	for _, ans := range a.Answers {
		vals := ans.AnswerValues
		if vals == nil {
			vals = []string{}
		}
		answers = append(answers, SkorozvonAnketaAnswerDTO{
			QuestionID:   ans.QuestionID,
			QuestionName: ans.QuestionName,
			AnswerID:     ans.AnswerID,
			AnswerValues: vals,
		})
	}
	return &SkorozvonAnketaDTO{
		ID:         a.ID,
		UserID:     a.UserID,
		FormID:     a.FormID,
		ScenarioID: a.ScenarioID,
		FormName:   a.FormName,
		CallID:     a.CallID,
		UpdatedAt:  formatLeadTimePtr(a.UpdatedAt),
		Answers:    answers,
	}
}

func newSkorozvonAnketaFieldsDTO(rows []entity.SkorozvonAnketaFieldItem) []SkorozvonAnketaFieldDTO {
	out := make([]SkorozvonAnketaFieldDTO, 0)
	idx := make(map[int64]int, len(rows))
	for _, r := range rows {
		name := strings.TrimSpace(r.FieldName)
		if name == "" {
			name = fmt.Sprintf("FIELD_%d", r.FieldID)
		}
		val := strings.TrimSpace(r.EnumName)
		if val == "" {
			val = strconv.FormatInt(r.EnumID, 10)
		}
		i, ok := idx[r.FieldID]
		if !ok {
			idx[r.FieldID] = len(out)
			out = append(out, SkorozvonAnketaFieldDTO{
				FieldID: r.FieldID,
				Name:    name,
				Values:  []string{val},
			})
			continue
		}
		out[i].Values = append(out[i].Values, val)
	}
	return out
}
