package usecase

import (
	"context"
	"encoding/json"
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
	"github.com/lunyashon/sdmp/internal/lib/recordinglink"
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
	retries   []stubOutboxRetry
}

func (s *stubOutboxBx24) Complete(_ context.Context, uuid string) error {
	s.completed = append(s.completed, uuid)
	return nil
}

func (s *stubOutboxBx24) MarkRetry(_ context.Context, uuid string, countRetry int16, lockFor time.Duration) error {
	s.retries = append(s.retries, stubOutboxRetry{uuid: uuid, countRetry: countRetry, lockFor: lockFor})
	return nil
}

func (s *stubOutboxBx24) Claim(_ context.Context, limit int, lockFor time.Duration) ([]entity.OutboxBx24, error) {
	s.n++
	s.limit = limit
	s.stale = lockFor
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	return s.items, nil
}

type stubOutboxSkorozvon struct {
	items      []entity.OutboxSkorozvon
	enqueued   []string
	claimErr   error
	enqueueErr error
	limit      int
	lockFor    time.Duration
	n          int
	completed  []string
	retries    []stubOutboxRetry
}

type stubOutboxRetry struct {
	uuid       string
	countRetry int16
	lockFor    time.Duration
}

func (s *stubOutboxSkorozvon) Enqueue(_ context.Context, leadUUID string) error {
	if s.enqueueErr != nil {
		return s.enqueueErr
	}
	s.enqueued = append(s.enqueued, leadUUID)
	return nil
}

func (s *stubOutboxSkorozvon) EnqueueResend(_ context.Context, leadUUID string, _ *int64) error {
	return s.Enqueue(context.Background(), leadUUID)
}

func (s *stubOutboxSkorozvon) Complete(_ context.Context, uuid string) error {
	s.completed = append(s.completed, uuid)
	return nil
}

func (s *stubOutboxSkorozvon) MarkRetry(_ context.Context, uuid string, countRetry int16, lockFor time.Duration) error {
	s.retries = append(s.retries, stubOutboxRetry{uuid: uuid, countRetry: countRetry, lockFor: lockFor})
	return nil
}

func (s *stubOutboxSkorozvon) Claim(_ context.Context, limit int, lockFor time.Duration) ([]entity.OutboxSkorozvon, error) {
	s.n++
	s.limit = limit
	s.lockFor = lockFor
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
	leads := &stubLeads{byUUID: entity.LeadDetail{ViaUs: false}}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).WithOutboxBx24(box)
	n, err := svc.ProcessBx24(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || box.n != 1 || box.stale != 2*time.Minute {
		t.Fatalf("n=%d calls=%d stale=%s", n, box.n, box.stale)
	}
	if len(box.completed) != 2 || box.completed[0] != "task-1" || box.completed[1] != "task-2" {
		t.Fatalf("completed %v", box.completed)
	}
}

