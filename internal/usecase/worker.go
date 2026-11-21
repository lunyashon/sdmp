package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/crm"
	"github.com/lunyashon/sdmp/internal/lib/curl"
	"github.com/lunyashon/sdmp/internal/lib/operator"
	"github.com/lunyashon/sdmp/internal/lib/phone"
	"github.com/lunyashon/sdmp/internal/lib/phonesearch"
	"github.com/lunyashon/sdmp/internal/lib/recordinglink"
	"github.com/lunyashon/sdmp/internal/port"
	"golang.org/x/sync/errgroup"
)

type formPoster interface {
	PostForm(ctx context.Context, rawURL string, data url.Values, contentType string) ([]byte, int, error)
}

type PhoneSearch interface {
	Search(ctx context.Context, phone string) (phonesearch.Result, error)
	CheckDuplicate(ctx context.Context, phone string) (bool, error)
	CheckDuplicateOld(ctx context.Context, phone string) (duplicate bool, write bool, err error)
}

const (
	defaultOutboxLimit = 16
	outboxIdleWait     = 300 * time.Millisecond
	outboxErrorWait    = time.Second
	outboxLockTTL      = 2 * time.Minute
	outboxBx24Stale    = 5 * time.Minute

	// bx24LeadCreateEnabled: false — drain without crm.lead.add / bind.
	bx24LeadCreateEnabled = true
)

var partnerRetryWaits = []time.Duration{5 * time.Second, 10 * time.Second, 25 * time.Second}

// skorozvonRetryWaits — паузы внутри одной попытки отправки в СК.
var skorozvonRetryWaits = []time.Duration{time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second}

// skorozvonOutboxDelays — паузы между попытками outbox. После последней строка остаётся для ручной переотправки.
var skorozvonOutboxDelays = []time.Duration{time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute, 30 * time.Minute}

// outboxBx24RetryDelays — 5 ретраев после первой попытки, пауза 5 минут.
var outboxBx24RetryDelays = []time.Duration{
	outboxBx24Stale,
	outboxBx24Stale,
	outboxBx24Stale,
	outboxBx24Stale,
	outboxBx24Stale,
}

type WorkerService struct {
	phoneSearch     PhoneSearch
	outbox          port.OutboxRepository
	outboxBx24      port.OutboxBx24Repository
	outboxSkorozvon port.OutboxSkorozvonRepository
	dicts           port.DictRepository
	leads           port.LeadRepository
	splits          port.SplitRepository
	partners        port.PartnerRepository
	googleSheets    port.GoogleSheetRepository
	googleAPI       port.GoogleSheets
	links           port.SkorozvonLinkRepository
	crmcallsLinks   port.CrmcallsLinkRepository
	crmcallsAPI     port.Crmcalls
	outboxCrmcalls  port.OutboxCrmcallsRepository
	skorozvon       port.Skorozvon
	maps            port.SkorozvonMapRepository
	bitrixMaps      port.BitrixMapRepository
	projects        *SkorozvonProjectService
	bx24            bx24API
	poster          formPoster
	retryWaits      []time.Duration
	skorozvonWaits  []time.Duration
	log             *slog.Logger
	limit           int
	workers         int
	idle            time.Duration
	onErr           time.Duration
	createBx24      bool
	recordingBase   string
	recordingSecret string
}

func NewWorkerService(
	outbox port.OutboxRepository,
	phoneSearch PhoneSearch,
	dicts port.DictRepository,
	leads port.LeadRepository,
	splits port.SplitRepository,
	partners port.PartnerRepository,
	log *slog.Logger,
	workers int,
) *WorkerService {
	if log == nil {
		log = slog.Default()
	}
	if workers < 1 {
		workers = 1
	}
	return &WorkerService{
		outbox:      outbox,
		phoneSearch: phoneSearch,
		dicts:       dicts,
		leads:       leads,
		splits:      splits,
		partners:    partners,
		poster:      &httpFormPoster{http: &http.Client{Timeout: 10 * time.Second}},
		retryWaits:  partnerRetryWaits,
		log:         log,
		limit:       defaultOutboxLimit,
		workers:     workers,
		idle:        outboxIdleWait,
		onErr:       outboxErrorWait,
		createBx24:  bx24LeadCreateEnabled,
	}
}

func (s *WorkerService) WithSkorozvon(links port.SkorozvonLinkRepository, api port.Skorozvon) *WorkerService {
	s.links = links
	s.skorozvon = api
	return s
}

func (s *WorkerService) WithOutboxBx24(box port.OutboxBx24Repository) *WorkerService {
	s.outboxBx24 = box
	return s
}

func (s *WorkerService) WithOutboxSkorozvon(box port.OutboxSkorozvonRepository) *WorkerService {
	s.outboxSkorozvon = box
	return s
}

func (s *WorkerService) WithGoogleSheets(repo port.GoogleSheetRepository, api port.GoogleSheets) *WorkerService {
	s.googleSheets = repo
	s.googleAPI = api
	return s
}

func (s *WorkerService) WithCrmcalls(links port.CrmcallsLinkRepository, api port.Crmcalls, box port.OutboxCrmcallsRepository) *WorkerService {
	s.crmcallsLinks = links
	s.crmcallsAPI = api
	s.outboxCrmcalls = box
	return s
}

type bx24API interface {
	SearchLead(ctx context.Context, idempotencyKey string) (crm.LeadSearchResult, error)
	AddLead(ctx context.Context, payload []byte) (crm.LeadAddResult, error)
}

func (s *WorkerService) WithBx24(api bx24API) *WorkerService {
	s.bx24 = api
	return s
}

func (s *WorkerService) WithRecordingLink(base, secret string) *WorkerService {
	if s == nil {
		return nil
	}
	s.recordingBase = strings.TrimRight(strings.TrimSpace(base), "/")
	s.recordingSecret = strings.TrimSpace(secret)
	return s
}

func (s *WorkerService) WithBitrixMaps(maps port.BitrixMapRepository) *WorkerService {
	s.bitrixMaps = maps
	return s
}

