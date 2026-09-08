package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/crm"
	"github.com/lunyashon/sdmp/internal/lib/phonesearch"
	"github.com/lunyashon/sdmp/internal/port"
)

type stubOutbox struct {
	msgs        []entity.OutboxMessage
	claimErr    error
	completeErr error
	claimed     int
	completed   []int64
	released    string
}

func (s *stubOutbox) Claim(_ context.Context, _ int, _ time.Duration) (port.OutboxClaim, error) {
	if s.claimErr != nil {
		return port.OutboxClaim{}, s.claimErr
	}
	s.claimed++
	if len(s.msgs) == 0 {
		return port.OutboxClaim{}, nil
	}
	return port.OutboxClaim{Token: "tok-1", Messages: s.msgs}, nil
}

func (s *stubOutbox) Complete(_ context.Context, _ string, id int64) error {
	if s.completeErr != nil {
		return s.completeErr
	}
	s.completed = append(s.completed, id)
	return nil
}

func (s *stubOutbox) Release(_ context.Context, token string) error {
	s.released = token
	return nil
}

type stubOutboxBx24 struct {
	items     []entity.OutboxBx24
	claimErr  error
	limit     int
	stale     time.Duration
	n         int
	completed []string
}

func (s *stubOutboxBx24) Complete(_ context.Context, uuid string) error {
	s.completed = append(s.completed, uuid)
	return nil
}

func (s *stubOutboxBx24) Claim(_ context.Context, limit int, staleAfter time.Duration) ([]entity.OutboxBx24, error) {
	s.n++
	s.limit = limit
	s.stale = staleAfter
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	return s.items, nil
}

func TestProcessBx24ClaimsUnlocked(t *testing.T) {
	box := &stubOutboxBx24{items: []entity.OutboxBx24{
		{UUID: "task-1", LeadUUID: "lead-aaa"},
		{UUID: "task-2", LeadUUID: "lead-bbb"},
	}}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, nil, nil, nil, nil, 1).WithOutboxBx24(box)
	n, err := svc.ProcessBx24(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || box.n != 1 || box.stale != 5*time.Minute {
		t.Fatalf("n=%d calls=%d stale=%s", n, box.n, box.stale)
	}
}