func TestProcessBx24DropsWhenNotViaUs(t *testing.T) {
	box := &stubOutboxBx24{items: []entity.OutboxBx24{
		{UUID: "task-1", LeadUUID: "lead-aaa"},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{UUID: "lead-aaa", Phone: "79001234567", ViaUs: false}}
	api := &stubBx24{id: 0, addID: 77, addRaw: []byte(`{"result":77}`)}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)

	n, err := svc.ProcessBx24(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(api.keys) != 0 || len(api.payload) != 0 {
		t.Fatalf("bitrix called keys=%v payload=%s", api.keys, api.payload)
	}
	if len(leads.bitrixBinds) != 0 {
		t.Fatalf("binds %v", leads.bitrixBinds)
	}
	if len(box.completed) != 1 || box.completed[0] != "task-1" {
		t.Fatalf("completed %v", box.completed)
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
	id        int64
	raw       []byte
	err       error
	addID     int64
	addRaw    []byte
	addStatus int
	addErr    error
	keys      []string
	payload   []byte
}

func (s *stubBx24) SearchLead(_ context.Context, key string) (crm.LeadSearchResult, error) {
	s.keys = append(s.keys, key)
	return crm.LeadSearchResult{ID: s.id, Raw: s.raw}, s.err
}

func (s *stubBx24) AddLead(_ context.Context, payload []byte) (crm.LeadAddResult, error) {
	s.payload = append([]byte(nil), payload...)
	status := s.addStatus
	if status == 0 && s.addErr == nil {
		status = 200
	}
	return crm.LeadAddResult{ID: s.addID, Raw: s.addRaw, Status: status}, s.addErr
}

func TestProcessBx24BindsWhenFound(t *testing.T) {
	box := &stubOutboxBx24{items: []entity.OutboxBx24{
		{UUID: "task-1", LeadUUID: "lead-aaa"},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{UUID: "lead-aaa", ViaUs: true}}
	api := &stubBx24{id: 51037210991, raw: []byte(`{"result":51037210991}`)}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)
	svc.createBx24 = true

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
		ViaUs:    true,
		AnketaFields: []entity.SkorozvonAnketaFieldItem{
			{FieldName: "Сумма долга", EnumName: "Более 500 тыс."},
			{FieldName: "Вид долга", EnumName: "Потребительские кредиты"},
			{FieldName: "Регион клиента", EnumName: "Москва"},
			{FieldName: "Оператор", EnumName: "Жарина Анастасия Юрьевна"},
		},
	}}
	api := &stubBx24{id: 0, raw: []byte(`{"result":0}`), addID: 77, addRaw: []byte(`{"result":77}`)}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)
	svc.createBx24 = true

	n, err := svc.ProcessBx24(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(api.payload) == 0 {
		t.Fatal("add not called")
	}
	if string(api.payload) == "" || !containsAll(string(api.payload), "79001234567", "UF_CRM_1788705605", "lead-aaa", "Москва", "Химки", "Жарина Анастасия Юрьевна", "Более 500 тыс.", "Потребительские кредиты") {
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
		ViaUs: true,
		AnketaFields: []entity.SkorozvonAnketaFieldItem{
			{FieldName: "ID лида Б24", EnumID: 17976350},
			{FieldName: "Сумма долга", EnumName: "300-400 тыс."},
		},
	}}
	api := &stubBx24{id: 0, addID: 77, addRaw: []byte(`{"result":77}`)}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)
	svc.createBx24 = true

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
	leads := &stubLeads{byUUID: entity.LeadDetail{ID: 42, UUID: "lead-aaa", Phone: "79001234567", ViaUs: true}}
	api := &stubBx24{id: 0, addErr: errors.New("timeout")}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)
	svc.createBx24 = true

	n, err := svc.ProcessBx24(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(leads.bitrixBinds) != 0 || len(box.completed) != 0 {
		t.Fatalf("binds %v completed %v", leads.bitrixBinds, box.completed)
	}
	if len(leads.dispatches) != 1 || leads.dispatches[0].ChannelID != entity.ChannelBitrix || leads.dispatches[0].SentOK {
		t.Fatalf("dispatch %+v", leads.dispatches)
	}
	if leads.dispatches[0].RawResponse != "timeout" {
		t.Fatalf("dispatch %+v", leads.dispatches[0])
	}
	if len(box.retries) != 1 || box.retries[0].uuid != "task-1" || box.retries[0].countRetry != 1 || box.retries[0].lockFor != 5*time.Minute {
		t.Fatalf("retries %+v", box.retries)
	}
}

func TestProcessBx24WritesDispatchWhenAddRejected(t *testing.T) {
	box := &stubOutboxBx24{items: []entity.OutboxBx24{
		{UUID: "task-1", LeadUUID: "lead-aaa"},
	}}
	body := `{"result":false,"error":"Лид с таким телефоном и SOURCE_ID создавался за последние 10 часов"}`
	leads := &stubLeads{byUUID: entity.LeadDetail{ID: 12332286, UUID: "lead-aaa", Phone: "79613523563", ViaUs: true}}
	api := &stubBx24{
		id:        0,
		addStatus: 400,
		addRaw:    []byte(body),
		addErr:    errors.New("bx24 crm.lead.add: status 400: " + body),
	}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)
	svc.createBx24 = true

	n, err := svc.ProcessBx24(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(leads.bitrixBinds) != 0 || len(box.completed) != 0 {
		t.Fatalf("binds %v completed %v", leads.bitrixBinds, box.completed)
	}
	if len(leads.dispatches) != 1 {
		t.Fatalf("dispatch %+v", leads.dispatches)
	}
	d := leads.dispatches[0]
	if d.LeadRowID != 12332286 || d.ChannelID != entity.ChannelBitrix || d.SentOK {
		t.Fatalf("dispatch %+v", d)
	}
	if d.HTTPStatus == nil || *d.HTTPStatus != 400 || d.RawResponse != body {
		t.Fatalf("dispatch %+v", d)
	}
	if len(box.retries) != 1 || box.retries[0].countRetry != 1 || box.retries[0].lockFor != 5*time.Minute {
		t.Fatalf("retries %+v", box.retries)
	}
}

