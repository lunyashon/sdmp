package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/lib/phone"
	"github.com/lunyashon/sdmp/internal/port"
)

const (
	LeadDateLayout = "02.01.2006 15:04:05"
	defaultLimit   = 10
	maxLimit       = 100
)

type LeadFilterQuery struct {
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
	Limit        int
	Offset       int
}

type LeadFilterResult struct {
	Total  int
	Limit  int
	Offset int
	Leads  []entity.LeadListItem
}

type LeadRaw struct {
	Filename    string
	ContentType string
	Body        []byte
}

type PartnerResendResult struct {
	PartnerID   int    `json:"partner_id"`
	PartnerName string `json:"partner_name"`
	SentOK      bool   `json:"sent_ok"`
	HTTPStatus  int    `json:"http_status"`
	RawResponse string `json:"raw_response"`
	Attempts    int16  `json:"attempts"`
}

type LeadService struct {
	leads    port.LeadRepository
	storage  port.S3
	partners port.PartnerRepository
	poster   port.HTTPFormPoster
	dicts    port.DictRepository
	maps     port.SkorozvonMapRepository
	log      *slog.Logger
}

func NewLeadService(leads port.LeadRepository, log *slog.Logger) *LeadService {
	if log == nil {
		log = slog.Default()
	}
	return &LeadService{leads: leads, log: log}
}

func (s *LeadService) WithStorage(storage port.S3) *LeadService {
	s.storage = storage
	return s
}

func (s *LeadService) WithPartners(partners port.PartnerRepository, poster port.HTTPFormPoster) *LeadService {
	s.partners = partners
	s.poster = poster
	return s
}

func (s *LeadService) WithDicts(dicts port.DictRepository) *LeadService {
	s.dicts = dicts
	return s
}