func (s *WorkerService) leadFieldMap(ctx context.Context) (crm.LeadFieldMap, error) {
	if s.bitrixMaps == nil {
		return crm.DefaultLeadFieldMap(), nil
	}
	rows, err := s.bitrixMaps.ListMaps(ctx)
	if err != nil {
		return crm.LeadFieldMap{}, err
	}
	if len(rows) == 0 {
		return crm.DefaultLeadFieldMap(), nil
	}
	return crm.LeadFieldMapFromRows(rows), nil
}

func (s *WorkerService) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	wg.Add(s.workers)
	for i := 0; i < s.workers; i++ {
		go func() {
			defer wg.Done()
			s.loop(ctx)
		}()
	}
	if s.skorozvon != nil && s.maps != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.loopSkorozvonMaps(ctx)
		}()
	}
	if s.projects != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.loopSkorozvonProjects(ctx)
		}()
	}
	if s.outboxSkorozvon != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.loopOutboxSkorozvon(ctx)
		}()
	}
	if s.outboxCrmcalls != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.loopOutboxCrmcalls(ctx)
		}()
	}
	if s.outboxBx24 != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.loopOutboxBx24(ctx)
		}()
	}
	wg.Wait()
	return ctx.Err()
}

func (s *WorkerService) loop(ctx context.Context) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}

		n, err := s.Process(ctx)
		if err != nil {
			s.log.ErrorContext(ctx, "outbox batch failed", "error", err)
			if err := wait(ctx, s.onErr); err != nil {
				return
			}
			continue
		}
		if n == 0 {
			if err := wait(ctx, s.idle); err != nil {
				return
			}
		}
	}
}

func (s *WorkerService) loopOutboxBx24(ctx context.Context) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		n, err := s.ProcessBx24(ctx)
		if err != nil {
			s.log.ErrorContext(ctx, "outbox_bx24 batch failed", "error", err)
			if err := wait(ctx, s.onErr); err != nil {
				return
			}
			continue
		}
		if n == 0 {
			if err := wait(ctx, s.idle); err != nil {
				return
			}
		}
	}
}

func (s *WorkerService) Process(ctx context.Context) (int, error) {
	claim, err := s.outbox.Claim(ctx, s.limit, outboxLockTTL)
	if err != nil {
		return 0, fmt.Errorf("claim outbox: %w", err)
	}
	if len(claim.Messages) == 0 {
		return 0, nil
	}
	defer func() { _ = s.outbox.Release(ctx, claim.Token) }()

	n := 0
	var firstErr error
	for _, msg := range claim.Messages {
		if err := s.Handle(ctx, msg); err != nil {
			s.log.Error("outbox handle failed",
				"outbox_id", msg.ID,
				"lead_id", msg.LeadRowID,
				"error", err,
			)
			if firstErr == nil {
				firstErr = fmt.Errorf("handle outbox %d lead %d: %w", msg.ID, msg.LeadRowID, err)
			}
			continue
		}
		if err := s.outbox.Complete(ctx, claim.Token, msg.ID); err != nil {
			return n, fmt.Errorf("complete outbox %d: %w", msg.ID, err)
		}
		n++
	}
	return n, firstErr
}

func (s *WorkerService) ProcessBx24(ctx context.Context) (int, error) {
	if s.outboxBx24 == nil {
		return 0, nil
	}
	items, err := s.outboxBx24.Claim(ctx, s.limit, outboxLockTTL)
	if err != nil {
		return 0, fmt.Errorf("claim outbox_bx24: %w", err)
	}
	for _, item := range items {
		s.log.Info("outbox_bx24 handle", "item", item)
		if err := s.handleBx24(ctx, item); err != nil {
			s.log.Error("outbox_bx24 handle failed",
				"uuid", item.UUID,
				"uuid_lead", item.LeadUUID,
				"count_retry", item.CountRetry,
				"error", err,
			)
			s.retryOutboxBx24(ctx, item, err)
		}
	}
	return len(items), nil
}

func (s *WorkerService) loopOutboxSkorozvon(ctx context.Context) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		n, err := s.ProcessSkorozvon(ctx)
		if err != nil {
			s.log.ErrorContext(ctx, "outbox_skorozvon batch failed", "error", err)
			if err := wait(ctx, s.onErr); err != nil {
				return
			}
			continue
		}
		if n == 0 {
			if err := wait(ctx, s.idle); err != nil {
				return
			}
		}
	}
}

func (s *WorkerService) ProcessSkorozvon(ctx context.Context) (int, error) {
	if s.outboxSkorozvon == nil || s.leads == nil || s.links == nil || s.skorozvon == nil {
		return 0, nil
	}
	items, err := s.outboxSkorozvon.Claim(ctx, s.limit, outboxLockTTL)
	if err != nil {
		return 0, fmt.Errorf("claim outbox_skorozvon: %w", err)
	}
	var wg sync.WaitGroup
	for _, item := range items {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.log.Info("outbox_skorozvon handle", "item", item)
			if err := s.handleSkorozvonOutbox(ctx, item); err != nil {
				s.log.Error("outbox_skorozvon handle failed",
					"uuid", item.UUID,
					"uuid_lead", item.LeadUUID,
					"count_retry", item.CountRetry,
					"error", err,
				)
				s.retryOutboxSkorozvon(ctx, item, err)
				return
			}
			if err := s.outboxSkorozvon.Complete(ctx, item.UUID); err != nil {
				s.log.Error("outbox_skorozvon complete failed",
					"uuid", item.UUID,
					"uuid_lead", item.LeadUUID,
					"error", err,
				)
			}
		}()
	}
	wg.Wait()
	return len(items), nil
}

func (s *WorkerService) handleSkorozvonOutbox(ctx context.Context, item entity.OutboxSkorozvon) error {
	lead, err := s.leads.GetForSkorozvon(ctx, item.LeadUUID)
	if err != nil {
		return fmt.Errorf("get lead: %w", err)
	}
	if lead.ID <= 0 || strings.TrimSpace(lead.Phone) == "" {
		return fmt.Errorf("lead %s is not ready", item.LeadUUID)
	}
	if item.ProjectID != nil && *item.ProjectID > 0 {
		return s.sendImportedSkorozvon(ctx, lead, *item.ProjectID)
	}
	return s.sendToSkorozvon(ctx, lead, zaryaDuplicate(lead.Zarya))
}

func (s *WorkerService) sendImportedSkorozvon(ctx context.Context, lead entity.LeadForSkorozvon, projectID int64) error {
	return s.sendOneSkorozvon(ctx, lead, entity.SkorozvonLink{
		Name:      "import",
		ProjectID: projectID,
	})
}

