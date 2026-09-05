package usecase

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type stubPartners struct {
	items []entity.Partner
	got   entity.Partner
	err   error
}

func (s *stubPartners) List(context.Context) ([]entity.Partner, error) { return s.items, s.err }
func (s *stubPartners) Get(_ context.Context, id int) (entity.Partner, error) {
	if s.err != nil {
		return entity.Partner{}, s.err
	}
	if s.got.ID != 0 {
		return s.got, nil
	}
	return entity.Partner{ID: id, Name: "КР", WebhookURL: "https://kr.test/hook"}, nil
}
func (s *stubPartners) Create(_ context.Context, in entity.Partner) (entity.Partner, error) {
	if s.err != nil {
		return entity.Partner{}, s.err
	}
	in.ID = 7
	return in, nil
}
func (s *stubPartners) Update(_ context.Context, in entity.Partner) (entity.Partner, error) {
	return in, s.err
}
func (s *stubPartners) ListRoutable(context.Context, string, *int, int) ([]entity.Partner, error) {
	return s.items, s.err
}

func (s *stubPartners) UpdateCheck(_ context.Context, id int, _ time.Time, ok bool, status int, response string) (entity.Partner, error) {
	return entity.Partner{ID: id, LastRequest: &ok, LastHTTPStatus: &status, LastResponse: response}, s.err
}

type stubPartnerPoster struct {
	status int
	body   string
	err    error
	url    string
	form   url.Values
}

func (s *stubPartnerPoster) PostForm(_ context.Context, rawURL string, data url.Values) ([]byte, int, error) {
	s.url = rawURL
	s.form = data
	return []byte(s.body), s.status, s.err
}

func TestCreatePartnerNormalizesZarya(t *testing.T) {
	svc := NewPartnerService(&stubPartners{}, nil)
	got, err := svc.Create(context.Background(), PartnerInput{
		Name:         "РТ",
		WebhookURL:   "https://example.test/hook",
		IsActive:     true,
		ZaryaResults: []string{"unique", "dup", "dup"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 || got.Name != "РТ" {
		t.Fatalf("%+v", got)
	}
	if len(got.ZaryaResults) != 2 || got.ZaryaResults[0] != "dup" || got.ZaryaResults[1] != "unique" {
		t.Fatalf("zarya %v", got.ZaryaResults)
	}
}

func TestCreatePartnerRejectsBadZarya(t *testing.T) {
	svc := NewPartnerService(&stubPartners{}, nil)
	_, err := svc.Create(context.Background(), PartnerInput{Name: "РТ", ZaryaResults: []string{"error"}})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestCreatePartnerRequiresName(t *testing.T) {
	svc := NewPartnerService(&stubPartners{}, nil)
	_, err := svc.Create(context.Background(), PartnerInput{Name: "  "})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}

func TestSendPartnerTest(t *testing.T) {
	poster := &stubPartnerPoster{status: 200, body: "accepted"}
	got, err := NewPartnerService(&stubPartners{}, poster).SendTest(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.HTTPStatus != 200 || got.Response != "accepted" || poster.url != "https://kr.test/hook" {
		t.Fatalf("%+v url=%s", got, poster.url)
	}
	if poster.form.Get("phone") != partnerTestPhone || poster.form.Get("fields[PHONE][0][VALUE]") != partnerTestPhone {
		t.Fatalf("form=%v", poster.form)
	}
}

func TestRefreshPartnersPostsEachWebhook(t *testing.T) {
	poster := &stubPartnerPoster{status: 201, body: "queued"}
	repo := &stubPartners{items: []entity.Partner{
		{ID: 1, Name: "КР", WebhookURL: "https://kr.test/hook"},
		{ID: 2, Name: "РТ", WebhookURL: ""},
	}}
	got, err := NewPartnerService(repo, poster).Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	var empty, posted entity.PartnerCheck
	for _, item := range got {
		if item.ID == 2 {
			empty = item
		} else {
			posted = item
		}
	}
	if empty.OK || empty.Response != "webhook url is empty" {
		t.Fatalf("empty %+v", empty)
	}
	if !posted.OK || posted.HTTPStatus != 201 || posted.Response != "queued" {
		t.Fatalf("posted %+v", posted)
	}
}