func TestProcessBx24EmptyWithoutRepo(t *testing.T) {
	svc := NewWorkerService(&stubOutbox{}, nil, nil, nil, nil, nil, nil, 1)
	n, err := svc.ProcessBx24(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

type stubBx24 struct {
	id      int64
	raw     []byte
	err     error
	addID   int64
	addRaw  []byte
	addErr  error
	keys    []string
	payload []byte
}

func (s *stubBx24) SearchLead(_ context.Context, key string) (crm.LeadSearchResult, error) {
	s.keys = append(s.keys, key)
	return crm.LeadSearchResult{ID: s.id, Raw: s.raw}, s.err
}

func (s *stubBx24) AddLead(_ context.Context, payload []byte) (crm.LeadAddResult, error) {
	s.payload = append([]byte(nil), payload...)
	return crm.LeadAddResult{ID: s.addID, Raw: s.addRaw, Status: 200}, s.addErr
}

func TestProcessBx24BindsWhenFound(t *testing.T) {
	box := &stubOutboxBx24{items: []entity.OutboxBx24{
		{UUID: "task-1", LeadUUID: "lead-aaa"},
	}}
	leads := &stubLeads{}
	api := &stubBx24{id: 51037210991, raw: []byte(`{"result":51037210991}`)}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)

	n, err := svc.ProcessBx24(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(api.keys) != 1 || api.keys[0] != "lead-aaa" {
		t.Fatalf("keys %v", api.keys)
	}
	if len(leads.bitrixBinds) != 1 || leads.bitrixBinds[0] != [2]string{"lead-aaa", "51037210991"} {
		t.Fatalf("binds %v", leads.bitrixBinds)
	}
	if len(box.completed) != 1 || box.completed[0] != "task-1" {
		t.Fatalf("completed %v", box.completed)
	}
}

func TestProcessBx24AddsWhenMissing(t *testing.T) {
	box := &stubOutboxBx24{items: []entity.OutboxBx24{
		{UUID: "task-1", LeadUUID: "lead-aaa"},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{
		UUID:     "lead-aaa",
		Phone:    "79001234567",
		Region:   "Москва",
		City:     "Химки",
		Operator: "БИЛАЙН",
		AnketaFields: []entity.SkorozvonAnketaFieldItem{
			{FieldName: "Сумма долга", EnumName: "Более 500 тыс."},
			{FieldName: "Вид долга", EnumName: "Потребительские кредиты"},
		},
	}}
	api := &stubBx24{id: 0, raw: []byte(`{"result":0}`), addID: 77, addRaw: []byte(`{"result":77}`)}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)

	n, err := svc.ProcessBx24(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(api.payload) == 0 {
		t.Fatal("add not called")
	}
	if string(api.payload) == "" || !containsAll(string(api.payload), "79001234567", "UF_CRM_1788705605", "lead-aaa", "Москва", "Химки", "БИЛАЙН", "Более 500 тыс.", "Потребительские кредиты") {
		t.Fatalf("payload %s", api.payload)
	}
	if len(leads.bitrixBinds) != 1 || leads.bitrixBinds[0] != [2]string{"lead-aaa", "77"} {
		t.Fatalf("binds %v", leads.bitrixBinds)
	}
	if len(box.completed) != 1 || box.completed[0] != "task-1" {
		t.Fatalf("completed %v", box.completed)
	}
}

func TestProcessBx24AddsEvenWithSkorozvonBitrixField(t *testing.T) {
	box := &stubOutboxBx24{items: []entity.OutboxBx24{
		{UUID: "task-1", LeadUUID: "lead-aaa"},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{
		UUID:  "lead-aaa",
		Phone: "79001234567",
		AnketaFields: []entity.SkorozvonAnketaFieldItem{
			{FieldName: "ID лида Б24", EnumID: 17976350},
			{FieldName: "Сумма долга", EnumName: "300-400 тыс."},
		},
	}}
	api := &stubBx24{id: 0, addID: 77, addRaw: []byte(`{"result":77}`)}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)

	n, err := svc.ProcessBx24(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(api.payload) == 0 {
		t.Fatal("add not called")
	}
	if len(leads.bitrixBinds) != 1 || leads.bitrixBinds[0] != [2]string{"lead-aaa", "77"} {
		t.Fatalf("binds %v", leads.bitrixBinds)
	}
}

func TestProcessBx24KeepsWhenAddFails(t *testing.T) {
	box := &stubOutboxBx24{items: []entity.OutboxBx24{
		{UUID: "task-1", LeadUUID: "lead-aaa"},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{UUID: "lead-aaa", Phone: "79001234567"}}
	api := &stubBx24{id: 0, addErr: errors.New("timeout")}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)

	n, err := svc.ProcessBx24(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(leads.bitrixBinds) != 0 || len(box.completed) != 0 {
		t.Fatalf("binds %v completed %v", leads.bitrixBinds, box.completed)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}

func TestProcessEmpty(t *testing.T) {
	box := &stubOutbox{}
	svc := NewWorkerService(box, nil, nil, nil, nil, nil, nil, 1)
	n, err := svc.Process(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if box.released != "" || len(box.completed) != 0 {
		t.Fatalf("empty claim must not complete/release: %+v", box)
	}
}

func TestProcessCompletesAfterHandle(t *testing.T) {
	box := &stubOutbox{msgs: []entity.OutboxMessage{
		{ID: 1, LeadRowID: 10, SourceID: 2, Payload: `{"fields":{"PHONE":"79001234567"}}`},
		{ID: 2, LeadRowID: 11, SourceID: 2, Payload: `{"fields":{"PHONE":"79001234567"}}`},
	}}
	svc := NewWorkerService(box, nil, nil, nil, nil, nil, nil, 1)
	n, err := svc.Process(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(box.completed) != 2 || box.completed[0] != 1 || box.completed[1] != 2 {
		t.Fatalf("n=%d completed=%v", n, box.completed)
	}
	if box.released != "tok-1" {
		t.Fatalf("release token %q", box.released)
	}
}

func TestProcessInvalidPhoneWritesDispatchAndCompletes(t *testing.T) {
	box := &stubOutbox{msgs: []entity.OutboxMessage{
		{ID: 49, LeadRowID: 12317807, SourceID: 2, Payload: `{"fields":{"PHONE":"dgdfgdfg"}}`},
		{ID: 50, LeadRowID: 11, SourceID: 2, Payload: `{"fields":{"PHONE":"79001234567"}}`},
	}}
	leads := &stubLeads{}
	svc := NewWorkerService(box, nil, nil, leads, nil, nil, nil, 1)
	n, err := svc.Process(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(box.completed) != 2 {
		t.Fatalf("n=%d completed=%v", n, box.completed)
	}
	if len(leads.dispatches) != 1 {
		t.Fatalf("dispatches %+v", leads.dispatches)
	}
	d := leads.dispatches[0]
	if d.LeadRowID != 12317807 || d.SentOK || d.RawResponse != "invalid phone number: dgdfgdfg" {
		t.Fatalf("%+v", d)
	}
}

func TestProcessReleasesOnHandleError(t *testing.T) {
	box := &stubOutbox{msgs: []entity.OutboxMessage{
		{ID: 1, LeadRowID: 10, Payload: `{`},
	}}
	svc := NewWorkerService(box, nil, nil, nil, nil, nil, nil, 1)
	if _, err := svc.Process(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if len(box.completed) != 0 {
		t.Fatal("must not complete")
	}
	if box.released != "tok-1" {
		t.Fatalf("must release, got %q", box.released)
	}
}

func TestProcessCompleteError(t *testing.T) {
	box := &stubOutbox{
		msgs:        []entity.OutboxMessage{{ID: 1, LeadRowID: 10, Payload: `{"fields":{"PHONE":"79001234567"}}`}},
		completeErr: errors.New("disk"),
	}
	svc := NewWorkerService(box, nil, nil, nil, nil, nil, nil, 1)
	if _, err := svc.Process(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if box.released != "tok-1" {
		t.Fatalf("must release leftover, got %q", box.released)
	}
}

type stubDicts struct {
	regions   map[string]int
	operators map[string]int
}

func (s stubDicts) ListRegions(context.Context) ([]entity.NamedRef, error) { return nil, nil }
func (s stubDicts) ListOperators(context.Context) ([]entity.NamedRef, error) {
	out := make([]entity.NamedRef, 0, len(s.operators))
	for name, id := range s.operators {
		out = append(out, entity.NamedRef{ID: id, Name: name})
	}
	return out, nil
}
func (s stubDicts) ListStatuses(context.Context) ([]entity.NamedRef, error) { return nil, nil }
func (s stubDicts) ListOperatorGroups(context.Context) ([]entity.OperatorGroup, error) {
	return nil, nil
}
func (s stubDicts) AddOperatorAlias(context.Context, int, string) error    { return nil }
func (s stubDicts) RemoveOperatorAlias(context.Context, int, string) error { return nil }

func (s stubDicts) ResolveRegion(_ context.Context, alias string) (int, bool, error) {
	id, ok := s.regions[alias]
	return id, ok, nil
}

func (s stubDicts) ResolveOperator(_ context.Context, alias string) (int, bool, error) {
	id, ok := s.operators[alias]
	return id, ok, nil
}

func (s stubDicts) OperatorByName(_ context.Context, name string) (int, bool, error) {
	id, ok := s.operators[name]
	return id, ok, nil
}

type stubLeads struct {
	stubLeadList
	mu             sync.Mutex
	got            entity.LeadEnrichment
	leadID         int64
	dispatches     []entity.Dispatch
	skorozvonBinds [][3]string
	bitrixBinds    [][2]string
	byUUID         entity.LeadDetail
	zaryaLeadID    int64
	zaryaDup       bool
	zaryaSaved     bool
}

func (s *stubLeads) BindSkorozvon(_ context.Context, leadID int64, skorozvonLeadID, projectID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skorozvonBinds = append(s.skorozvonBinds, [3]string{strconv.FormatInt(leadID, 10), skorozvonLeadID, projectID})
	return nil
}

func (s *stubLeads) BindBitrix(_ context.Context, leadUUID, bitrixLeadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bitrixBinds = append(s.bitrixBinds, [2]string{leadUUID, bitrixLeadID})
	return nil
}

func (s *stubLeads) GetByUUID(_ context.Context, uuid string) (entity.LeadDetail, error) {
	d := s.byUUID
	if d.UUID == "" {
		d.UUID = uuid
	}
	return d, nil
}

func (s *stubLeads) UpdateEnrichment(_ context.Context, leadID int64, in entity.LeadEnrichment) error {
	s.leadID = leadID
	s.got = in
	return nil
}

func (s *stubLeads) SaveZaryaResult(_ context.Context, leadID int64, duplicate bool) error {
	s.zaryaLeadID = leadID
	s.zaryaDup = duplicate
	s.zaryaSaved = true
	return nil
}

func (s *stubLeads) InsertDispatch(_ context.Context, in entity.Dispatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatches = append(s.dispatches, in)
	return nil
}

type stubPhoneSearch struct {
	operator  string
	region    string
	duplicate bool
}

func (s stubPhoneSearch) Search(context.Context, string) (phonesearch.Result, error) {
	return phonesearch.Result{Number: phonesearch.NumberInfo{
		Operator: s.operator,
		Region:   s.region,
	}}, nil
}

func (s stubPhoneSearch) CheckDuplicate(context.Context, string) (bool, error) {
	return s.duplicate, nil
}

func TestHandleEnrichesFromLookup(t *testing.T) {
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		stubPhoneSearch{operator: `ПАО "ВЫМПЕЛКОМ"`, region: "Москва и Московская область"},
		stubDicts{
			regions:   map[string]int{"Москва и Московская область": 77},
			operators: map[string]int{"БИЛАЙН": 1, "ДРУГИЕ": 4},
		},
		leads,
		nil,
		nil,
		nil,
		1,
	)
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		ID: 1, LeadRowID: 42, Payload: `{"fields":{"PHONE":"79001234567"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if leads.leadID != 42 || leads.got.RegionID == nil || *leads.got.RegionID != 77 {
		t.Fatalf("region %+v", leads.got)
	}
	if leads.got.RegionSource != entity.GeoSourceLookup {
		t.Fatalf("region source %d", leads.got.RegionSource)
	}
	if leads.got.OperatorID == nil || *leads.got.OperatorID != 1 || leads.got.OperatorSource != entity.GeoSourceLookup {
		t.Fatalf("operator %+v", leads.got)
	}
	if leads.got.StatusID != entity.StatusEnriched {
		t.Fatalf("status %d", leads.got.StatusID)
	}
	if !leads.zaryaSaved || leads.zaryaLeadID != 42 || leads.zaryaDup {
		t.Fatalf("zarya saved=%v id=%d dup=%v", leads.zaryaSaved, leads.zaryaLeadID, leads.zaryaDup)
	}
}

func TestHandleSavesZaryaDuplicate(t *testing.T) {
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		stubPhoneSearch{duplicate: true},
		stubDicts{operators: map[string]int{"ДРУГИЕ": 4}},
		leads,
		nil,
		nil,
		nil,
		1,
	)
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 8,
		Payload:   `{"fields":{"PHONE":"79001234567"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !leads.zaryaSaved || !leads.zaryaDup {
		t.Fatalf("zarya saved=%v dup=%v", leads.zaryaSaved, leads.zaryaDup)
	}
}

func TestHandleKeepsSupplierRegion(t *testing.T) {
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		stubPhoneSearch{operator: `ПАО "МТС"`, region: "ignored"},
		stubDicts{
			regions:   map[string]int{"Краснодарский край": 23},
			operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4},
		},
		leads,
		nil,
		nil,
		nil,
		1,
	)
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		Payload: `{"fields":{"PHONE":"79001234567","UF_CRM_1747216350206":"Краснодарский край","UF_CRM_1734523005552":"ПАО \"МТС\""}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if leads.got.RegionID == nil || *leads.got.RegionID != 23 || leads.got.RegionSource != entity.GeoSourceSupplier {
		t.Fatalf("region %+v", leads.got)
	}
	if leads.got.OperatorID == nil || *leads.got.OperatorID != 2 || leads.got.OperatorSource != entity.GeoSourceSupplier {
		t.Fatalf("operator %+v", leads.got)
	}
}

func TestHandleResolvesOperatorAlias(t *testing.T) {
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{
			operators: map[string]int{`ПАО "МТС"`: 2, "ДРУГИЕ": 4},
		},
		leads,
		nil,
		nil,
		nil,
		1,
	)
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		Payload: `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"ПАО \"МТС\""}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if leads.got.OperatorID == nil || *leads.got.OperatorID != 2 {
		t.Fatalf("operator %+v", leads.got)
	}
}

type stubSplitResolve struct {
	ruleByKey map[string]entity.SplitResolve
	gotSource int
	gotOp     int
}

func (s *stubSplitResolve) List(context.Context) ([]entity.SourceSplit, error) {
	return nil, nil
}
func (s *stubSplitResolve) Get(context.Context, int) (entity.SourceSplit, error) {
	return entity.SourceSplit{}, nil
}
func (s *stubSplitResolve) Replace(context.Context, entity.SourceSplit) (entity.SourceSplit, error) {
	return entity.SourceSplit{}, nil
}
func (s *stubSplitResolve) Resolve(_ context.Context, sourceID, operatorID int) (entity.SplitResolve, bool, error) {
	s.gotSource = sourceID
	s.gotOp = operatorID
	rule, ok := s.ruleByKey[fmt.Sprintf("%d:%d", sourceID, operatorID)]
	return rule, ok, nil
}

func TestHandleAppliesSplitByOperatorAndSource(t *testing.T) {
	leads := &stubLeads{}
	splits := &stubSplitResolve{ruleByKey: map[string]entity.SplitResolve{
		"10:2": {SplitRule: entity.SplitRule{ChildSourceID: 20, ColdLabel: "ice_base_1", BaseLabel: "how_base_1"}},
	}}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4}},
		leads,
		splits,
		nil,
		nil,
		1,
	)
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 7,
		SourceID:  10,
		Payload:   `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"МТС"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if splits.gotSource != 10 || splits.gotOp != 2 {
		t.Fatalf("resolve source=%d operator=%d", splits.gotSource, splits.gotOp)
	}
	if leads.got.ChildSourceID == nil || *leads.got.ChildSourceID != 20 {
		t.Fatalf("child %+v", leads.got)
	}
	if leads.got.ColdLabel != "ice_base_1" || leads.got.BaseLabel != "how_base_1" {
		t.Fatalf("labels %+v", leads.got)
	}
}

func TestHandleLeavesChildEmptyWhenNoSplit(t *testing.T) {
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{operators: map[string]int{"БИЛАЙН": 1, "ДРУГИЕ": 4}},
		leads,
		&stubSplitResolve{},
		nil,
		nil,
		1,
	)
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		SourceID: 10,
		Payload:  `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"БИЛАЙН"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if leads.got.ChildSourceID != nil {
		t.Fatalf("child %+v", leads.got)
	}
}

func TestHandleUsesTestChildWhenTagMisses(t *testing.T) {
	testID := 21
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4}},
		leads,
		&stubSplitResolve{ruleByKey: map[string]entity.SplitResolve{
			"10:2": {
				SplitRule: entity.SplitRule{
					ChildSourceID:     20,
					TestChildSourceID: &testID,
					ColdLabel:         "ice_base_1",
					BaseLabel:         "how_base_1",
				},
				TegActive:        true,
				TegExpectedValue: "подтверждён",
				TestColdLabel:    "ice_test",
				TestBaseLabel:    "how_test",
			},
		}},
		nil,
		nil,
		1,
	)
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		SourceID: 10,
		Payload:  `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"МТС","UF_CRM_1766149282":"нет"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if leads.got.ChildSourceID == nil || *leads.got.ChildSourceID != 21 {
		t.Fatalf("child %+v", leads.got)
	}
	if leads.got.ColdLabel != "ice_test" || leads.got.Tag != "нет" {
		t.Fatalf("labels %+v", leads.got)
	}
}

func TestHandleKeepsChildWhenTagMatches(t *testing.T) {
	testID := 21
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4}},
		leads,
		&stubSplitResolve{ruleByKey: map[string]entity.SplitResolve{
			"10:2": {
				SplitRule: entity.SplitRule{
					ChildSourceID:     20,
					TestChildSourceID: &testID,
					ColdLabel:         "ice_base_1",
					BaseLabel:         "how_base_1",
				},
				TegActive:        true,
				TegExpectedValue: "подтверждён",
			},
		}},
		nil,
		nil,
		1,
	)
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		SourceID: 10,
		Payload:  `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"МТС","UF_CRM_1766149282":"подтверждён"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if leads.got.ChildSourceID == nil || *leads.got.ChildSourceID != 20 {
		t.Fatalf("child %+v", leads.got)
	}
}

type stubRoutePartners struct {
	gotZarya  string
	gotRegion *int
	gotSource int
	items     []entity.Partner
}

func (s *stubRoutePartners) List(context.Context) ([]entity.Partner, error) { return nil, nil }
func (s *stubRoutePartners) Get(context.Context, int) (entity.Partner, error) {
	return entity.Partner{}, nil
}
func (s *stubRoutePartners) Create(context.Context, entity.Partner) (entity.Partner, error) {
	return entity.Partner{}, nil
}
func (s *stubRoutePartners) Update(context.Context, entity.Partner) (entity.Partner, error) {
	return entity.Partner{}, nil
}
func (s *stubRoutePartners) UpdateCheck(context.Context, int, time.Time, bool, int, string) (entity.Partner, error) {
	return entity.Partner{}, nil
}
func (s *stubRoutePartners) ListRoutable(_ context.Context, zarya string, regionID *int, sourceID int) ([]entity.Partner, error) {
	s.gotZarya = zarya
	s.gotRegion = regionID
	s.gotSource = sourceID
	return s.items, nil
}

type stubPoster struct {
	mu       sync.Mutex
	urls     []string
	body     url.Values
	statuses []int
	calls    int
}

func (s *stubPoster) PostForm(_ context.Context, rawURL string, data url.Values, _ string) ([]byte, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urls = append(s.urls, rawURL)
	s.body = data
	status := 200
	if s.calls < len(s.statuses) {
		status = s.statuses[s.calls]
	}
	s.calls++
	return []byte(fmt.Sprintf("status %d", status)), status, nil
}

func TestHandleSendsPartnersInParallel(t *testing.T) {
	partners := &stubRoutePartners{items: []entity.Partner{
		{ID: 1, Name: "РТ", WebhookURL: "https://rt.test/hook", Body: map[string]string{"token": "rt", "phone": "{{phone}}"}},
		{ID: 2, Name: "КР", WebhookURL: "https://kr.test/hook", Body: map[string]string{"auth": "kr", "phone": "{{phone}}", "tag": "{{region}}"}},
	}}
	poster := &stubPoster{statuses: []int{200, 200}}
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4}},
		leads,
		nil,
		partners,
		nil,
		1,
	)
	svc.poster = poster
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 7,
		SourceID:  10,
		Payload:   `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"МТС","UF_CRM_1747216350206":"Москва"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if partners.gotZarya != "unique" || partners.gotSource != 10 {
		t.Fatalf("filter zarya=%s source=%d", partners.gotZarya, partners.gotSource)
	}
	if len(poster.urls) != 2 {
		t.Fatalf("urls %v", poster.urls)
	}
	if poster.body.Get("phone") != "79001234567" {
		t.Fatalf("phone %v", poster.body)
	}
	hasQuery := false
	for _, raw := range poster.urls {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("url %s: %v", raw, err)
		}
		if u.Query().Get("phone") == "79001234567" {
			hasQuery = true
		}
	}
	if !hasQuery {
		t.Fatalf("urls %v", poster.urls)
	}
	if len(leads.dispatches) != 2 || !leads.dispatches[0].SentOK {
		t.Fatalf("dispatch %+v", leads.dispatches)
	}
}

func TestHandlePartner400WritesDispatchAndContinues(t *testing.T) {
	leads := &stubLeads{}
	poster := &stubPoster{statuses: []int{400}}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4}},
		leads,
		nil,
		&stubRoutePartners{items: []entity.Partner{
			{ID: 1, Name: "РТ", WebhookURL: "https://rt.test/hook"},
		}},
		nil,
		1,
	)
	svc.poster = poster
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 9,
		SourceID:  10,
		Payload:   `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"МТС"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if poster.calls != 1 {
		t.Fatalf("retries on 400: calls=%d", poster.calls)
	}
	if len(leads.dispatches) != 1 || leads.dispatches[0].SentOK {
		t.Fatalf("dispatch %+v", leads.dispatches)
	}
	if leads.dispatches[0].HTTPStatus == nil || *leads.dispatches[0].HTTPStatus != 400 {
		t.Fatalf("status %+v", leads.dispatches[0])
	}
	if leads.dispatches[0].Attempts != 1 || leads.dispatches[0].RawResponse == "" {
		t.Fatalf("dispatch %+v", leads.dispatches[0])
	}
}