func (s *LeadService) WithSkorozvonMaps(maps port.SkorozvonMapRepository) *LeadService {
	s.maps = maps
	return s
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
		DateFrom:     from,
		DateTo:       to,
		Sources:      q.Sources,
		ChildSources: q.ChildSources,
		ColdLabels:   q.ColdLabels,
		Regions:      q.Regions,
		Operators:    q.Operators,
		Statuses:     q.Statuses,
		Zarya:        q.Zarya,
		Phone:        normalizeListPhone(q.Phone),
		City:         strings.TrimSpace(q.City),
		Tag:          strings.TrimSpace(q.Tag),
		CallResult:   q.CallResult,
		ViaUs:        q.ViaUs,
		IDs:          q.IDs,
		Limit:        limit,
		Offset:       offset,
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

func (s *LeadService) SkorozvonBitrixStats(ctx context.Context, dateFrom, dateTo string) ([]entity.LeadSkorozvonBitrixStat, error) {
	from, to, err := parseStatsRange(dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	items, err := s.leads.CountSkorozvonBitrixBySource(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []entity.LeadSkorozvonBitrixStat{}
	}
	return items, nil
}

func (s *LeadService) Get(ctx context.Context, id int64) (entity.LeadDetail, error) {
	if id <= 0 {
		return entity.LeadDetail{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	return s.leads.Get(ctx, id)
}

func (s *LeadService) GetByUUID(ctx context.Context, uuid string) (entity.LeadDetail, error) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return entity.LeadDetail{}, fmt.Errorf("%w: uuid is required", domain.ErrInvalidInput)
	}
	return s.leads.GetByUUID(ctx, uuid)
}

func (s *LeadService) ResendPartner(ctx context.Context, id int64, uuid string, partnerID int) (PartnerResendResult, error) {
	if partnerID <= 0 {
		return PartnerResendResult{}, fmt.Errorf("%w: partner id is required", domain.ErrInvalidInput)
	}
	if s.partners == nil || s.poster == nil {
		return PartnerResendResult{}, fmt.Errorf("partner sender is not configured")
	}
	var (
		lead entity.LeadDetail
		err  error
	)
	if id > 0 {
		lead, err = s.Get(ctx, id)
	} else {
		lead, err = s.GetByUUID(ctx, uuid)
	}
	if err != nil {
		return PartnerResendResult{}, err
	}
	p, err := s.partners.Get(ctx, partnerID)
	if err != nil {
		return PartnerResendResult{}, err
	}
	if strings.TrimSpace(p.WebhookURL) == "" {
		return PartnerResendResult{}, fmt.Errorf("%w: webhook url is empty", domain.ErrInvalidInput)
	}
	vars := PartnerVars{
		Phone:      lead.Phone,
		PhoneValid: lead.PhoneValid,
		Region:     lead.Region,
		City:       lead.City,
	}
	form := BuildPartnerForm(p, vars)
	target := PartnerRequestURL(p.WebhookURL, form)
	raw, status, err := s.poster.PostForm(ctx, target, form, PartnerContentType(p))
	ok := err == nil && status >= 200 && status < 300
	upkID := p.ID
	in := entity.Dispatch{
		LeadRowID:   lead.ID,
		ChannelID:   entity.ChannelPartner,
		UPKID:       &upkID,
		SentOK:      ok,
		Attempts:    1,
		RawResponse: strings.TrimSpace(string(raw)),
	}
	if status > 0 {
		in.HTTPStatus = &status
	}
	if in.RawResponse == "" && err != nil {
		in.RawResponse = err.Error()
	}
	if saveErr := s.leads.InsertDispatch(ctx, in); saveErr != nil {
		return PartnerResendResult{}, fmt.Errorf("save dispatch: %w", saveErr)
	}
	return PartnerResendResult{
		PartnerID:   p.ID,
		PartnerName: p.Name,
		SentOK:      ok,
		HTTPStatus:  status,
		RawResponse: in.RawResponse,
		Attempts:    1,
	}, nil
}

func (s *LeadService) Raw(ctx context.Context, id int64, uuid string) (LeadRaw, error) {
	if s.storage == nil {
		return LeadRaw{}, fmt.Errorf("s3 is not configured")
	}
	var (
		lead entity.LeadDetail
		err  error
	)
	if id > 0 {
		lead, err = s.Get(ctx, id)
	} else {
		lead, err = s.GetByUUID(ctx, uuid)
	}
	if err != nil {
		return LeadRaw{}, err
	}
	key := strings.TrimSpace(lead.S3Key)
	if key == "" {
		return LeadRaw{}, fmt.Errorf("%w: s3_key is empty", domain.ErrNotFound)
	}
	obj, err := s.storage.Get(ctx, key)
	if err != nil {
		return LeadRaw{}, fmt.Errorf("get raw from s3: %w", err)
	}
	defer obj.Body.Close()
	body, err := io.ReadAll(obj.Body)
	if err != nil {
		return LeadRaw{}, fmt.Errorf("read raw from s3: %w", err)
	}
	name := path.Base(key)
	if name == "." || name == "/" {
		name = "raw.json"
	}
	ct := strings.TrimSpace(obj.ContentType)
	if ct == "" {
		ct = "application/json"
	}
	return LeadRaw{Filename: name, ContentType: ct, Body: body}, nil
}

func (s *LeadService) ApplyCallResult(ctx context.Context, skorozvonLeadID string, in entity.LeadCallResult, snap entity.LeadFromSkorozvon) error {
	skorozvonLeadID = strings.TrimSpace(skorozvonLeadID)
	if skorozvonLeadID == "" {
		return fmt.Errorf("%w: lead.id is required", domain.ErrInvalidInput)
	}
	s.enrichSnapGeo(ctx, &snap)
	s.logCallResult(ctx, skorozvonLeadID, in, snap)
	err := s.leads.ApplyCallResult(ctx, skorozvonLeadID, in)
	if errors.Is(err, domain.ErrNotFound) {
		if err := s.ensureFromSkorozvon(ctx, skorozvonLeadID, snap); err != nil {
			return err
		}
		err = s.leads.ApplyCallResult(ctx, skorozvonLeadID, in)
	}
	if err != nil {
		return err
	}
	s.updateSkorozvonGeo(ctx, skorozvonLeadID, snap)
	s.replaceSkorozvonAnketaFields(ctx, skorozvonLeadID, snap.CustomFields)
	if !isTransferredLead(in.CallResultName) {
		return nil
	}
	s.log.Info("skorozvon enqueue bx24", "skorozvon_lead_id", skorozvonLeadID)
	return s.leads.EnqueueBx24(ctx, skorozvonLeadID)
}

func (s *LeadService) ApplyFormResponse(ctx context.Context, skorozvonLeadID string, in entity.LeadFormResponse, snap entity.LeadFromSkorozvon) error {
	skorozvonLeadID = strings.TrimSpace(skorozvonLeadID)
	if skorozvonLeadID == "" {
		return fmt.Errorf("%w: lead.id is required", domain.ErrInvalidInput)
	}
	s.enrichSnapGeo(ctx, &snap)
	err := s.leads.ApplyFormResponse(ctx, skorozvonLeadID, in)
	if errors.Is(err, domain.ErrNotFound) {
		if err := s.ensureFromSkorozvon(ctx, skorozvonLeadID, snap); err != nil {
			return err
		}
		err = s.leads.ApplyFormResponse(ctx, skorozvonLeadID, in)
	}
	if err != nil {
		return err
	}
	s.updateSkorozvonGeo(ctx, skorozvonLeadID, snap)
	s.replaceSkorozvonAnketaFields(ctx, skorozvonLeadID, snap.CustomFields)
	return nil
}

func (s *LeadService) logCallResult(ctx context.Context, skorozvonLeadID string, in entity.LeadCallResult, snap entity.LeadFromSkorozvon) {
	var resultID any
	if in.CallResult != nil {
		resultID = *in.CallResult
	}
	var regionID any
	if snap.RegionID != nil {
		regionID = *snap.RegionID
	}
	var lastAt any
	if in.CallLastAt != nil {
		lastAt = in.CallLastAt.In(clock.Location).Format(LeadDateLayout)
	}
	s.log.Info("skorozvon call_result parsed",
		"skorozvon_lead_id", skorozvonLeadID,
		"phone", snap.Phone,
		"city", snap.City,
		"region", snap.Region,
		"region_id", regionID,
		"tag", snap.Tag,
		"call_id", in.CallID,
		"call_url", in.CallURL,
		"call_last_at", lastAt,
		"call_result", resultID,
		"result_name", in.CallResultName,
		"project_num", in.ProjectNum,
		"custom_fields", s.formatSkorozvonCustomFields(ctx, snap.CustomFields),
	)
}

func (s *LeadService) formatSkorozvonCustomFields(ctx context.Context, fields []entity.SkorozvonCustomValue) string {
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, s.skorozvonFieldName(ctx, f.FieldID)+"="+s.skorozvonFieldValue(ctx, f))
	}
	return strings.Join(parts, ",")
}

