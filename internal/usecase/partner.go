package usecase

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/curl"
	"github.com/lunyashon/sdmp/internal/port"
)

var zaryaOrder = []string{"dup", "unique"}

const partnerTestPhone = "79800010203"

type PartnerService struct {
	repo   port.PartnerRepository
	poster port.HTTPFormPoster
	now    func() time.Time
}

func NewPartnerService(repo port.PartnerRepository, poster port.HTTPFormPoster) *PartnerService {
	return &PartnerService{
		repo:   repo,
		poster: poster,
		now:    func() time.Time { return time.Now() },
	}
}

type partnerFormPoster struct {
	http *http.Client
}

func NewPartnerPoster(timeout time.Duration) port.HTTPFormPoster {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &partnerFormPoster{http: &http.Client{Timeout: timeout}}
}

func (p *partnerFormPoster) PostForm(ctx context.Context, rawURL string, data url.Values) ([]byte, int, error) {
	return curl.CurlWithContext(ctx, p.http, rawURL, http.MethodPost, strings.NewReader(data.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
}

type PartnerInput struct {
	Name          string
	WebhookURL    string
	IsActive      bool
	ZaryaResults  []string
	RegionIDs     []int
	StopSourceIDs []int
}

func (s *PartnerService) List(ctx context.Context) ([]entity.Partner, error) {
	return s.repo.List(ctx)
}

func (s *PartnerService) Create(ctx context.Context, in PartnerInput) (entity.Partner, error) {
	p, err := normalizePartner(0, in)
	if err != nil {
		return entity.Partner{}, err
	}
	return s.repo.Create(ctx, p)
}

func (s *PartnerService) Update(ctx context.Context, id int, in PartnerInput) (entity.Partner, error) {
	if id <= 0 {
		return entity.Partner{}, fmt.Errorf("%w: partner id is required", domain.ErrInvalidInput)
	}
	p, err := normalizePartner(id, in)
	if err != nil {
		return entity.Partner{}, err
	}
	return s.repo.Update(ctx, p)
}

func (s *PartnerService) SendTest(ctx context.Context, id int) (entity.PartnerCheck, error) {
	if id <= 0 {
		return entity.PartnerCheck{}, fmt.Errorf("%w: partner id is required", domain.ErrInvalidInput)
	}
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return entity.PartnerCheck{}, err
	}
	return s.checkPartner(ctx, p)
}

func (s *PartnerService) Refresh(ctx context.Context) ([]entity.PartnerCheck, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]entity.PartnerCheck, len(items))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var first error
	wg.Add(len(items))
	for i, item := range items {
		i, item := i, item
		go func() {
			defer wg.Done()
			got, err := s.checkPartner(ctx, item)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && first == nil {
				first = err
				return
			}
			out[i] = got
		}()
	}
	wg.Wait()
	if first != nil {
		return nil, first
	}
	return out, nil
}

func (s *PartnerService) checkPartner(ctx context.Context, p entity.Partner) (entity.PartnerCheck, error) {
	now := s.now()
	check := entity.PartnerCheck{ID: p.ID, Name: p.Name, CheckedAt: now}
	if strings.TrimSpace(p.WebhookURL) == "" {
		check.Response = "webhook url is empty"
		if _, err := s.repo.UpdateCheck(ctx, p.ID, now, false, 0, check.Response); err != nil {
			return entity.PartnerCheck{}, err
		}
		return check, nil
	}
	if s.poster == nil {
		return entity.PartnerCheck{}, fmt.Errorf("partner http client is not configured")
	}
	raw, status, err := s.poster.PostForm(ctx, p.WebhookURL, partnerTestForm())
	check.HTTPStatus = status
	check.Response = clipPartnerResponse(raw, err)
	check.OK = err == nil && status >= 200 && status < 300
	if _, err := s.repo.UpdateCheck(ctx, p.ID, now, check.OK, status, check.Response); err != nil {
		return entity.PartnerCheck{}, err
	}
	return check, nil
}

func partnerTestForm() url.Values {
	form := url.Values{}
	form.Set("fields[PHONE][0][VALUE]", partnerTestPhone)
	form.Set("fields[PHONE][0][VALUE_TYPE]", "WORK")
	form.Set("fields[ADDRESS_REGION]", "тест")
	form.Set("fields[STATUS_ID]", entity.StatusEnrichedName)
	form.Set("fields[COMMENTS]", "sdmp activity check")
	form.Set("phone", partnerTestPhone)
	form.Set("PHONE", partnerTestPhone)
	return form
}

func clipPartnerResponse(raw []byte, err error) string {
	text := strings.TrimSpace(string(raw))
	if text == "" && err != nil {
		text = err.Error()
	}
	if utf8.RuneCountInString(text) > 400 {
		runes := []rune(text)
		text = string(runes[:400]) + "…"
	}
	return text
}

func normalizePartner(id int, in PartnerInput) (entity.Partner, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.Partner{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	zarya, err := normalizeZarya(in.ZaryaResults)
	if err != nil {
		return entity.Partner{}, err
	}
	regions, err := uniquePositiveIDs(in.RegionIDs, "region id")
	if err != nil {
		return entity.Partner{}, err
	}
	stops, err := uniquePositiveIDs(in.StopSourceIDs, "source id")
	if err != nil {
		return entity.Partner{}, err
	}
	return entity.Partner{
		ID:            id,
		Name:          name,
		WebhookURL:    strings.TrimSpace(in.WebhookURL),
		IsActive:      in.IsActive,
		ZaryaResults:  zarya,
		RegionIDs:     regions,
		StopSourceIDs: stops,
	}, nil
}

func normalizeZarya(in []string) ([]string, error) {
	seen := map[string]bool{}
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		ok := false
		for _, allowed := range zaryaOrder {
			if v == allowed {
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("%w: zarya result %q", domain.ErrInvalidInput, v)
		}
		seen[v] = true
	}
	out := make([]string, 0, len(zaryaOrder))
	for _, v := range zaryaOrder {
		if seen[v] {
			out = append(out, v)
		}
	}
	return out, nil
}

func uniquePositiveIDs(in []int, label string) ([]int, error) {
	seen := map[int]bool{}
	out := make([]int, 0, len(in))
	for _, id := range in {
		if id <= 0 {
			return nil, fmt.Errorf("%w: %s is required", domain.ErrInvalidInput, label)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}