func TestProcessBx24RetriesExhausted(t *testing.T) {
	box := &stubOutboxBx24{items: []entity.OutboxBx24{
		{UUID: "task-1", LeadUUID: "lead-aaa", CountRetry: 5},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{ID: 42, UUID: "lead-aaa", Phone: "79001234567", ViaUs: true}}
	api := &stubBx24{id: 0, addErr: errors.New("timeout")}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxBx24(box).
		WithBx24(api)

	n, err := svc.ProcessBx24(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(box.completed) != 0 {
		t.Fatalf("completed %v", box.completed)
	}
	if len(box.retries) != 1 || box.retries[0].countRetry != 6 || box.retries[0].lockFor != -1 {
		t.Fatalf("retries %+v", box.retries)
	}
}

func TestNextBx24OutboxRetry(t *testing.T) {
	next, delay, exhausted := nextBx24OutboxRetry(0)
	if next != 1 || delay != 5*time.Minute || exhausted {
		t.Fatalf("first %+v %s %v", next, delay, exhausted)
	}
	next, delay, exhausted = nextBx24OutboxRetry(4)
	if next != 5 || delay != 5*time.Minute || exhausted {
		t.Fatalf("last auto %+v %s %v", next, delay, exhausted)
	}
	next, delay, exhausted = nextBx24OutboxRetry(5)
	if next != 6 || delay != 0 || !exhausted {
		t.Fatalf("exhausted %+v %s %v", next, delay, exhausted)
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

func TestProcessContinuesAfterHandleError(t *testing.T) {
	box := &stubOutbox{msgs: []entity.OutboxMessage{
		{ID: 1, LeadRowID: 10, Payload: `{`},
		{ID: 2, LeadRowID: 11, SourceID: 2, Payload: `{"fields":{"PHONE":"79001234567"}}`},
	}}
	svc := NewWorkerService(box, nil, nil, nil, nil, nil, nil, 1)
	n, err := svc.Process(context.Background())
	if err == nil {
		t.Fatal("expected first error")
	}
	if n != 1 || len(box.completed) != 1 || box.completed[0] != 2 {
		t.Fatalf("n=%d completed=%v", n, box.completed)
	}
	if box.released != "tok-1" {
		t.Fatalf("must release leftover, got %q", box.released)
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
	sentProjects   map[string]bool
}

func (s *stubLeads) HasSkorozvonSent(_ context.Context, _ int64, projectID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sentProjects[projectID], nil
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

func (s *stubLeads) GetForSkorozvon(_ context.Context, uuid string) (entity.LeadForSkorozvon, error) {
	d := s.byUUID
	if d.UUID == "" {
		d.UUID = uuid
	}
	return entity.LeadForSkorozvon{
		ID:                d.ID,
		UUID:              d.UUID,
		Phone:             d.Phone,
		ChildSourceID:     d.ChildSourceID,
		Region:            d.Region,
		RegionID:          d.RegionID,
		Zarya:             d.Zarya,
		ColdLabel:         d.ColdLabel,
		SourceDescription: d.SourceDescription,
	}, nil
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

func (s *stubLeads) SaveZaryaOldResult(context.Context, int64, bool) error { return nil }

func (s *stubLeads) InsertDispatch(_ context.Context, in entity.Dispatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatches = append(s.dispatches, in)
	return nil
}

type stubPhoneSearch struct {
	operator  string
	region    string
	territory string
	duplicate bool
}

func (s stubPhoneSearch) Search(context.Context, string) (phonesearch.Result, error) {
	return phonesearch.Result{Number: phonesearch.NumberInfo{
		Operator:  s.operator,
		Region:    s.region,
		Territory: s.territory,
	}}, nil
}

func (s stubPhoneSearch) CheckDuplicate(context.Context, string) (bool, error) {
	return s.duplicate, nil
}

func (s stubPhoneSearch) CheckDuplicateOld(context.Context, string) (bool, bool, error) {
	return false, false, nil
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

func TestHandleUsesTerritoryWhenRegionPlaceholder(t *testing.T) {
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		stubPhoneSearch{operator: `ПАО "МЕГАФОН"`, region: "-", territory: "Самарская область"},
		stubDicts{
			regions:   map[string]int{"Самарская область": 33},
			operators: map[string]int{"МЕГАФОН": 2, "ДРУГИЕ": 4},
		},
		leads,
		nil,
		nil,
		nil,
		1,
	)
	err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 42,
		Payload:   `{"fields":{"PHONE":"79001234567","UF_CRM_1747216350206":"-"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if leads.got.RegionID == nil || *leads.got.RegionID != 33 {
		t.Fatalf("region %+v", leads.got)
	}
	if leads.got.RegionSource != entity.GeoSourceLookup {
		t.Fatalf("region source %d", leads.got.RegionSource)
	}
}

func TestHandlePhoneSearchEmptyUsesOther(t *testing.T) {
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		stubPhoneSearch{},
		stubDicts{operators: map[string]int{"ДРУГИЕ": 4}},
		leads,
		nil,
		nil,
		nil,
		1,
	)
	if err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 9,
		Payload:   `{"fields":{"PHONE":"79409993206"}}`,
	}); err != nil {
		t.Fatal(err)
	}
	if leads.got.StatusID != entity.StatusEnriched {
		t.Fatalf("status %d", leads.got.StatusID)
	}
	if leads.got.OperatorID == nil || *leads.got.OperatorID != 4 || leads.got.OperatorSource != entity.GeoSourceUnknown {
		t.Fatalf("operator %+v", leads.got)
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

func TestHandleAppendsGoogleSheet(t *testing.T) {
	sheets := &stubRouteGoogleSheets{items: []entity.GoogleSheetTarget{
		{ID: 1, Name: "Телефоны", SpreadsheetID: "sheet-id", SheetName: "Телефоны", Credentials: []byte(`{"client_email":"dev@example.com"}`)},
	}}
	api := &stubGoogleAPI{}
	leads := &stubLeads{}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4}},
		leads,
		nil,
		nil,
		nil,
		1,
	).WithGoogleSheets(sheets, api)
	if err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 9,
		SourceID:  12,
		Payload:   `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"МТС","UF_CRM_1747216350206":"Москва"}}`,
	}); err != nil {
		t.Fatal(err)
	}
	if sheets.gotZarya != "unique" || sheets.gotSource != 12 {
		t.Fatalf("filter zarya=%s source=%d", sheets.gotZarya, sheets.gotSource)
	}
	if len(api.phones) != 1 || api.phones[0] != "79001234567" {
		t.Fatalf("phones %v", api.phones)
	}
	if len(leads.dispatches) != 1 || leads.dispatches[0].ChannelID != entity.ChannelGoogle || !leads.dispatches[0].SentOK {
		t.Fatalf("dispatch %+v", leads.dispatches)
	}
}

type stubRouteGoogleSheets struct {
	gotZarya  string
	gotRegion *int
	gotSource int
	items     []entity.GoogleSheetTarget
}

func (s *stubRouteGoogleSheets) List(context.Context) ([]entity.GoogleSheet, error) { return nil, nil }
func (s *stubRouteGoogleSheets) Get(context.Context, int) (entity.GoogleSheet, error) {
	return entity.GoogleSheet{}, nil
}
func (s *stubRouteGoogleSheets) Create(context.Context, entity.GoogleSheet, json.RawMessage) (entity.GoogleSheet, error) {
	return entity.GoogleSheet{}, nil
}
func (s *stubRouteGoogleSheets) Update(context.Context, entity.GoogleSheet, json.RawMessage) (entity.GoogleSheet, error) {
	return entity.GoogleSheet{}, nil
}
func (s *stubRouteGoogleSheets) Delete(context.Context, int) error { return nil }
func (s *stubRouteGoogleSheets) Credentials(context.Context, int) (json.RawMessage, error) {
	return nil, nil
}
func (s *stubRouteGoogleSheets) UpdateCheck(context.Context, int, time.Time, bool, int, string) error {
	return nil
}
func (s *stubRouteGoogleSheets) ListRoutable(_ context.Context, zarya string, regionID *int, sourceID int) ([]entity.GoogleSheetTarget, error) {
	s.gotZarya = zarya
	s.gotRegion = regionID
	s.gotSource = sourceID
	return s.items, nil
}

type stubGoogleAPI struct {
	phones []string
}

func (s *stubGoogleAPI) HeaderRow(context.Context, json.RawMessage, string, string) (string, []string, []string, error) {
	return "", nil, nil, nil
}

func (s *stubGoogleAPI) AppendRow(_ context.Context, _ json.RawMessage, _, _ string, values []string) (int, []byte, error) {
	if len(values) > 0 {
		s.phones = append(s.phones, values[0])
	}
	return 200, []byte(`{"ok":true}`), nil
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
func (s *stubRouteLinks) Delete(context.Context, int) error { return nil }
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
func (s *stubSkorozvonSend) ListCallProjects(context.Context) ([]port.SkorozvonCallProject, error) {
	return nil, nil
}
func (s *stubSkorozvonSend) DownloadCallRecording(context.Context, string) ([]byte, string, error) {
	return nil, "", nil
}

func TestHandleEnqueuesOutboxSkorozvon(t *testing.T) {
	box := &stubOutboxSkorozvon{}
	api := &stubSkorozvonSend{statuses: []int{200}}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		stubDicts{operators: map[string]int{"МТС": 2, "ДРУГИЕ": 4}},
		&stubLeads{},
		nil,
		nil,
		nil,
		1,
	).WithOutboxSkorozvon(box).WithSkorozvon(&stubRouteLinks{items: []entity.SkorozvonLink{
		{ID: 1, Name: "Тест", Type: "uniq", ProjectID: 9},
	}}, api)

	err := svc.Handle(context.Background(), entity.OutboxMessage{
		LeadRowID: 7,
		LeadUUID:  "11111111-1111-4111-8111-111111111111",
		SourceID:  10,
		Payload:   `{"fields":{"PHONE":"79001234567","UF_CRM_1734523005552":"МТС"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(box.enqueued) != 1 || box.enqueued[0] != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("enqueued %+v", box.enqueued)
	}
	if api.n != 0 {
		t.Fatalf("skorozvon called %d", api.n)
	}
}

func TestProcessSkorozvonSendsAndCompletes(t *testing.T) {
	box := &stubOutboxSkorozvon{items: []entity.OutboxSkorozvon{
		{UUID: "task-1", LeadUUID: "lead-aaa"},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{
		ID: 7, UUID: "lead-aaa", Phone: "79001234567", Region: "Москва", Zarya: "Чистый",
	}}
	api := &stubSkorozvonSend{statuses: []int{200}, leadIDs: []int{501}}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxSkorozvon(box).
		WithSkorozvon(&stubRouteLinks{items: []entity.SkorozvonLink{
			{ID: 1, Name: "Уник", Type: "uniq", ProjectID: 9},
		}}, api)

	n, err := svc.ProcessSkorozvon(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || box.n != 1 || box.lockFor != 2*time.Minute {
		t.Fatalf("n=%d calls=%d lock=%s", n, box.n, box.lockFor)
	}
	if api.n != 1 {
		t.Fatalf("skorozvon calls %d", api.n)
	}
	if len(box.completed) != 1 || box.completed[0] != "task-1" {
		t.Fatalf("completed %v", box.completed)
	}
	if len(box.retries) != 0 {
		t.Fatalf("retries %v", box.retries)
	}
}

func TestProcessSkorozvonImportSendsOnlyToProject(t *testing.T) {
	projectID := int64(50000191177)
	box := &stubOutboxSkorozvon{items: []entity.OutboxSkorozvon{
		{UUID: "task-1", LeadUUID: "lead-aaa", ProjectID: &projectID},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{
		ID: 7, UUID: "lead-aaa", Phone: "79001234567", Region: "Москва",
	}}
	links := &stubRouteLinks{items: []entity.SkorozvonLink{
		{ID: 1, Name: "Уник", Type: "uniq", ProjectID: 9},
		{ID: 2, Name: "Дубли", Type: "dup", ProjectID: 10},
	}}
	api := &stubSkorozvonSend{statuses: []int{200}, leadIDs: []int{501}}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxSkorozvon(box).
		WithSkorozvon(links, api)

	n, err := svc.ProcessSkorozvon(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if links.gotKind != "" {
		t.Fatalf("import must not route by links, got kind %q", links.gotKind)
	}
	if api.n != 1 || len(api.calls) != 1 || api.calls[0].ProjectID != projectID {
		t.Fatalf("calls %+v", api.calls)
	}
	if len(box.completed) != 1 || box.completed[0] != "task-1" {
		t.Fatalf("completed %v", box.completed)
	}
	if len(leads.skorozvonBinds) != 1 || leads.skorozvonBinds[0][2] != "50000191177" {
		t.Fatalf("binds %v", leads.skorozvonBinds)
	}
}

func TestProcessSkorozvonImportWithoutLinks(t *testing.T) {
	projectID := int64(88)
	box := &stubOutboxSkorozvon{items: []entity.OutboxSkorozvon{
		{UUID: "task-1", LeadUUID: "lead-aaa", ProjectID: &projectID},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{ID: 7, UUID: "lead-aaa", Phone: "79001234567"}}
	api := &stubSkorozvonSend{statuses: []int{200}, leadIDs: []int{601}}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxSkorozvon(box).
		WithSkorozvon(&stubRouteLinks{}, api)

	n, err := svc.ProcessSkorozvon(context.Background())
	if err != nil || n != 1 || api.n != 1 || api.calls[0].ProjectID != 88 {
		t.Fatalf("n=%d err=%v calls %+v", n, err, api.calls)
	}
	if len(box.completed) != 1 {
		t.Fatalf("completed %v", box.completed)
	}
}

func TestProcessSkorozvonSchedulesOutboxRetry(t *testing.T) {
	box := &stubOutboxSkorozvon{items: []entity.OutboxSkorozvon{
		{UUID: "task-1", LeadUUID: "lead-aaa", CountRetry: 0},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{ID: 7, UUID: "lead-aaa", Phone: "79001234567"}}
	api := &stubSkorozvonSend{statuses: []int{429, 429, 429, 429, 429}}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxSkorozvon(box).
		WithSkorozvon(&stubRouteLinks{items: []entity.SkorozvonLink{
			{ID: 1, Name: "Тест", Type: "uniq", ProjectID: 9},
		}}, api)
	svc.skorozvonWaits = []time.Duration{time.Nanosecond, time.Nanosecond, time.Nanosecond, time.Nanosecond}

	n, err := svc.ProcessSkorozvon(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(box.completed) != 0 {
		t.Fatalf("completed %v", box.completed)
	}
	if len(box.retries) != 1 || box.retries[0].uuid != "task-1" || box.retries[0].countRetry != 1 || box.retries[0].lockFor != time.Minute {
		t.Fatalf("retries %+v", box.retries)
	}
}

func TestProcessSkorozvonExhaustsRetries(t *testing.T) {
	box := &stubOutboxSkorozvon{items: []entity.OutboxSkorozvon{
		{UUID: "task-1", LeadUUID: "lead-aaa", CountRetry: 5},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{ID: 7, UUID: "lead-aaa", Phone: "79001234567"}}
	api := &stubSkorozvonSend{statuses: []int{500, 500, 500, 500, 500}}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxSkorozvon(box).
		WithSkorozvon(&stubRouteLinks{items: []entity.SkorozvonLink{
			{ID: 1, Name: "Тест", Type: "uniq", ProjectID: 9},
		}}, api)
	svc.skorozvonWaits = []time.Duration{time.Nanosecond}

	n, err := svc.ProcessSkorozvon(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(box.completed) != 0 {
		t.Fatalf("completed %v", box.completed)
	}
	if len(box.retries) != 1 || box.retries[0].countRetry != 6 || box.retries[0].lockFor != -1 {
		t.Fatalf("retries %+v", box.retries)
	}
}

func TestNextSkorozvonOutboxRetry(t *testing.T) {
	next, delay, exhausted := nextSkorozvonOutboxRetry(0)
	if next != 1 || delay != time.Minute || exhausted {
		t.Fatalf("first %+v %s %v", next, delay, exhausted)
	}
	next, delay, exhausted = nextSkorozvonOutboxRetry(4)
	if next != 5 || delay != 30*time.Minute || exhausted {
		t.Fatalf("last auto %+v %s %v", next, delay, exhausted)
	}
	next, delay, exhausted = nextSkorozvonOutboxRetry(5)
	if next != 6 || delay != 0 || !exhausted {
		t.Fatalf("exhausted %+v %s %v", next, delay, exhausted)
	}
}

func TestSendToSkorozvonInParallel(t *testing.T) {
	child := 20
	leads := &stubLeads{}
	links := &stubRouteLinks{items: []entity.SkorozvonLink{
		{ID: 1, Name: "Дубли МСК", Type: "dup", ProjectID: 4410},
		{ID: 2, Name: "Дубли Юг", Type: "dup", ProjectID: 4411},
	}}
	api := &stubSkorozvonSend{statuses: []int{200, 200}, leadIDs: []int{501, 502}}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		nil,
		leads,
		nil,
		nil,
		nil,
		1,
	).WithSkorozvon(links, api)

	err := svc.sendToSkorozvon(context.Background(), entity.LeadForSkorozvon{
		ID: 7, UUID: "11111111-1111-4111-8111-111111111111", Phone: "79001234567",
		ChildSourceID: &child, Region: "Москва",
	}, true)
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

func TestSendToSkorozvonSkipsAlreadySentProject(t *testing.T) {
	child := 20
	leads := &stubLeads{sentProjects: map[string]bool{"4410": true}}
	links := &stubRouteLinks{items: []entity.SkorozvonLink{
		{ID: 1, Name: "Дубли МСК", Type: "dup", ProjectID: 4410},
		{ID: 2, Name: "Дубли Юг", Type: "dup", ProjectID: 4411},
	}}
	api := &stubSkorozvonSend{statuses: []int{200}, leadIDs: []int{502}}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		nil,
		leads,
		nil,
		nil,
		nil,
		1,
	).WithSkorozvon(links, api)

	err := svc.sendToSkorozvon(context.Background(), entity.LeadForSkorozvon{
		ID: 7, UUID: "11111111-1111-4111-8111-111111111111", Phone: "79001234567",
		ChildSourceID: &child, Region: "Москва",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if api.n != 1 {
		t.Fatalf("calls=%d", api.n)
	}
	if api.calls[0].ProjectID != 4411 {
		t.Fatalf("project %d", api.calls[0].ProjectID)
	}
	if len(leads.dispatches) != 1 || leads.dispatches[0].SkorozvonProjectID != "4411" {
		t.Fatalf("dispatch %+v", leads.dispatches)
	}
	if len(leads.skorozvonBinds) != 1 || leads.skorozvonBinds[0][2] != "4411" {
		t.Fatalf("binds %+v", leads.skorozvonBinds)
	}
}

func TestRetrySkorozvon(t *testing.T) {
	if !retrySkorozvon(429, nil) || !retrySkorozvon(500, nil) || !retrySkorozvon(0, errors.New("net")) {
		t.Fatal("retryable")
	}
	if retrySkorozvon(422, errors.New("http 422")) || retrySkorozvon(400, nil) || retrySkorozvon(200, nil) {
		t.Fatal("permanent")
	}
}

func TestSendToSkorozvon422DoesNotRetry(t *testing.T) {
	child := 20
	leads := &stubLeads{}
	api := &stubSkorozvonSend{statuses: []int{422, 422, 422, 422, 422}}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		nil,
		leads,
		nil,
		nil,
		nil,
		1,
	).WithSkorozvon(&stubRouteLinks{items: []entity.SkorozvonLink{
		{ID: 1, Name: "Тест", Type: "unique", ProjectID: 9},
	}}, api)
	svc.skorozvonWaits = []time.Duration{time.Nanosecond, time.Nanosecond, time.Nanosecond}

	err := svc.sendToSkorozvon(context.Background(), entity.LeadForSkorozvon{
		ID: 9, UUID: "lead-9", Phone: "79001234567", ChildSourceID: &child,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if api.n != 1 {
		t.Fatalf("calls=%d", api.n)
	}
	if len(leads.dispatches) != 1 || leads.dispatches[0].SentOK {
		t.Fatalf("dispatch %+v", leads.dispatches)
	}
	if leads.dispatches[0].HTTPStatus == nil || *leads.dispatches[0].HTTPStatus != 422 {
		t.Fatalf("status %+v", leads.dispatches[0].HTTPStatus)
	}
}

func TestProcessSkorozvon422CompletesOutbox(t *testing.T) {
	box := &stubOutboxSkorozvon{items: []entity.OutboxSkorozvon{
		{UUID: "task-1", LeadUUID: "lead-aaa"},
	}}
	leads := &stubLeads{byUUID: entity.LeadDetail{ID: 7, UUID: "lead-aaa", Phone: "79001234567"}}
	api := &stubSkorozvonSend{statuses: []int{422}}
	svc := NewWorkerService(&stubOutbox{}, nil, nil, leads, nil, nil, nil, 1).
		WithOutboxSkorozvon(box).
		WithSkorozvon(&stubRouteLinks{items: []entity.SkorozvonLink{
			{ID: 1, Name: "Тест", Type: "uniq", ProjectID: 9},
		}}, api)

	n, err := svc.ProcessSkorozvon(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if api.n != 1 {
		t.Fatalf("calls=%d", api.n)
	}
	if len(box.completed) != 1 {
		t.Fatalf("completed %v retries %+v", box.completed, box.retries)
	}
}

func TestSendToSkorozvon429RetriesThenWritesFailedDispatch(t *testing.T) {
	child := 20
	leads := &stubLeads{}
	api := &stubSkorozvonSend{statuses: []int{429, 429, 429, 429}}
	svc := NewWorkerService(
		&stubOutbox{},
		nil,
		nil,
		leads,
		nil,
		nil,
		nil,
		1,
	).WithSkorozvon(&stubRouteLinks{items: []entity.SkorozvonLink{
		{ID: 1, Name: "Тест", Type: "unique", ProjectID: 9},
	}}, api)
	svc.skorozvonWaits = []time.Duration{time.Nanosecond, time.Nanosecond, time.Nanosecond}

	err := svc.sendToSkorozvon(context.Background(), entity.LeadForSkorozvon{
		ID: 9, UUID: "lead-9", Phone: "79001234567", ChildSourceID: &child,
	}, false)
	if err == nil {
		t.Fatal("expected send error")
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

func TestBitrixCallURLProxy(t *testing.T) {
	const id = "9822217c-e2ff-4aa4-b16b-005db28a876c"
	s := (&WorkerService{}).WithRecordingLink("https://cdp-dmp.ru", "sek")
	got := s.bitrixCallURL(entity.LeadDetail{UUID: id, CallID: "51639650467"})
	want := recordinglink.URL("https://cdp-dmp.ru", "sek", id)
	if got != want {
		t.Fatalf("got %q", got)
	}
	if s.bitrixCallURL(entity.LeadDetail{UUID: id, CallID: "call-abc"}) != "" {
		t.Fatal("crmcalls without url")
	}
	plain := &WorkerService{}
	if plain.bitrixCallURL(entity.LeadDetail{UUID: id, CallID: "1"}) != "" {
		t.Fatal("no secret")
	}
}