func (s *LeadService) skorozvonFieldName(ctx context.Context, fieldID int64) string {
	fallback := fmt.Sprintf("FIELD_%d", fieldID)
	if s.maps == nil || fieldID == 0 {
		return fallback
	}
	meta, ok, err := s.maps.Get(ctx, fieldID, 0)
	if err != nil || !ok {
		return fallback
	}
	if name := strings.TrimSpace(meta.Name); name != "" {
		return name
	}
	return fallback
}

func (s *LeadService) skorozvonFieldValue(ctx context.Context, field entity.SkorozvonCustomValue) string {
	parts := decodeCustomFieldParts(field.Raw)
	if len(parts) == 0 {
		return strings.TrimSpace(string(field.Raw))
	}
	vals := make([]string, 0, len(parts))
	for _, part := range parts {
		if text := strings.TrimSpace(part.text); text != "" {
			vals = append(vals, text)
			continue
		}
		if part.optionID == 0 || s.maps == nil {
			continue
		}
		opt, ok, err := s.maps.Get(ctx, field.FieldID, part.optionID)
		if err != nil || !ok {
			vals = append(vals, fmt.Sprintf("%d", part.optionID))
			continue
		}
		if name := strings.TrimSpace(opt.Name); name != "" {
			vals = append(vals, name)
			continue
		}
		vals = append(vals, fmt.Sprintf("%d", part.optionID))
	}
	if len(vals) == 0 {
		return strings.TrimSpace(string(field.Raw))
	}
	return strings.Join(vals, ", ")
}

func (s *LeadService) updateSkorozvonGeo(ctx context.Context, skorozvonLeadID string, snap entity.LeadFromSkorozvon) {
	if err := s.leads.UpdateSkorozvonGeo(ctx, skorozvonLeadID, snap); err != nil {
		s.log.Warn("skorozvon geo update failed", "skorozvon_lead_id", skorozvonLeadID, "error", err)
	}
}

func (s *LeadService) replaceSkorozvonAnketaFields(ctx context.Context, skorozvonLeadID string, fields []entity.SkorozvonCustomValue) {
	rows := anketaFieldsFromCustom(fields)
	if len(rows) == 0 {
		return
	}
	if err := s.leads.ReplaceSkorozvonAnketaFields(ctx, skorozvonLeadID, rows); err != nil {
		s.log.Warn("skorozvon anketa fields replace failed", "skorozvon_lead_id", skorozvonLeadID, "error", err)
	}
}

func isTransferredLead(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "Передан лид")
}

func (s *LeadService) ensureFromSkorozvon(ctx context.Context, skorozvonLeadID string, snap entity.LeadFromSkorozvon) error {
	snap.SkorozvonLeadID = skorozvonLeadID
	if snap.CreatedAt.IsZero() {
		snap.CreatedAt = time.Now().In(clock.Location)
	}
	s.log.Info("skorozvon lead missing, creating from webhook",
		"skorozvon_lead_id", skorozvonLeadID,
		"phone", snap.Phone,
		"city", snap.City,
		"region", snap.Region,
	)
	return s.leads.InsertFromSkorozvon(ctx, snap)
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
	return phone.ForceCountry7(phone.Digits(raw))
}