func TestHandlePartner500RetriesThenDispatch(t *testing.T) {
	leads := &stubLeads{}
	poster := &stubPoster{statuses: []int{500, 500, 500, 500}}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4}},
		leads,
		nil,
		&stubRoutePartners{items: []entity.Partner{
			{ID: 1, Name: "РТ", WebhookURL: "https://rt.test/hook"},
		}},
		nil,
		1,
	)
	svc.poster = poster
	svc.retryWaits = []time.Duration{time.Nanosecond, time.Nanosecond, time.Nanosecond}
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 9,
		SourceID:  10,
		Payload:   `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"МТС"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if poster.calls != 4 {
		t.Fatalf("calls=%d", poster.calls)
	}
	if len(leads.dispatches) != 1 || leads.dispatches[0].SentOK || leads.dispatches[0].Attempts != 4 {
		t.Fatalf("dispatch %+v", leads.dispatches)
	}
}

type stubRouteLinks struct {
	gotKind   string
	gotChild  int
	gotRegion *int
	items     []entity.SkorozvonLink
}

func (s *stubRouteLinks) List(context.Context) ([]entity.SkorozvonLink, error) { return nil, nil }
func (s *stubRouteLinks) Get(context.Context, int) (entity.SkorozvonLink, error) {
	return entity.SkorozvonLink{}, nil
}
func (s *stubRouteLinks) Create(context.Context, entity.SkorozvonLink) (entity.SkorozvonLink, error) {
	return entity.SkorozvonLink{}, nil
}
func (s *stubRouteLinks) Update(context.Context, entity.SkorozvonLink) (entity.SkorozvonLink, error) {
	return entity.SkorozvonLink{}, nil
}
func (s *stubRouteLinks) ListRoutable(_ context.Context, kind string, childSourceID int, regionID *int) ([]entity.SkorozvonLink, error) {
	s.gotKind = kind
	s.gotChild = childSourceID
	s.gotRegion = regionID
	return s.items, nil
}

type stubSkorozvonSend struct {
	mu       sync.Mutex
	calls    []port.SkorozvonLead
	statuses []int
	leadIDs  []int
	n        int
}

func (s *stubSkorozvonSend) SendLead(_ context.Context, lead port.SkorozvonLead) (port.SkorozvonSendResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, lead)
	status := 200
	if s.n < len(s.statuses) {
		status = s.statuses[s.n]
	}
	id := 100 + s.n
	if s.n < len(s.leadIDs) {
		id = s.leadIDs[s.n]
	}
	s.n++
	res := port.SkorozvonSendResult{LeadID: id, Status: status, Body: []byte(`{"id":` + fmt.Sprintf("%d", id) + `}`)}
	if status != 200 {
		res.LeadID = 0
		return res, fmt.Errorf("http %d", status)
	}
	return res, nil
}
func (s *stubSkorozvonSend) DeleteLead(context.Context, int) error { return nil }
func (s *stubSkorozvonSend) CheckProject(context.Context, int64) bool {
	return true
}
func (s *stubSkorozvonSend) ListCustomFields(context.Context) ([]port.SkorozvonCustomField, error) {
	return nil, nil
}

func TestHandleSendsSkorozvonInParallel(t *testing.T) {
	child := 20
	leads := &stubLeads{}
	links := &stubRouteLinks{items: []entity.SkorozvonLink{
		{ID: 1, Name: "Дубли МСК", Type: "dup", ProjectID: 4410},
		{ID: 2, Name: "Дубли Юг", Type: "dup", ProjectID: 4411},
	}}
	api := &stubSkorozvonSend{statuses: []int{200, 200}, leadIDs: []int{501, 502}}
	svc := NewWorkerService(
		&stubOutbox{},
		stubPhoneSearch{duplicate: true},
		stubDicts{operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4}},
		leads,
		&stubSplitResolve{ruleByKey: map[string]entity.SplitResolve{
			"10:2": {SplitRule: entity.SplitRule{ChildSourceID: child}},
		}},
		nil,
		nil,
		1,
	).WithSkorozvon(links, api)

	err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 7,
		LeadUUID:  "11111111-1111-4111-8111-111111111111",
		SourceID:  10,
		Payload:   `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"МТС","UF_CRM_1747216350206":"Москва"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if links.gotKind != "dup" || links.gotChild != child {
		t.Fatalf("filter kind=%s child=%d", links.gotKind, links.gotChild)
	}
	if len(api.calls) != 2 {
		t.Fatalf("calls %d", len(api.calls))
	}
	for _, call := range api.calls {
		if call.ExternalID != "11111111-1111-4111-8111-111111111111" {
			t.Fatalf("external_id %q", call.ExternalID)
		}
	}
	if len(leads.dispatches) != 2 {
		t.Fatalf("dispatch %+v", leads.dispatches)
	}
	for _, d := range leads.dispatches {
		if d.ChannelID != entity.ChannelSkorozvon || !d.SentOK || d.SkorozvonProjectID == "" || d.ExternalID == "" {
			t.Fatalf("dispatch %+v", d)
		}
	}
	if len(leads.skorozvonBinds) != 2 {
		t.Fatalf("binds %+v", leads.skorozvonBinds)
	}
	for _, b := range leads.skorozvonBinds {
		if b[0] != "7" || b[1] == "" || (b[2] != "4410" && b[2] != "4411") {
			t.Fatalf("bind %+v", b)
		}
	}
}

