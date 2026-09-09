package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type stubLeadList struct {
	got   port.LeadListFilter
	items []entity.LeadListItem
	total int
	err   error
}

func (s *stubLeadList) InsertAccepted(context.Context, entity.LeadRow, entity.OutboxMessage) (int64, error) {
	return 0, nil
}

func (s *stubLeadList) List(_ context.Context, f port.LeadListFilter) ([]entity.LeadListItem, int, error) {
	s.got = f
	return s.items, s.total, s.err
}

func (s *stubLeadList) CountBySource(context.Context, time.Time, time.Time) ([]entity.LeadSourceCount, error) {
	return []entity.LeadSourceCount{{SourceID: 7, Count: 21}}, s.err
}

func (s *stubLeadList) CountEnrichmentBySource(context.Context, time.Time, time.Time) ([]entity.LeadEnrichmentStat, error) {
	return []entity.LeadEnrichmentStat{{
		SourceID: 7, Total: 20,
		Region:   entity.LeadGeoShare{Supplier: 10, Provider: 6, Unknown: 4},
		Operator: entity.LeadGeoShare{Supplier: 8, Provider: 9, Unknown: 3},
	}}, s.err
}

func (s *stubLeadList) CountZaryaBySource(context.Context, time.Time, time.Time) ([]entity.LeadZaryaStat, error) {
	return []entity.LeadZaryaStat{{SourceID: 7, Total: 15, Unique: 9, Dup: 6}}, s.err
}

func (s *stubLeadList) CountSkorozvonBitrixBySource(context.Context, time.Time, time.Time) ([]entity.LeadSkorozvonBitrixStat, error) {
	return []entity.LeadSkorozvonBitrixStat{{
		SourceID: 7, Updated: 20, UpdatedSent: 11, Created: 5, CreatedSent: 3,
	}}, s.err
}

func (s *stubLeadList) Get(context.Context, int64) (entity.LeadDetail, error) {
	return entity.LeadDetail{}, nil
}

func (s *stubLeadList) GetByUUID(context.Context, string) (entity.LeadDetail, error) {
	return entity.LeadDetail{}, nil
}

func (s *stubLeadList) UpdateEnrichment(context.Context, int64, entity.LeadEnrichment) error {
	return nil
}

func (s *stubLeadList) InsertDispatch(context.Context, entity.Dispatch) error {
	return nil
}

func (s *stubLeadList) SaveZaryaResult(context.Context, int64, bool) error {
	return nil
}

func (s *stubLeadList) BindSkorozvon(context.Context, int64, string, string) error {
	return nil
}

func (s *stubLeadList) BindBitrix(context.Context, string, string) error {
	return nil
}

func (s *stubLeadList) ApplyCallResult(context.Context, string, entity.LeadCallResult) error {
	return nil
}

func (s *stubLeadList) ApplyFormResponse(context.Context, string, entity.LeadFormResponse) error {
	return nil
}

func (s *stubLeadList) InsertFromSkorozvon(context.Context, entity.LeadFromSkorozvon) error {
	return nil
}

func (s *stubLeadList) UpdateSkorozvonGeo(context.Context, string, entity.LeadFromSkorozvon) error {
	return nil
}

func (s *stubLeadList) ReplaceSkorozvonAnketaFields(context.Context, string, []entity.SkorozvonAnketaField) error {
	return nil
}

func (s *stubLeadList) EnqueueBx24(context.Context, string) error {
	return nil
}