func (s *WorkerService) retryOutboxSkorozvon(ctx context.Context, item entity.OutboxSkorozvon, sendErr error) {
	next, delay, exhausted := nextSkorozvonOutboxRetry(item.CountRetry)
	lockFor := delay
	if exhausted {
		lockFor = -1
		s.log.Error("outbox_skorozvon retries exhausted",
			"uuid", item.UUID,
			"uuid_lead", item.LeadUUID,
			"count_retry", next,
			"error", sendErr,
		)
	}
	if err := s.outboxSkorozvon.MarkRetry(ctx, item.UUID, next, lockFor); err != nil {
		s.log.Error("outbox_skorozvon mark retry failed",
			"uuid", item.UUID,
			"uuid_lead", item.LeadUUID,
			"error", err,
		)
	}
}

func nextSkorozvonOutboxRetry(countRetry int16) (int16, time.Duration, bool) {
	next := countRetry + 1
	if int(countRetry) >= len(skorozvonOutboxDelays) {
		return next, 0, true
	}
	return next, skorozvonOutboxDelays[countRetry], false
}

func zaryaDuplicate(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return n == "дубль" || n == "dup"
}

func (s *WorkerService) loopOutboxCrmcalls(ctx context.Context) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		n, err := s.ProcessCrmcalls(ctx)
		if err != nil {
			s.log.ErrorContext(ctx, "outbox_crmcalls batch failed", "error", err)
			if err := wait(ctx, s.onErr); err != nil {
				return
			}
			continue
		}
		if n == 0 {
			if err := wait(ctx, s.idle); err != nil {
				return
			}
		}
	}
}

func (s *WorkerService) ProcessCrmcalls(ctx context.Context) (int, error) {
	if s.outboxCrmcalls == nil || s.leads == nil || s.crmcallsLinks == nil || s.crmcallsAPI == nil {
		return 0, nil
	}
	items, err := s.outboxCrmcalls.Claim(ctx, s.limit, outboxLockTTL)
	if err != nil {
		return 0, fmt.Errorf("claim outbox_crmcalls: %w", err)
	}
	for _, item := range items {
		s.log.Info("outbox_crmcalls handle", "item", item)
		if err := s.handleCrmcallsOutbox(ctx, item); err != nil {
			s.log.Error("outbox_crmcalls handle failed",
				"uuid", item.UUID,
				"uuid_lead", item.LeadUUID,
				"count_retry", item.CountRetry,
				"error", err,
			)
			s.retryOutboxCrmcalls(ctx, item, err)
			continue
		}
		if err := s.outboxCrmcalls.Complete(ctx, item.UUID); err != nil {
			s.log.Error("outbox_crmcalls complete failed",
				"uuid", item.UUID,
				"uuid_lead", item.LeadUUID,
				"error", err,
			)
		}
	}
	return len(items), nil
}

func (s *WorkerService) handleCrmcallsOutbox(ctx context.Context, item entity.OutboxCrmcalls) error {
	lead, err := s.leads.GetForSkorozvon(ctx, item.LeadUUID)
	if err != nil {
		return fmt.Errorf("get lead: %w", err)
	}
	if lead.ID <= 0 || strings.TrimSpace(lead.Phone) == "" {
		return fmt.Errorf("lead %s is not ready", item.LeadUUID)
	}
	return s.sendToCrmcalls(ctx, lead, zaryaDuplicate(lead.Zarya))
}

