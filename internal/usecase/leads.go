package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/lib/crm"
	"github.com/lunyashon/sdmp/internal/lib/phone"
	"github.com/lunyashon/sdmp/internal/port"
)

const (
	LeadDateLayout   = "02.01.2006 15:04:05"
	defaultLimit     = 10
	maxLimit         = 100
	maxSkDispatchIDs = 10000
	maxSkResendIDs   = 1000
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
	SkSend       []string
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

type SkorozvonResendResult struct {
	Queued  int `json:"queued"`
	Missing int `json:"missing"`
}

type LeadRaw struct {
	Filename    string
	ContentType string
	Body        []byte
}

type LeadRecording struct {
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
	leads     port.LeadRepository
	storage   port.S3
	partners  port.PartnerRepository
	poster    port.HTTPFormPoster
	dicts     port.DictRepository
	maps      port.SkorozvonMapRepository
	projects  port.SkorozvonProjectRepository
	skOutbox  port.OutboxSkorozvonRepository
	sources   port.SourceRepository
	skorozvon port.Skorozvon
	crmcalls  port.Crmcalls
	log       *slog.Logger
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

func (s *LeadService) WithProjects(projects port.SkorozvonProjectRepository) *LeadService {
	s.projects = projects
	return s
}

func (s *LeadService) WithOutboxSkorozvon(box port.OutboxSkorozvonRepository) *LeadService {
	s.skOutbox = box
	return s
}

func (s *LeadService) WithSources(sources port.SourceRepository) *LeadService {
	s.sources = sources
	return s
}

func (s *LeadService) WithSkorozvon(api port.Skorozvon) *LeadService {
	s.skorozvon = api
	return s
}

func (s *LeadService) WithCrmcalls(api port.Crmcalls) *LeadService {
	s.crmcalls = api
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
		SkSend:       normalizeSkSend(q.SkSend),
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

func (s *LeadService) ListSkDispatch(ctx context.Context, ids []int64) ([]entity.LeadSkDispatch, error) {
	out := uniqueLeadIDs(ids)
	if len(out) == 0 {
		return []entity.LeadSkDispatch{}, nil
	}
	if len(out) > maxSkDispatchIDs {
		return nil, fmt.Errorf("%w: too many ids, max %d", domain.ErrInvalidInput, maxSkDispatchIDs)
	}
	items, err := s.leads.ListSkDispatch(ctx, out)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []entity.LeadSkDispatch{}
	}
	return items, nil
}

func (s *LeadService) ResendSkorozvon(ctx context.Context, ids []int64) (SkorozvonResendResult, error) {
	if s.skOutbox == nil {
		return SkorozvonResendResult{}, fmt.Errorf("skorozvon outbox is not configured")
	}
	out := uniqueLeadIDs(ids)
	if len(out) == 0 {
		return SkorozvonResendResult{}, fmt.Errorf("%w: ids are required", domain.ErrInvalidInput)
	}
	if len(out) > maxSkResendIDs {
		return SkorozvonResendResult{}, fmt.Errorf("%w: too many ids, max %d", domain.ErrInvalidInput, maxSkResendIDs)
	}
	rows, err := s.leads.ListSkResend(ctx, out)
	if err != nil {
		return SkorozvonResendResult{}, err
	}
	queued := 0
	for _, row := range rows {
		if strings.TrimSpace(row.UUID) == "" {
			continue
		}
		if err := s.skOutbox.EnqueueResend(ctx, row.UUID, parseSkorozvonProjectID(row.ProjectID)); err != nil {
			return SkorozvonResendResult{}, err
		}
		queued++
	}
	return SkorozvonResendResult{Queued: queued, Missing: len(out) - queued}, nil
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

func (s *LeadService) OutgoingStats(ctx context.Context, dateFrom, dateTo string) (entity.LeadOutgoingStats, error) {
	from, to, err := parseStatsRange(dateFrom, dateTo)
	if err != nil {
		return entity.LeadOutgoingStats{}, err
	}
	stats, err := s.leads.CountOutgoingDispatch(ctx, from, to)
	if err != nil {
		return entity.LeadOutgoingStats{}, err
	}
	if stats.Partners == nil {
		stats.Partners = []entity.LeadPartnerDispatchCount{}
	}
	if stats.Skorozvon == nil {
		stats.Skorozvon = []entity.LeadSkorozvonDispatchCount{}
	}
	if stats.Crmcalls == nil {
		stats.Crmcalls = []entity.LeadSkorozvonDispatchCount{}
	}
	return stats, nil
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

func (s *LeadService) Recording(ctx context.Context, id int64, uuid string) (LeadRecording, error) {
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
		return LeadRecording{}, err
	}

	if isSkorozvonRecording(lead) {
		if s.skorozvon == nil {
			return LeadRecording{}, fmt.Errorf("skorozvon is not configured")
		}
		callID := callIDFromLead(lead)
		if callID == "" {
			return LeadRecording{}, fmt.Errorf("%w: call recording is missing", domain.ErrNotFound)
		}
		body, ctype, err := s.skorozvon.DownloadCallRecording(ctx, callID)
		if err != nil {
			return LeadRecording{}, fmt.Errorf("download recording: %w", err)
		}
		if ctype == "" {
			ctype = "audio/mpeg"
		}
		return LeadRecording{Filename: callID + ".mp3", ContentType: ctype, Body: body}, nil
	}

	ref := strings.TrimSpace(lead.CallURL)
	if ref == "" {
		ref = strings.TrimSpace(lead.CallID)
	}
	if ref == "" {
		return LeadRecording{}, fmt.Errorf("%w: call recording is missing", domain.ErrNotFound)
	}
	if s.crmcalls == nil {
		return LeadRecording{}, fmt.Errorf("crmcalls is not configured")
	}
	body, ctype, err := s.crmcalls.DownloadRecording(ctx, ref)
	if err != nil {
		return LeadRecording{}, fmt.Errorf("download recording: %w", err)
	}
	if ctype == "" {
		ctype = "audio/mpeg"
	}
	name := strings.TrimSpace(lead.CallID)
	if name == "" {
		name = "call"
	}
	return LeadRecording{Filename: name + ".mp3", ContentType: ctype, Body: body}, nil
}

func isSkorozvonRecording(lead entity.LeadDetail) bool {
	u := strings.ToLower(strings.TrimSpace(lead.CallURL))
	if strings.Contains(u, "skorozvon") {
		return true
	}
	if strings.Contains(u, "crmcalls") || strings.Contains(u, "/api/recordings/") {
		return false
	}
	// Старые лиды СК: numeric call_id / call_url без crmcalls.
	return callIDFromLead(lead) != ""
}

func callIDFromLead(lead entity.LeadDetail) string {
	if id := digitsOnly(strings.TrimSpace(lead.CallID)); id != "" {
		return id
	}
	return callIDFromURL(lead.CallURL)
}

func callIDFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	base := path.Base(raw)
	if i := strings.IndexByte(base, '?'); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimSuffix(base, ".mp3")
	return digitsOnly(base)
}

func digitsOnly(s string) string {
	if s == "" || len(s) > 20 {
		return ""
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return ""
		}
	}
	return s
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
	in.CallURL = crm.ResolveCallURL(in.CallURL, in.CallID)
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
	s.log.Info("skorozvon enqueue bx24", "skorozvon_lead_id", skorozvonLeadID, "result_name", in.CallResultName)
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

func (s *LeadService) ApplyCrmcallsPostback(ctx context.Context, in entity.CrmcallsPostback) error {
	in.ID = strings.TrimSpace(in.ID)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Name = strings.TrimSpace(in.Name)
	in.Result = strings.TrimSpace(in.Result)
	if in.ID == "" && in.Phone == "" {
		return fmt.Errorf("%w: id or phone is required", domain.ErrInvalidInput)
	}
	s.log.Info("crmcalls postback",
		"id", in.ID,
		"phone", in.Phone,
		"name", in.Name,
		"result", in.Result,
		"call_id", in.CallID,
		"call_url", in.CallURL,
	)
	err := s.leads.ApplyCrmcallsPostback(ctx, in)
	if errors.Is(err, domain.ErrNotFound) {
		s.log.Warn("crmcalls postback lead not found", "id", in.ID, "phone", in.Phone)
		return nil
	}
	return err
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
	if normalized, ok := phone.Normalize(raw); ok {
		return normalized
	}
	return strings.TrimSpace(raw)
}

func normalizeSkSend(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		key := ""
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "ok", "success":
			key = "ok"
		case "error", "fail", "err":
			key = "error"
		case "none", "missing", "no":
			key = "none"
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func uniqueLeadIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func parseSkorozvonProjectID(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return nil
	}
	return &n
}

func (s *LeadService) attachSkDispatch(ctx context.Context, items []entity.LeadListItem) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	rows, err := s.leads.ListSkDispatch(ctx, uniqueLeadIDs(ids))
	if err != nil {
		return err
	}
	byID := make(map[int64]entity.LeadSkDispatch, len(rows))
	for _, row := range rows {
		byID[row.LeadRowID] = row
	}
	for i := range items {
		d, ok := byID[items[i].ID]
		if !ok {
			continue
		}
		okVal := d.SentOK
		items[i].SkSentOK = &okVal
		items[i].SkHTTPStatus = d.HTTPStatus
		items[i].SkError = d.Error
	}
	return nil
}