func TestFilterLeads(t *testing.T) {
	created := time.Date(2026, 8, 27, 16, 27, 56, 0, time.Local)
	repo := &stubLeadList{
		total: 2,
		items: []entity.LeadListItem{{
			DateCreate: created,
			Source:     "GDMP",
			ColdLabel:  "",
			Region:     "Москва",
			Operator:   "БИЛАЙН",
			Stage:      "в Б24",
			Phone:      "79009876539",
		}},
	}
	svc := NewLeadService(repo, nil)
	res, err := svc.Filter(context.Background(), LeadFilterQuery{
		DateFrom:     "01.08.2026 00:00:00",
		DateTo:       "30.08.2026 23:59:59",
		Sources:      []int{1, 12},
		ChildSources: []int{20},
		ColdLabels:   []string{"ВР"},
		Regions:      []int{1, 5},
		Operators:    []int{2, 3},
		Zarya:        []string{"Чистый", "Дубль"},
		Phone:        "8 (900) 987-65-39",
		ViaUs:        boolPtr(false),
		IDs:          []int64{42},
		Limit:        0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 || res.Limit != 10 || len(res.Leads) != 1 {
		t.Fatalf("%+v", res)
	}
	if repo.got.Phone != "79009876539" {
		t.Fatalf("phone %q", repo.got.Phone)
	}
	if len(repo.got.Sources) != 2 || repo.got.ColdLabels[0] != "ВР" || len(repo.got.ChildSources) != 1 || repo.got.ChildSources[0] != 20 {
		t.Fatalf("filter %+v", repo.got)
	}
	if len(repo.got.Regions) != 2 || len(repo.got.Operators) != 2 || len(repo.got.Zarya) != 2 || repo.got.Zarya[0] != "Чистый" {
		t.Fatalf("filter %+v", repo.got)
	}
	if repo.got.ViaUs == nil || *repo.got.ViaUs {
		t.Fatalf("via_us %+v", repo.got.ViaUs)
	}
	if len(repo.got.IDs) != 1 || repo.got.IDs[0] != 42 {
		t.Fatalf("ids %+v", repo.got.IDs)
	}
}

func TestFilterLeadsStripsPhoneSpaces(t *testing.T) {
	repo := &stubLeadList{}
	_, err := NewLeadService(repo, nil).Filter(context.Background(), LeadFilterQuery{
		DateFrom: "01.08.2026 00:00:00",
		DateTo:   "30.08.2026 23:59:59",
		Phone:    "7 900 987 65 39",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.got.Phone != "79009876539" {
		t.Fatalf("phone %q", repo.got.Phone)
	}
}

func TestFilterInvalidRange(t *testing.T) {
	svc := NewLeadService(&stubLeadList{}, nil)
	_, err := svc.Filter(context.Background(), LeadFilterQuery{
		DateFrom: "30.08.2026 23:59:59",
		DateTo:   "01.08.2026 00:00:00",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestFilterRequiresDates(t *testing.T) {
	svc := NewLeadService(&stubLeadList{}, nil)
	_, err := svc.Filter(context.Background(), LeadFilterQuery{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestIncomingStats(t *testing.T) {
	got, err := NewLeadService(&stubLeadList{}, nil).IncomingStats(context.Background(), "2026-08-14", "2026-08-20")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SourceID != 7 || got[0].Count != 21 {
		t.Fatalf("%+v", got)
	}
}

func TestEnrichmentStats(t *testing.T) {
	got, err := NewLeadService(&stubLeadList{}, nil).EnrichmentStats(context.Background(), "2026-08-14", "2026-08-20")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Region.Supplier != 10 || got[0].Operator.Unknown != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestZaryaStats(t *testing.T) {
	got, err := NewLeadService(&stubLeadList{}, nil).ZaryaStats(context.Background(), "2026-08-14", "2026-08-20")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Unique != 9 || got[0].Dup != 6 {
		t.Fatalf("%+v", got)
	}
}

func TestSkorozvonBitrixStats(t *testing.T) {
	got, err := NewLeadService(&stubLeadList{}, nil).SkorozvonBitrixStats(context.Background(), "2026-08-14", "2026-08-20")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Updated != 20 || got[0].UpdatedSent != 11 || got[0].Created != 5 || got[0].CreatedSent != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestGetLead(t *testing.T) {
	created := time.Date(2026, 8, 27, 16, 27, 56, 0, time.Local)
	repo := &stubLeadGet{
		detail: entity.LeadDetail{ID: 7, Phone: "79001234567", CreatedAt: created, Stage: "принята"},
	}
	got, err := NewLeadService(repo, nil).Get(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 || repo.gotID != 7 {
		t.Fatalf("%+v id=%d", got, repo.gotID)
	}
}

func TestGetLeadByUUID(t *testing.T) {
	repo := &stubLeadGet{
		detail: entity.LeadDetail{ID: 7, UUID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"},
	}
	got, err := NewLeadService(repo, nil).GetByUUID(context.Background(), "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	if err != nil {
		t.Fatal(err)
	}
	if got.UUID != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" || repo.gotUUID != got.UUID {
		t.Fatalf("%+v uuid=%q", got, repo.gotUUID)
	}
}

func TestGetLeadInvalidID(t *testing.T) {
	_, err := NewLeadService(&stubLeadList{}, nil).Get(context.Background(), 0)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestLeadRaw(t *testing.T) {
	repo := &stubLeadGet{detail: entity.LeadDetail{ID: 7, S3Key: "raw/10/2026/09/a.json"}}
	storage := &memS3{files: map[string]string{"raw/10/2026/09/a.json": `{"ok":true}`}}
	got, err := NewLeadService(repo, nil).WithStorage(storage).Raw(context.Background(), 7, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "a.json" || string(got.Body) != `{"ok":true}` || got.ContentType != "application/json" {
		t.Fatalf("%+v", got)
	}
}

func TestLeadRawEmptyKey(t *testing.T) {
	repo := &stubLeadGet{detail: entity.LeadDetail{ID: 7}}
	_, err := NewLeadService(repo, nil).WithStorage(&memS3{}).Raw(context.Background(), 7, "")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

type stubCallResultRepo struct {
	stubLeadList
	gotID    string
	got      entity.LeadCallResult
	geo      *entity.LeadFromSkorozvon
	fields   []entity.SkorozvonAnketaField
	queued   string
	inserted *entity.LeadFromSkorozvon
	applyN   int
	err      error
}

func (s *stubCallResultRepo) ApplyCallResult(_ context.Context, skorozvonLeadID string, in entity.LeadCallResult) error {
	s.applyN++
	s.gotID = skorozvonLeadID
	s.got = in
	if s.err != nil && s.inserted == nil {
		return s.err
	}
	return nil
}

func (s *stubCallResultRepo) InsertFromSkorozvon(_ context.Context, in entity.LeadFromSkorozvon) error {
	cp := in
	s.inserted = &cp
	return nil
}

func (s *stubCallResultRepo) UpdateSkorozvonGeo(_ context.Context, _ string, in entity.LeadFromSkorozvon) error {
	cp := in
	s.geo = &cp
	return nil
}

func (s *stubCallResultRepo) ReplaceSkorozvonAnketaFields(_ context.Context, _ string, fields []entity.SkorozvonAnketaField) error {
	s.fields = append([]entity.SkorozvonAnketaField(nil), fields...)
	return nil
}

func (s *stubCallResultRepo) EnqueueBx24(_ context.Context, skorozvonLeadID string) error {
	s.queued = skorozvonLeadID
	return nil
}

func TestApplyCallResult(t *testing.T) {
	ended := time.Date(2019, 1, 30, 10, 6, 8, 0, time.UTC)
	result := int64(54321)
	repo := &stubCallResultRepo{}
	err := NewLeadService(repo, nil).ApplyCallResult(context.Background(), " 123456788 ", entity.LeadCallResult{
		CallID:         "987654321",
		CallURL:        "https://api.skorozvon.ru/api/calls/987654321.mp3",
		CallLastAt:     &ended,
		CallResult:     &result,
		CallResultName: "Результат звонка",
		ProjectNum:     "12345678",
	}, entity.LeadFromSkorozvon{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.gotID != "123456788" {
		t.Fatalf("id %q", repo.gotID)
	}
	if repo.got.CallID != "987654321" || repo.got.ProjectNum != "12345678" || repo.got.CallResult == nil || *repo.got.CallResult != 54321 || repo.got.CallResultName != "Результат звонка" {
		t.Fatalf("%+v", repo.got)
	}
	if repo.inserted != nil {
		t.Fatalf("unexpected insert %+v", repo.inserted)
	}
	if repo.queued != "" {
		t.Fatalf("queued %q", repo.queued)
	}
}

func TestApplyCallResultQueuesBx24WhenTransferred(t *testing.T) {
	repo := &stubCallResultRepo{}
	err := NewLeadService(repo, nil).ApplyCallResult(context.Background(), "51038587780", entity.LeadCallResult{
		CallID:         "1",
		CallResultName: " Передан лид ",
	}, entity.LeadFromSkorozvon{
		CustomFields: []entity.SkorozvonCustomValue{
			{FieldID: 50000009440, Raw: []byte(`[50000014691]`)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.queued != "51038587780" {
		t.Fatalf("queued %q", repo.queued)
	}
	if len(repo.fields) != 1 || repo.fields[0].FieldID != 50000009440 || repo.fields[0].EnumID != 50000014691 {
		t.Fatalf("fields %+v", repo.fields)
	}
}

func TestApplyCallResultCreatesWhenMissing(t *testing.T) {
	repo := &stubCallResultRepo{err: domain.ErrNotFound}
	snap := entity.LeadFromSkorozvon{Phone: "79001234567", City: "Москва"}
	err := NewLeadService(repo, nil).ApplyCallResult(context.Background(), "99", entity.LeadCallResult{CallID: "1"}, snap)
	if err != nil {
		t.Fatal(err)
	}
	if repo.applyN != 2 || repo.inserted == nil {
		t.Fatalf("applyN=%d inserted=%v", repo.applyN, repo.inserted)
	}
	if repo.inserted.SkorozvonLeadID != "99" || repo.inserted.Phone != "79001234567" || repo.inserted.City != "Москва" {
		t.Fatalf("%+v", repo.inserted)
	}
}

func TestApplyFormResponse(t *testing.T) {
	repo := &stubFormResponseRepo{}
	formID := int64(1)
	err := NewLeadService(repo, nil).ApplyFormResponse(context.Background(), " 123456788 ", entity.LeadFormResponse{
		FormName: "Анкета",
		FormID:   &formID,
		Answers:  []entity.SkorozvonAnketaAnswer{{QuestionName: "Город", AnswerValues: []string{"Екатеринбург"}}},
	}, entity.LeadFromSkorozvon{
		CustomFields: []entity.SkorozvonCustomValue{
			{FieldID: 50000011777, Raw: []byte(`[50000004560]`)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.gotID != "123456788" || repo.got.FormName != "Анкета" || len(repo.got.Answers) != 1 {
		t.Fatalf("%q %+v", repo.gotID, repo.got)
	}
	if len(repo.fields) != 1 || repo.fields[0].FieldID != 50000011777 || repo.fields[0].EnumID != 50000004560 {
		t.Fatalf("fields %+v", repo.fields)
	}
}

func TestApplyFormResponseCreatesWhenMissing(t *testing.T) {
	repo := &stubFormResponseRepo{err: domain.ErrNotFound}
	err := NewLeadService(repo, nil).ApplyFormResponse(context.Background(), "88", entity.LeadFormResponse{FormName: "Анкета"}, entity.LeadFromSkorozvon{Phone: "+79001112233"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.applyN != 2 || repo.inserted == nil || repo.inserted.Phone != "+79001112233" {
		t.Fatalf("applyN=%d inserted=%v", repo.applyN, repo.inserted)
	}
}

type stubFormResponseRepo struct {
	stubLeadList
	gotID    string
	got      entity.LeadFormResponse
	fields   []entity.SkorozvonAnketaField
	inserted *entity.LeadFromSkorozvon
	applyN   int
	err      error
}

func (s *stubFormResponseRepo) ApplyFormResponse(_ context.Context, skorozvonLeadID string, in entity.LeadFormResponse) error {
	s.applyN++
	s.gotID = skorozvonLeadID
	s.got = in
	if s.err != nil && s.inserted == nil {
		return s.err
	}
	return nil
}

func (s *stubFormResponseRepo) InsertFromSkorozvon(_ context.Context, in entity.LeadFromSkorozvon) error {
	cp := in
	s.inserted = &cp
	return nil
}

func (s *stubFormResponseRepo) ReplaceSkorozvonAnketaFields(_ context.Context, _ string, fields []entity.SkorozvonAnketaField) error {
	s.fields = append([]entity.SkorozvonAnketaField(nil), fields...)
	return nil
}

func TestApplyCallResultRequiresLeadID(t *testing.T) {
	err := NewLeadService(&stubLeadList{}, nil).ApplyCallResult(context.Background(), "  ", entity.LeadCallResult{}, entity.LeadFromSkorozvon{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

type stubLeadGet struct {
	stubLeadList
	detail   entity.LeadDetail
	gotID    int64
	gotUUID  string
	dispatch entity.Dispatch
}

func (s *stubLeadGet) Get(_ context.Context, id int64) (entity.LeadDetail, error) {
	s.gotID = id
	return s.detail, nil
}

func (s *stubLeadGet) GetByUUID(_ context.Context, uuid string) (entity.LeadDetail, error) {
	s.gotUUID = uuid
	return s.detail, nil
}

func (s *stubLeadGet) InsertDispatch(_ context.Context, in entity.Dispatch) error {
	s.dispatch = in
	return nil
}

func TestResendPartnerPostsLeadDataAndWritesDispatch(t *testing.T) {
	leads := &stubLeadGet{detail: entity.LeadDetail{
		ID: 7, Phone: "79001234567", PhoneValid: true, Region: "Москва", City: "Химки",
	}}
	partners := &stubPartners{got: entity.Partner{
		ID: 2, Name: "КР", WebhookURL: "https://kr.test/hook",
		Body:        map[string]string{"phone": "{{phone}}", "tag": "{{region}}", "city": "{{city}}"},
		ContentType: entity.PartnerContentTypeForm,
	}}
	poster := &stubPartnerPoster{status: 200, body: "accepted"}
	got, err := NewLeadService(leads, nil).WithPartners(partners, poster).ResendPartner(context.Background(), 7, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !got.SentOK || got.PartnerID != 2 || got.RawResponse != "accepted" {
		t.Fatalf("%+v", got)
	}
	if poster.form.Get("phone") != "79001234567" || poster.form.Get("tag") != "Москва" || poster.form.Get("city") != "Химки" {
		t.Fatalf("form=%v", poster.form)
	}
	if leads.dispatch.LeadRowID != 7 || leads.dispatch.UPKID == nil || *leads.dispatch.UPKID != 2 || !leads.dispatch.SentOK {
		t.Fatalf("dispatch %+v", leads.dispatch)
	}
}

func boolPtr(v bool) *bool { return &v }

func TestResendPartnerRequiresWebhook(t *testing.T) {
	leads := &stubLeadGet{detail: entity.LeadDetail{ID: 7, Phone: "79001234567"}}
	partners := &stubPartners{got: entity.Partner{ID: 2, Name: "КР", WebhookURL: ""}}
	_, err := NewLeadService(leads, nil).WithPartners(partners, &stubPartnerPoster{status: 200}).ResendPartner(context.Background(), 7, "", 2)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}