func (s *WorkerService) sendToCrmcalls(ctx context.Context, lead entity.LeadForSkorozvon, duplicate bool) error {
	if s.crmcallsLinks == nil || s.crmcallsAPI == nil {
		return nil
	}
	childID := 0
	if lead.ChildSourceID != nil {
		childID = *lead.ChildSourceID
	}
	kind := "uniq"
	if duplicate {
		kind = "dup"
	}
	items, err := s.crmcallsLinks.ListRoutable(ctx, kind, childID, lead.RegionID)
	if err != nil {
		return fmt.Errorf("list crmcalls links: %w", err)
	}
	if len(items) == 0 {
		return nil
	}
	var firstErr error
	for i := range items {
		if err := s.sendOneCrmcalls(ctx, lead, items[i]); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *WorkerService) sendOneCrmcalls(ctx context.Context, lead entity.LeadForSkorozvon, link entity.CrmcallsLink) error {
	if s.leads != nil {
		sent, err := s.leads.HasCrmcallsSent(ctx, lead.ID, link.ProjectID)
		if err != nil {
			return fmt.Errorf("has crmcalls sent: %w", err)
		}
		if sent {
			s.log.Info("crmcalls skip already sent",
				"lead_id", lead.ID,
				"project_id", link.ProjectID,
				"name", link.Name,
			)
			return nil
		}
	}
	res, err := s.crmcallsAPI.SendLead(ctx, port.CrmcallsLead{
		Phone:      lead.Phone,
		ExternalID: strings.TrimSpace(lead.UUID),
		ProjectID:  link.ProjectID,
		Region:     strings.TrimSpace(lead.Region),
	})
	ok := err == nil && res.Status >= 200 && res.Status < 300
	if !ok {
		s.log.Error("crmcalls send failed",
			"lead_id", lead.ID,
			"project_id", link.ProjectID,
			"name", link.Name,
			"status", res.Status,
			"error", err,
			"body", string(res.Body),
		)
	} else {
		s.log.Info("crmcalls sent",
			"lead_id", lead.ID,
			"project_id", link.ProjectID,
			"name", link.Name,
			"contact_id", res.ContactID,
			"status", res.Status,
		)
	}
	if ok && s.leads != nil {
		if err := s.leads.BindCrmcalls(ctx, lead.ID, res.ContactID, link.ProjectID); err != nil {
			s.log.Error("bind crmcalls failed", "lead_id", lead.ID, "error", err)
		}
	}
	s.saveCrmcallsDispatch(ctx, lead.ID, link.ProjectID, ok, res, err)
	if ok {
		return nil
	}
	if res.Status > 0 && res.Status < 500 && res.Status != http.StatusTooManyRequests {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("crmcalls status %d", res.Status)
}

func (s *WorkerService) saveCrmcallsDispatch(ctx context.Context, leadID int64, projectID string, ok bool, res port.CrmcallsSendResult, sendErr error) {
	if s.leads == nil || leadID <= 0 {
		return
	}
	in := entity.Dispatch{
		LeadRowID:          leadID,
		ChannelID:          entity.ChannelCrmcalls,
		SkorozvonProjectID: projectID,
		SentOK:             ok,
		Attempts:           1,
		ExternalID:         strings.TrimSpace(res.ContactID),
		RawResponse:        strings.TrimSpace(string(res.Body)),
	}
	if res.Status > 0 {
		in.HTTPStatus = &res.Status
	}
	if in.RawResponse == "" && sendErr != nil {
		in.RawResponse = sendErr.Error()
	}
	if err := s.leads.InsertDispatch(ctx, in); err != nil {
		s.log.Error("save crmcalls dispatch failed",
			"lead_id", leadID,
			"project_id", projectID,
			"error", err,
		)
	}
}

func (s *WorkerService) retryOutboxCrmcalls(ctx context.Context, item entity.OutboxCrmcalls, sendErr error) {
	next, delay, exhausted := nextSkorozvonOutboxRetry(item.CountRetry)
	lockFor := delay
	if exhausted {
		lockFor = -1
		s.log.Error("outbox_crmcalls retries exhausted",
			"uuid", item.UUID,
			"uuid_lead", item.LeadUUID,
			"count_retry", next,
			"error", sendErr,
		)
	}
	if err := s.outboxCrmcalls.MarkRetry(ctx, item.UUID, next, lockFor); err != nil {
		s.log.Error("outbox_crmcalls mark retry failed",
			"uuid", item.UUID,
			"uuid_lead", item.LeadUUID,
			"error", err,
		)
	}
}

func (s *WorkerService) enqueueOutboxCrmcalls(ctx context.Context, leadUUID string) error {
	if s.outboxCrmcalls == nil {
		return nil
	}
	leadUUID = strings.TrimSpace(leadUUID)
	if leadUUID == "" {
		return fmt.Errorf("enqueue outbox_crmcalls: uuid_lead is required")
	}
	if err := s.outboxCrmcalls.Enqueue(ctx, leadUUID); err != nil {
		return fmt.Errorf("enqueue outbox_crmcalls: %w", err)
	}
	return nil
}

func (s *WorkerService) enqueueOutboxSkorozvon(ctx context.Context, leadUUID string) error {
	if s.outboxSkorozvon == nil {
		return nil
	}
	leadUUID = strings.TrimSpace(leadUUID)
	if leadUUID == "" {
		return fmt.Errorf("enqueue outbox_skorozvon: uuid_lead is required")
	}
	if err := s.outboxSkorozvon.Enqueue(ctx, leadUUID); err != nil {
		return fmt.Errorf("enqueue outbox_skorozvon: %w", err)
	}
	return nil
}

func (s *WorkerService) handleBx24(ctx context.Context, item entity.OutboxBx24) error {
	if s.leads == nil {
		return fmt.Errorf("leads repo is required")
	}
	lead, err := s.leads.GetByUUID(ctx, item.LeadUUID)
	if err != nil {
		return fmt.Errorf("get lead: %w", err)
	}
	if !lead.ViaUs {
		s.log.Info("outbox_bx24 skipped, not via_us",
			"uuid", item.UUID,
			"uuid_lead", item.LeadUUID,
		)
		return s.outboxBx24.Complete(ctx, item.UUID)
	}
	if !s.createBx24 {
		s.log.Info("outbox_bx24 skipped, drop",
			"uuid", item.UUID,
			"uuid_lead", item.LeadUUID,
		)
		return s.outboxBx24.Complete(ctx, item.UUID)
	}
	if s.bx24 == nil {
		return nil
	}

	res, err := s.bx24.SearchLead(ctx, item.LeadUUID)
	if err != nil {
		return err
	}
	if res.ID != 0 {
		return s.bindBx24(ctx, item, res.ID)
	}

	m, err := s.leadFieldMap(ctx)
	if err != nil {
		return err
	}
	if u := s.bitrixCallURL(lead); u != "" {
		lead.CallURL = u
	}
	payload, err := crm.LeadAddPayloadMapped(lead, m)
	if err != nil {
		return fmt.Errorf("bx24 payload: %w", err)
	}
	added, err := s.bx24.AddLead(ctx, payload)
	s.log.Info("bx24 crm.lead.add",
		"uuid", item.UUID,
		"uuid_lead", item.LeadUUID,
		"status", added.Status,
		"result", added.ID,
		"body", string(added.Raw),
		"payload", string(payload),
	)
	if err != nil || added.ID == 0 {
		s.saveBx24Dispatch(ctx, lead.ID, added, err)
	}
	if err != nil {
		return err
	}
	if added.ID == 0 {
		return fmt.Errorf("bx24 crm.lead.add: empty id")
	}
	return s.bindBx24(ctx, item, added.ID)
}

// bitrixCallURL — наша ссылка-прокси, если запись можно отдать. Пусто = оставить call_url лида.
func (s *WorkerService) bitrixCallURL(lead entity.LeadDetail) string {
	if s == nil || s.recordingBase == "" || s.recordingSecret == "" || strings.TrimSpace(lead.UUID) == "" {
		return ""
	}
	if strings.TrimSpace(lead.CallURL) == "" && crm.ResolveCallURL("", lead.CallID) == "" {
		return ""
	}
	return recordinglink.URL(s.recordingBase, s.recordingSecret, lead.UUID)
}

func (s *WorkerService) retryOutboxBx24(ctx context.Context, item entity.OutboxBx24, sendErr error) {
	if s.outboxBx24 == nil {
		return
	}
	next, delay, exhausted := nextBx24OutboxRetry(item.CountRetry)
	lockFor := delay
	if exhausted {
		lockFor = -1
		s.log.Error("outbox_bx24 retries exhausted",
			"uuid", item.UUID,
			"uuid_lead", item.LeadUUID,
			"count_retry", next,
			"error", sendErr,
		)
	}
	if err := s.outboxBx24.MarkRetry(ctx, item.UUID, next, lockFor); err != nil {
		s.log.Error("outbox_bx24 mark retry failed",
			"uuid", item.UUID,
			"uuid_lead", item.LeadUUID,
			"error", err,
		)
	}
}

func nextBx24OutboxRetry(countRetry int16) (int16, time.Duration, bool) {
	next := countRetry + 1
	if int(countRetry) >= len(outboxBx24RetryDelays) {
		return next, 0, true
	}
	return next, outboxBx24RetryDelays[countRetry], false
}

func (s *WorkerService) bindBx24(ctx context.Context, item entity.OutboxBx24, bitrixLeadID int64) error {
	if s.leads != nil {
		if err := s.leads.BindBitrix(ctx, item.LeadUUID, strconv.FormatInt(bitrixLeadID, 10)); err != nil {
			return fmt.Errorf("bind bitrix: %w", err)
		}
	}
	if err := s.outboxBx24.Complete(ctx, item.UUID); err != nil {
		return fmt.Errorf("complete outbox_bx24: %w", err)
	}
	return nil
}

func (s *WorkerService) Handle(ctx context.Context, msg entity.OutboxMessage) error {
	fields, err := crm.ParsePayload(msg.Payload)
	if err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	normalized, ok := phone.Normalize(fields.Phone.String())
	if !ok {
		return s.rejectInvalidPhone(ctx, msg, fields.Phone.String())
	}

	regionRaw := strings.TrimSpace(fields.RegionUF.String())
	if phonesearch.BlankRegion(regionRaw) {
		regionRaw = ""
	}
	operatorRaw := strings.TrimSpace(fields.Operator.String())
	regionSrc := entity.GeoSourceUnknown
	operatorSrc := entity.GeoSourceUnknown
	if regionRaw != "" {
		regionSrc = entity.GeoSourceSupplier
	}
	if operatorRaw != "" {
		operatorSrc = entity.GeoSourceSupplier
	}

	var resultDuplicate bool
	var resultOldDuplicate bool
	var oldDuplicateEnabled bool
	if s.phoneSearch != nil {
		errg, ctxg := errgroup.WithContext(ctx)
		needLookup := regionRaw == "" || operatorRaw == ""

		if needLookup {
			errg.Go(func() error {
				result, err := s.phoneSearch.Search(ctxg, normalized)
				if err != nil {
					return fmt.Errorf("phone search: %w", err)
				}
				if regionRaw == "" {
					regionRaw = result.Number.RegionName()
					if regionRaw != "" {
						regionSrc = entity.GeoSourceLookup
					}
				}
				if operatorRaw == "" {
					operatorRaw = strings.TrimSpace(result.Number.Operator)
					if operatorRaw == "" {
						operatorRaw = strings.TrimSpace(result.Number.MobileOperator)
					}
					if operatorRaw != "" {
						operatorSrc = entity.GeoSourceLookup
					}
				}
				return nil
			})
		}

		// Заря (новая/старая) не должна ронять обогащение: иначе лиды залипают в «принята».
		errg.Go(func() error {
			dup, err := s.phoneSearch.CheckDuplicate(ctxg, normalized)
			if err != nil {
				s.log.Error("check duplicate failed, treat as unique",
					"outbox_id", msg.ID, "lead_id", msg.LeadRowID, "error", err)
				resultDuplicate = false
				return nil
			}
			resultDuplicate = dup
			return nil
		})
		errg.Go(func() error {
			dup, write, err := s.phoneSearch.CheckDuplicateOld(ctxg, normalized)
			if err != nil {
				s.log.Error("check duplicate old failed, skip zarya_old write",
					"outbox_id", msg.ID, "lead_id", msg.LeadRowID, "error", err)
				resultOldDuplicate = false
				oldDuplicateEnabled = false
				return nil
			}
			resultOldDuplicate = dup
			oldDuplicateEnabled = write
			return nil
		})

		if err := errg.Wait(); err != nil {
			return fmt.Errorf("handle outbox %d lead %d: %w", msg.ID, msg.LeadRowID, err)
		}
	}

	group := operator.Resolve(operatorRaw)
	if operatorRaw == "" {
		operatorSrc = entity.GeoSourceUnknown
	}

	enrich := entity.LeadEnrichment{
		RegionSource:   regionSrc,
		OperatorRaw:    operatorRaw,
		OperatorSource: operatorSrc,
		StatusID:       entity.StatusEnriched,
	}
	if s.dicts != nil {
		errg, ctxg := errgroup.WithContext(ctx)
		if regionRaw != "" {
			errg.Go(func() error {
				id, ok, err := s.dicts.ResolveRegion(ctxg, regionRaw)
				if err != nil {
					return fmt.Errorf("resolve region %q: %w", regionRaw, err)
				}
				if ok {
					enrich.RegionID = &id
					return nil
				}
				enrich.RegionSource = entity.GeoSourceUnknown
				return nil
			})
		}
		errg.Go(func() error {
			id, ok, err := s.dicts.ResolveOperator(ctxg, operatorRaw)
			if err != nil {
				return fmt.Errorf("resolve operator %q: %w", operatorRaw, err)
			}
			if !ok {
				id, ok, err = s.dicts.OperatorByName(ctxg, group)
				if err != nil {
					return fmt.Errorf("resolve operator %q: %w", group, err)
				}
			}
			if ok {
				enrich.OperatorID = &id
			}
			return nil
		})
		if err := errg.Wait(); err != nil {
			return err
		}
	}

	tag := strings.TrimSpace(fields.Tag.String())
	enrich.Tag = tag
	if err := s.applySplit(ctx, msg.SourceID, tag, &enrich); err != nil {
		return err
	}

	if s.leads != nil {
		errg, ctxg := errgroup.WithContext(ctx)
		errg.Go(func() error {
			if err := s.leads.UpdateEnrichment(ctxg, msg.LeadRowID, enrich); err != nil {
				return fmt.Errorf("save enrichment: %w", err)
			}
			return nil
		})
		if s.phoneSearch != nil {
			errg.Go(func() error {
				if err := s.leads.SaveZaryaResult(ctxg, msg.LeadRowID, resultDuplicate); err != nil {
					return fmt.Errorf("save zarya: %w", err)
				}
				return nil
			})
			if oldDuplicateEnabled {
				errg.Go(func() error {
					if err := s.leads.SaveZaryaOldResult(ctxg, msg.LeadRowID, resultOldDuplicate); err != nil {
						return fmt.Errorf("save zarya old: %w", err)
					}
					return nil
				})
			}
		}
		if err := errg.Wait(); err != nil {
			return err
		}
	}

	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		if err := s.sendToPartner(ctx, msg.LeadRowID, msg.SourceID, normalized, regionRaw, enrich.RegionID, resultDuplicate, true); err != nil {
			s.log.Error("send partners failed", "lead_id", msg.LeadRowID, "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := s.sendToGoogleSheets(ctx, msg.LeadRowID, msg.SourceID, normalized, regionRaw, enrich.RegionID, resultDuplicate); err != nil {
			s.log.Error("send google sheets failed", "lead_id", msg.LeadRowID, "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := s.enqueueOutboxSkorozvon(ctx, msg.LeadUUID); err != nil {
			s.log.Error("enqueue outbox_skorozvon failed", "lead_id", msg.LeadRowID, "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := s.enqueueOutboxCrmcalls(ctx, msg.LeadUUID); err != nil {
			s.log.Error("enqueue outbox_crmcalls failed", "lead_id", msg.LeadRowID, "error", err)
		}
	}()
	wg.Wait()

	return nil
}

func (s *WorkerService) rejectInvalidPhone(ctx context.Context, msg entity.OutboxMessage, raw string) error {
	reason := fmt.Sprintf("invalid phone number: %s", raw)
	s.log.Warn("outbox lead rejected",
		"outbox_id", msg.ID,
		"lead_id", msg.LeadRowID,
		"phone", raw,
	)
	if s.leads == nil {
		return nil
	}
	if err := s.leads.InsertDispatch(ctx, entity.Dispatch{
		LeadRowID:   msg.LeadRowID,
		ChannelID:   entity.ChannelBitrix,
		SentOK:      false,
		Attempts:    1,
		RawResponse: reason,
	}); err != nil {
		return fmt.Errorf("save invalid phone dispatch: %w", err)
	}
	return nil
}

func (s *WorkerService) applySplit(ctx context.Context, sourceID int, tag string, enrich *entity.LeadEnrichment) error {
	if s.splits == nil || sourceID <= 0 || enrich.OperatorID == nil {
		return nil
	}
	rule, ok, err := s.splits.Resolve(ctx, sourceID, *enrich.OperatorID)
	if err != nil {
		return fmt.Errorf("resolve split source %d operator %d: %w", sourceID, *enrich.OperatorID, err)
	}
	if !ok {
		s.log.Info("split rule missing",
			"source_id", sourceID,
			"operator_id", *enrich.OperatorID,
			"tag", tag,
		)
		return nil
	}

	matched := tegMatches(tag, rule.TegExpectedValue)
	useTest := rule.TegActive && matched && rule.TestChildSourceID != nil && *rule.TestChildSourceID > 0

	if rule.TegExpectedValue != "" {
		if useTest {
			enrich.ChildSourceID = &rule.ChildSourceID
			if rule.TestColdLabel != "" {
				enrich.ColdLabel = rule.TestColdLabel
			} else {
				enrich.ColdLabel = rule.ColdLabel
			}
			if rule.TestBaseLabel != "" {
				enrich.BaseLabel = rule.TestBaseLabel
			} else {
				enrich.BaseLabel = rule.BaseLabel
			}
		} else {
			enrich.ChildSourceID = rule.TestChildSourceID
			enrich.ColdLabel = rule.ColdLabel
			enrich.BaseLabel = rule.BaseLabel
		}
	} else {
		enrich.ChildSourceID = &rule.ChildSourceID
		enrich.ColdLabel = rule.ColdLabel
		enrich.BaseLabel = rule.BaseLabel
	}

	return nil
}

func (s *WorkerService) sendToSkorozvon(ctx context.Context, lead entity.LeadForSkorozvon, duplicate bool) error {
	if s.links == nil || s.skorozvon == nil {
		return fmt.Errorf("skorozvon is not configured")
	}
	childID := 0
	if lead.ChildSourceID != nil {
		childID = *lead.ChildSourceID
	}
	kind := "uniq"
	if duplicate {
		kind = "dup"
	}
	items, err := s.links.ListRoutable(ctx, kind, childID, lead.RegionID)
	if err != nil {
		return fmt.Errorf("list skorozvon links: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	for i := range items {
		link := items[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.sendOneSkorozvon(ctx, lead, link); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return firstErr
}

func (s *WorkerService) sendOneSkorozvon(ctx context.Context, lead entity.LeadForSkorozvon, link entity.SkorozvonLink) error {
	projectID := strconv.FormatInt(link.ProjectID, 10)
	if s.leads != nil {
		sent, err := s.leads.HasSkorozvonSent(ctx, lead.ID, projectID)
		if err != nil {
			return fmt.Errorf("has skorozvon sent: %w", err)
		}
		if sent {
			s.log.Info("skorozvon skip already sent",
				"lead_id", lead.ID,
				"project_id", link.ProjectID,
				"name", link.Name,
			)
			return nil
		}
	}
	res, attempts, err := s.postSkorozvon(ctx, port.SkorozvonLead{
		ID:           lead.ID,
		Phone:        lead.Phone,
		ProjectID:    link.ProjectID,
		Region:       lead.Region,
		ExternalID:   strings.TrimSpace(lead.UUID),
		CustomFields: s.skorozvonCustomFields(ctx, lead),
	})
	ok := err == nil && res.LeadID > 0 && res.Status >= 200 && res.Status < 300
	if !ok {
		s.log.Error("skorozvon send failed",
			"lead_id", lead.ID,
			"project_id", link.ProjectID,
			"name", link.Name,
			"status", res.Status,
			"attempts", attempts,
			"error", err,
			"body", string(res.Body),
		)
	}
	if ok {
		s.bindSkorozvon(ctx, lead.ID, strconv.Itoa(res.LeadID), strconv.FormatInt(link.ProjectID, 10))
	}
	s.saveSkorozvonDispatch(ctx, lead.ID, link.ProjectID, ok, res, attempts, err)
	if ok {
		return nil
	}
	if !retrySkorozvon(res.Status, err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("skorozvon status %d", res.Status)
}

func (s *WorkerService) bindSkorozvon(ctx context.Context, leadID int64, skorozvonLeadID, projectID string) {
	if s.leads == nil || leadID <= 0 {
		return
	}
	if err := s.leads.BindSkorozvon(ctx, leadID, skorozvonLeadID, projectID); err != nil {
		s.log.Error("bind skorozvon failed",
			"lead_id", leadID,
			"skorozvon_lead_id", skorozvonLeadID,
			"project_id", projectID,
			"error", err,
		)
	}
}

func (s *WorkerService) postSkorozvon(ctx context.Context, lead port.SkorozvonLead) (port.SkorozvonSendResult, int16, error) {
	waits := s.skorozvonWaits
	if len(waits) == 0 {
		waits = skorozvonRetryWaits
	}
	var (
		res      port.SkorozvonSendResult
		err      error
		attempts int16
	)
	for i := 0; i <= len(waits); i++ {
		if i > 0 {
			if waitErr := wait(ctx, waits[i-1]); waitErr != nil {
				return res, attempts, waitErr
			}
		}
		attempts++
		res, err = s.skorozvon.SendLead(ctx, lead)
		if !retrySkorozvon(res.Status, err) {
			return res, attempts, err
		}
	}
	return res, attempts, err
}

func (s *WorkerService) saveSkorozvonDispatch(ctx context.Context, leadID int64, projectID int64, ok bool, res port.SkorozvonSendResult, attempts int16, sendErr error) {
	if s.leads == nil || leadID <= 0 {
		return
	}
	in := entity.Dispatch{
		LeadRowID:          leadID,
		ChannelID:          entity.ChannelSkorozvon,
		SkorozvonProjectID: strconv.FormatInt(projectID, 10),
		SentOK:             ok,
		Attempts:           attempts,
		RawResponse:        strings.TrimSpace(string(res.Body)),
	}
	if ok && res.LeadID > 0 {
		in.ExternalID = strconv.Itoa(res.LeadID)
	}
	if res.Status > 0 {
		in.HTTPStatus = &res.Status
	}
	if in.RawResponse == "" && sendErr != nil {
		in.RawResponse = sendErr.Error()
	}
	if err := s.leads.InsertDispatch(ctx, in); err != nil {
		s.log.Error("save skorozvon dispatch failed",
			"lead_id", leadID,
			"project_id", projectID,
			"error", err,
		)
	}
}

func (s *WorkerService) sendToGoogleSheets(ctx context.Context, leadID int64, sourceID int, phone, region string, regionID *int, duplicate bool) error {
	if s.googleSheets == nil || s.googleAPI == nil {
		return nil
	}
	phone = strings.TrimSpace(phone)
	zarya := "unique"
	if duplicate {
		zarya = "dup"
	}
	items, err := s.googleSheets.ListRoutable(ctx, zarya, regionID, sourceID)
	if err != nil {
		return fmt.Errorf("list google sheets: %w", err)
	}
	s.log.Info("google sheets selected",
		"lead_id", leadID,
		"source_id", sourceID,
		"zarya", zarya,
		"region", region,
		"region_id", regionID,
		"count", len(items),
	)
	if len(items) == 0 {
		return nil
	}

	vars := PartnerVars{Phone: phone, PhoneValid: true, Region: strings.TrimSpace(region), City: partnerCity(ctx, s.leads, leadID)}
	var wg sync.WaitGroup
	for i := range items {
		sheet := items[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.sendOneGoogleSheet(ctx, leadID, zarya, vars, sheet)
		}()
	}
	wg.Wait()
	return nil
}

func (s *WorkerService) sendOneGoogleSheet(ctx context.Context, leadID int64, zarya string, vars PartnerVars, sheet entity.GoogleSheetTarget) {
	raw, status, attempts, err := s.postGoogleSheet(ctx, sheet, vars)
	ok := err == nil && status >= 200 && status < 300
	if !ok {
		s.log.Error("google sheet send failed",
			"lead_id", leadID,
			"sheet_id", sheet.ID,
			"name", sheet.Name,
			"spreadsheet_id", sheet.SpreadsheetID,
			"status", status,
			"attempts", attempts,
			"error", err,
			"body", string(raw),
		)
	} else {
		s.log.Info("google sheet sent",
			"lead_id", leadID,
			"sheet_id", sheet.ID,
			"name", sheet.Name,
			"spreadsheet_id", sheet.SpreadsheetID,
			"zarya", zarya,
			"status", status,
			"attempts", attempts,
		)
	}
	s.saveGoogleDispatch(ctx, leadID, sheet.ID, ok, status, attempts, raw, err)
}

func (s *WorkerService) postGoogleSheet(ctx context.Context, sheet entity.GoogleSheetTarget, vars PartnerVars) ([]byte, int, int16, error) {
	values := GoogleSheetRow(sheet.Columns, vars)
	if len(values) == 0 {
		return nil, 0, 0, fmt.Errorf("google sheet %d has nothing to append", sheet.ID)
	}
	waits := s.retryWaits
	if len(waits) == 0 {
		waits = partnerRetryWaits
	}
	var (
		raw      []byte
		status   int
		err      error
		attempts int16
	)
	for i := 0; i <= len(waits); i++ {
		if i > 0 {
			if waitErr := wait(ctx, waits[i-1]); waitErr != nil {
				return raw, status, attempts, waitErr
			}
		}
		attempts++
		status, raw, err = s.googleAPI.AppendRow(ctx, sheet.Credentials, sheet.SpreadsheetID, sheet.SheetName, values)
		if !retryPartner(status, err) {
			return raw, status, attempts, err
		}
	}
	return raw, status, attempts, err
}

func (s *WorkerService) saveGoogleDispatch(ctx context.Context, leadID int64, sheetID int, ok bool, status int, attempts int16, raw []byte, sendErr error) {
	if s.leads == nil || leadID <= 0 {
		return
	}
	in := entity.Dispatch{
		LeadRowID:   leadID,
		ChannelID:   entity.ChannelGoogle,
		SentOK:      ok,
		Attempts:    attempts,
		ExternalID:  strconv.Itoa(sheetID),
		RawResponse: strings.TrimSpace(string(raw)),
	}
	if status > 0 {
		in.HTTPStatus = &status
	}
	if in.RawResponse == "" && sendErr != nil {
		in.RawResponse = sendErr.Error()
	}
	if err := s.leads.InsertDispatch(ctx, in); err != nil {
		s.log.Error("save google sheet dispatch failed",
			"lead_id", leadID,
			"sheet_id", sheetID,
			"error", err,
		)
	}
}

func (s *WorkerService) sendToPartner(ctx context.Context, leadID int64, sourceID int, phone, region string, regionID *int, duplicate, phoneValid bool) error {
	if s.partners == nil {
		return nil
	}
	zarya := "unique"
	if duplicate {
		zarya = "dup"
	}
	items, err := s.partners.ListRoutable(ctx, zarya, regionID, sourceID)
	if err != nil {
		return fmt.Errorf("list partners: %w", err)
	}
	s.log.Info("partners selected",
		"lead_id", leadID,
		"source_id", sourceID,
		"zarya", zarya,
		"region", region,
		"region_id", regionID,
		"count", len(items),
	)
	if len(items) == 0 {
		return nil
	}

	vars := PartnerVars{Phone: phone, PhoneValid: phoneValid, Region: region, City: partnerCity(ctx, s.leads, leadID)}
	var wg sync.WaitGroup
	for i := range items {
		p := items[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.sendOnePartner(ctx, leadID, zarya, p, vars)
		}()
	}
	wg.Wait()
	return nil
}

func partnerCity(ctx context.Context, leads port.LeadRepository, leadID int64) string {
	if leads == nil || leadID <= 0 {
		return ""
	}
	lead, err := leads.Get(ctx, leadID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(lead.City)
}

func (s *WorkerService) sendOnePartner(ctx context.Context, leadID int64, zarya string, p entity.Partner, vars PartnerVars) {
	if strings.TrimSpace(p.WebhookURL) == "" {
		s.log.Info("partner webhook empty", "partner_id", p.ID, "name", p.Name)
		return
	}
	form := BuildPartnerForm(p, vars)
	target := PartnerRequestURL(p.WebhookURL, form)
	raw, status, attempts, err := s.postPartner(ctx, target, form, PartnerContentType(p))
	ok := err == nil && status >= 200 && status < 300
	if !ok {
		s.log.Error("partner send failed",
			"lead_id", leadID,
			"partner_id", p.ID,
			"name", p.Name,
			"url", target,
			"status", status,
			"attempts", attempts,
			"error", err,
			"body", string(raw),
		)
	} else {
		s.log.Info("partner sent",
			"lead_id", leadID,
			"partner_id", p.ID,
			"name", p.Name,
			"url", target,
			"zarya", zarya,
			"status", status,
			"attempts", attempts,
			"body", string(raw),
		)
	}
	s.savePartnerDispatch(ctx, leadID, p.ID, ok, status, attempts, raw, err)
}

func (s *WorkerService) postPartner(ctx context.Context, rawURL string, form url.Values, contentType string) ([]byte, int, int16, error) {
	waits := s.retryWaits
	if len(waits) == 0 {
		waits = partnerRetryWaits
	}
	var (
		raw      []byte
		status   int
		err      error
		attempts int16
	)
	for i := 0; i <= len(waits); i++ {
		if i > 0 {
			if waitErr := wait(ctx, waits[i-1]); waitErr != nil {
				return raw, status, attempts, waitErr
			}
		}
		attempts++
		raw, status, err = s.poster.PostForm(ctx, rawURL, form, contentType)
		if !retryPartner(status, err) {
			return raw, status, attempts, err
		}
	}
	return raw, status, attempts, err
}

func retryPartner(status int, err error) bool {
	if err != nil {
		return true
	}
	return status == http.StatusTooManyRequests || status >= 500
}

func retrySkorozvon(status int, err error) bool {
	if status == http.StatusTooManyRequests || status >= 500 {
		return true
	}
	if status > 0 {
		return false
	}
	return err != nil
}

func (s *WorkerService) saveBx24Dispatch(ctx context.Context, leadID int64, added crm.LeadAddResult, sendErr error) {
	if s.leads == nil || leadID <= 0 {
		return
	}
	in := entity.Dispatch{
		LeadRowID:   leadID,
		ChannelID:   entity.ChannelBitrix,
		SentOK:      sendErr == nil && added.ID != 0 && added.Status >= 200 && added.Status < 300,
		Attempts:    1,
		ExternalID:  strconv.FormatInt(added.ID, 10),
		RawResponse: strings.TrimSpace(string(added.Raw)),
	}
	if added.ID == 0 {
		in.ExternalID = ""
	}
	if added.Status > 0 {
		in.HTTPStatus = &added.Status
	}
	if in.RawResponse == "" && sendErr != nil {
		in.RawResponse = sendErr.Error()
	}
	if err := s.leads.InsertDispatch(ctx, in); err != nil {
		s.log.Error("save bx24 dispatch failed",
			"lead_id", leadID,
			"error", err,
		)
	}
}

func (s *WorkerService) savePartnerDispatch(ctx context.Context, leadID int64, upkID int, ok bool, status int, attempts int16, raw []byte, sendErr error) {
	if s.leads == nil || leadID <= 0 {
		return
	}
	in := entity.Dispatch{
		LeadRowID:   leadID,
		ChannelID:   entity.ChannelPartner,
		UPKID:       &upkID,
		SentOK:      ok,
		Attempts:    attempts,
		RawResponse: strings.TrimSpace(string(raw)),
	}
	if status > 0 {
		in.HTTPStatus = &status
	}
	if in.RawResponse == "" && sendErr != nil {
		in.RawResponse = sendErr.Error()
	}
	if err := s.leads.InsertDispatch(ctx, in); err != nil {
		s.log.Error("save partner dispatch failed",
			"lead_id", leadID,
			"partner_id", upkID,
			"error", err,
		)
	}
}

type httpFormPoster struct {
	http *http.Client
}

func (p *httpFormPoster) PostForm(ctx context.Context, rawURL string, data url.Values, contentType string) ([]byte, int, error) {
	if strings.TrimSpace(contentType) == "" {
		contentType = entity.PartnerContentTypeForm
	}
	return curl.CurlWithContext(ctx, p.http, rawURL, http.MethodPost, strings.NewReader(data.Encode()), map[string]string{
		"Content-Type": contentType,
	})
}

func tegMatches(tag, expected string) bool {
	return strings.EqualFold(strings.TrimSpace(tag), strings.TrimSpace(expected))
}

func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
