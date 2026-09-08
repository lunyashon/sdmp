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
	"github.com/lunyashon/sdmp/internal/port"
	"golang.org/x/sync/errgroup"
)

type formPoster interface {
	PostForm(ctx context.Context, rawURL string, data url.Values, contentType string) ([]byte, int, error)
}

type PhoneSearch interface {
	Search(ctx context.Context, phone string) (phonesearch.Result, error)
	CheckDuplicate(ctx context.Context, phone string) (bool, error)
}

const (
	defaultOutboxLimit = 16
	outboxIdleWait     = 300 * time.Millisecond
	outboxErrorWait    = time.Second
	outboxLockTTL      = 2 * time.Minute
	outboxBx24Stale    = 5 * time.Minute
)

var partnerRetryWaits = []time.Duration{5 * time.Second, 10 * time.Second, 25 * time.Second}

type WorkerService struct {
	phoneSearch PhoneSearch
	outbox      port.OutboxRepository
	outboxBx24  port.OutboxBx24Repository
	dicts       port.DictRepository
	leads       port.LeadRepository
	splits      port.SplitRepository
	partners    port.PartnerRepository
	links       port.SkorozvonLinkRepository
	skorozvon   port.Skorozvon
	bx24        bx24API
	poster      formPoster
	retryWaits  []time.Duration
	log         *slog.Logger
	limit       int
	workers     int
	idle        time.Duration
	onErr       time.Duration
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

type bx24API interface {
	SearchLead(ctx context.Context, idempotencyKey string) (crm.LeadSearchResult, error)
	AddLead(ctx context.Context, payload []byte) (crm.LeadAddResult, error)
}

func (s *WorkerService) WithBx24(api bx24API) *WorkerService {
	s.bx24 = api
	return s
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
		bx, bxErr := s.ProcessBx24(ctx)
		if bxErr != nil {
			s.log.ErrorContext(ctx, "outbox_bx24 batch failed", "error", bxErr)
			if err := wait(ctx, s.onErr); err != nil {
				return
			}
			continue
		}
		if n == 0 && bx == 0 {
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
	for _, msg := range claim.Messages {
		if err := s.Handle(ctx, msg); err != nil {
			return n, fmt.Errorf("handle outbox %d lead %d: %w", msg.ID, msg.LeadRowID, err)
		}
		if err := s.outbox.Complete(ctx, claim.Token, msg.ID); err != nil {
			return n, fmt.Errorf("complete outbox %d: %w", msg.ID, err)
		}
		n++
	}
	return n, nil
}

func (s *WorkerService) ProcessBx24(ctx context.Context) (int, error) {
	if s.outboxBx24 == nil {
		return 0, nil
	}
	items, err := s.outboxBx24.Claim(ctx, s.limit, outboxBx24Stale)
	if err != nil {
		return 0, fmt.Errorf("claim outbox_bx24: %w", err)
	}
	for _, item := range items {
		if err := s.handleBx24(ctx, item); err != nil {
			s.log.Error("outbox_bx24 handle failed",
				"uuid", item.UUID,
				"uuid_lead", item.LeadUUID,
				"error", err,
			)
		}
	}
	return len(items), nil
}

func (s *WorkerService) handleBx24(ctx context.Context, item entity.OutboxBx24) error {
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

	if s.leads == nil {
		return fmt.Errorf("leads repo is required")
	}
	lead, err := s.leads.GetByUUID(ctx, item.LeadUUID)
	if err != nil {
		return fmt.Errorf("get lead: %w", err)
	}
	payload, err := crm.LeadAddPayload(lead)
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
	)
	if err != nil {
		return err
	}
	if added.ID == 0 {
		return nil
	}
	return s.bindBx24(ctx, item, added.ID)
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
					regionRaw = strings.TrimSpace(result.Number.Region)
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

		errg.Go(func() error {
			dup, err := s.phoneSearch.CheckDuplicate(ctxg, normalized)
			if err != nil {
				return fmt.Errorf("check duplicate: %w", err)
			}
			resultDuplicate = dup
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
		}
		if err := errg.Wait(); err != nil {
			return err
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := s.sendToPartner(ctx, msg.LeadRowID, msg.SourceID, normalized, regionRaw, enrich.RegionID, resultDuplicate, true); err != nil {
			s.log.Error("send partners failed", "lead_id", msg.LeadRowID, "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := s.sendToSkorozvon(ctx, msg.LeadRowID, msg.LeadUUID, enrich.ChildSourceID, normalized, regionRaw, enrich.RegionID, resultDuplicate); err != nil {
			s.log.Error("send skorozvon failed", "lead_id", msg.LeadRowID, "error", err)
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

func (s *WorkerService) sendToSkorozvon(ctx context.Context, leadID int64, leadUUID string, childSourceID *int, phone, region string, regionID *int, duplicate bool) error {
	if s.links == nil || s.skorozvon == nil {
		return nil
	}
	childID := 0
	if childSourceID != nil {
		childID = *childSourceID
	}
	kind := "uniq"
	if duplicate {
		kind = "dup"
	}
	items, err := s.links.ListRoutable(ctx, kind, childID, regionID)
	if err != nil {
		return fmt.Errorf("list skorozvon links: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	for i := range items {
		link := items[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.sendOneSkorozvon(ctx, leadID, leadUUID, phone, region, link)
		}()
	}
	wg.Wait()
	return nil
}

func (s *WorkerService) sendOneSkorozvon(ctx context.Context, leadID int64, leadUUID, phone, region string, link entity.SkorozvonLink) {
	res, attempts, err := s.postSkorozvon(ctx, port.SkorozvonLead{
		ID:         leadID,
		Phone:      phone,
		ProjectID:  link.ProjectID,
		Region:     region,
		ExternalID: strings.TrimSpace(leadUUID),
	})
	ok := err == nil && res.LeadID > 0 && res.Status >= 200 && res.Status < 300
	if !ok {
		s.log.Error("skorozvon send failed",
			"lead_id", leadID,
			"project_id", link.ProjectID,
			"name", link.Name,
			"status", res.Status,
			"attempts", attempts,
			"error", err,
			"body", string(res.Body),
		)
	}
	if ok {
		s.bindSkorozvon(ctx, leadID, strconv.Itoa(res.LeadID), strconv.FormatInt(link.ProjectID, 10))
	}
	s.saveSkorozvonDispatch(ctx, leadID, link.ProjectID, ok, res, attempts, err)
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
	waits := s.retryWaits
	if len(waits) == 0 {
		waits = partnerRetryWaits
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
		if !retryPartner(res.Status, err) {
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