func TestHandleSkorozvon429RetriesThenWritesFailedDispatch(t *testing.T) {
	child := 20
	leads := &stubLeads{}
	api := &stubSkorozvonSend{statuses: []int{429, 429, 429, 429}}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4}},
		leads,
		&stubSplitResolve{ruleByKey: map[string]entity.SplitResolve{
			"10:2": {SplitRule: entity.SplitRule{ChildSourceID: child}},
		}},
		nil,
		nil,
		1,
	).WithSkorozvon(&stubRouteLinks{items: []entity.SkorozvonLink{
		{ID: 1, Name: "Тест", Type: "unique", ProjectID: 9},
	}}, api)
	svc.retryWaits = []time.Duration{time.Nanosecond, time.Nanosecond, time.Nanosecond}

	err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 9,
		SourceID:  10,
		Payload:   `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"МТС"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if api.n != 4 {
		t.Fatalf("calls=%d", api.n)
	}
	if len(leads.dispatches) != 1 {
		t.Fatalf("dispatch %+v", leads.dispatches)
	}
	d := leads.dispatches[0]
	if d.ChannelID != entity.ChannelSkorozvon || d.SentOK || d.SkorozvonProjectID != "9" || d.Attempts != 4 {
		t.Fatalf("dispatch %+v", d)
	}
	if d.HTTPStatus == nil || *d.HTTPStatus != 429 {
		t.Fatalf("status %+v", d.HTTPStatus)
	}
	if len(leads.skorozvonBinds) != 0 {
		t.Fatalf("binds %+v", leads.skorozvonBinds)
	}
}
