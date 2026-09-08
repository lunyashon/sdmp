package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type memS3 struct {
	files   map[string]string
	puts    int
	deletes int
}

func (s *memS3) Put(_ context.Context, key string, body io.Reader, _ port.PutOptions) error {
	if s.files == nil {
		s.files = map[string]string{}
	}
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.files[key] = string(b)
	s.puts++
	return nil
}
func (s *memS3) Get(_ context.Context, key string) (*port.Object, error) {
	body, ok := s.files[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &port.Object{Key: key, Body: io.NopCloser(bytes.NewReader([]byte(body)))}, nil
}
func (s *memS3) Delete(_ context.Context, key string) error {
	delete(s.files, key)
	s.deletes++
	return nil
}
func (s *memS3) Exists(context.Context, string) (bool, error) { return false, nil }
func (s *memS3) List(context.Context, string) ([]port.ObjectInfo, error) {
	return nil, nil
}
func (s *memS3) Ping(context.Context) error { return nil }

type memRedis struct {
	hashes map[string]map[string]string
	ttls   map[string]time.Duration
	err    error
}

func (m *memRedis) Ping(context.Context) error { return m.err }

func (m *memRedis) HSet(_ context.Context, key string, values map[string]string) error {
	if m.err != nil {
		return m.err
	}
	if m.hashes == nil {
		m.hashes = map[string]map[string]string{}
	}
	h, ok := m.hashes[key]
	if !ok {
		h = map[string]string{}
		m.hashes[key] = h
	}
	for field, value := range values {
		h[field] = value
	}
	return nil
}

func (m *memRedis) HGet(_ context.Context, key, field string) (string, bool, error) {
	if m.err != nil {
		return "", false, m.err
	}
	h, ok := m.hashes[key]
	if !ok {
		return "", false, nil
	}
	val, ok := h[field]
	return val, ok, nil
}

func (m *memRedis) HGetAll(_ context.Context, key string) (map[string]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	h := m.hashes[key]
	if h == nil {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out, nil
}

func (m *memRedis) Expire(_ context.Context, key string, ttl time.Duration) error {
	if m.err != nil {
		return m.err
	}
	if m.ttls == nil {
		m.ttls = map[string]time.Duration{}
	}
	m.ttls[key] = ttl
	return nil
}

func (m *memRedis) Close() error { return nil }

func (m *memRedis) Del(_ context.Context, keys ...string) error {
	if m.err != nil {
		return m.err
	}
	for _, key := range keys {
		delete(m.hashes, key)
		delete(m.ttls, key)
	}
	return nil
}

type stubLeadRepo struct {
	rows []entity.LeadRow
	out  []entity.OutboxMessage
	err  error
	next int64
}

func (s *stubLeadRepo) List(context.Context, port.LeadListFilter) ([]entity.LeadListItem, int, error) {
	return nil, 0, nil
}

func (s *stubLeadRepo) CountBySource(context.Context, time.Time, time.Time) ([]entity.LeadSourceCount, error) {
	return nil, nil
}

func (s *stubLeadRepo) CountEnrichmentBySource(context.Context, time.Time, time.Time) ([]entity.LeadEnrichmentStat, error) {
	return nil, nil
}

func (s *stubLeadRepo) CountZaryaBySource(context.Context, time.Time, time.Time) ([]entity.LeadZaryaStat, error) {
	return nil, nil
}

func (s *stubLeadRepo) Get(context.Context, int64) (entity.LeadDetail, error) {
	return entity.LeadDetail{}, nil
}

func (s *stubLeadRepo) GetByUUID(context.Context, string) (entity.LeadDetail, error) {
	return entity.LeadDetail{}, nil
}

func (s *stubLeadRepo) UpdateEnrichment(context.Context, int64, entity.LeadEnrichment) error {
	return nil
}

func (s *stubLeadRepo) InsertDispatch(context.Context, entity.Dispatch) error {
	return nil
}

func (s *stubLeadRepo) SaveZaryaResult(context.Context, int64, bool) error {
	return nil
}

func (s *stubLeadRepo) BindSkorozvon(context.Context, int64, string, string) error {
	return nil
}

func (s *stubLeadRepo) BindBitrix(context.Context, string, string) error {
	return nil
}

func (s *stubLeadRepo) ApplyCallResult(context.Context, string, entity.LeadCallResult) error {
	return nil
}

func (s *stubLeadRepo) ApplyFormResponse(context.Context, string, entity.LeadFormResponse) error {
	return nil
}

func (s *stubLeadRepo) InsertFromSkorozvon(context.Context, entity.LeadFromSkorozvon) error {
	return nil
}

func (s *stubLeadRepo) InsertAccepted(_ context.Context, row entity.LeadRow, outbox entity.OutboxMessage) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	s.next++
	row.ID = s.next
	outbox.LeadRowID = s.next
	s.rows = append(s.rows, row)
	s.out = append(s.out, outbox)
	return s.next, nil
}

func newReceiveSvc(src entity.Source, leads *stubLeadRepo, storage *memS3) *ReceiveLeadService {
	return newReceiveSvcWithCache(src, leads, storage, &memRedis{})
}

func newReceiveSvcWithCache(src entity.Source, leads *stubLeadRepo, storage *memS3, cache port.Redis) *ReceiveLeadService {
	svc := NewReceiveLeadService(
		stubSources{byID: map[int]entity.Source{src.ID: src}},
		leads,
		storage,
		cache,
		nil,
	)
	svc.uuid = func() (string, error) { return "11111111-1111-4111-8111-111111111111", nil }
	svc.now = func() time.Time { return time.Date(2026, 8, 24, 21, 0, 0, 0, time.UTC) }
	return svc
}

func TestReceiveLeadSuccess(t *testing.T) {
	leads := &stubLeadRepo{}
	storage := &memS3{}
	svc := newReceiveSvc(entity.Source{ID: 10, Token: "hook-token", BitrixCode: "UC_L26K09", IsActive: true}, leads, storage)

	id, err := svc.Receive(context.Background(), ReceiveLeadInput{
		UserID:     "1",
		Token:      "hook-token",
		Phone:      "79001234567",
		SourceCode: "UC_L26K09",
		WebID:      "ext-1",
		Payload: map[string]any{
			"fields": map[string]any{
				"PHONE":                "79001234567",
				"UF_CRM_1734523005552": "mts",
				"UF_CRM_1766149282":    "БФЛ (подтв.)",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("id=%d", id)
	}
	if storage.puts != 1 || storage.files["raw/10/2026/08/11111111-1111-4111-8111-111111111111.json"] == "" {
		t.Fatalf("s3: %+v", storage.files)
	}
	if leads.rows[0].S3Key != "raw/10/2026/08/11111111-1111-4111-8111-111111111111.json" {
		t.Fatalf("s3_key: %s", leads.rows[0].S3Key)
	}
	if len(leads.rows) != 1 || leads.rows[0].SourceID != 10 || leads.rows[0].Phone != "79001234567" {
		t.Fatalf("lead: %+v", leads.rows)
	}
	if leads.rows[0].WebID != "ext-1" {
		t.Fatalf("web_id: %+v", leads.rows[0])
	}
	if len(leads.out) != 1 || leads.out[0].SourceID != 10 || leads.out[0].Payload == "" {
		t.Fatalf("outbox: %+v", leads.out)
	}
	if !strings.Contains(leads.out[0].Payload, `"PHONE":"79001234567"`) || !strings.Contains(leads.out[0].Payload, `"UF_CRM_1734523005552":"mts"`) {
		t.Fatalf("outbox must keep raw body: %s", leads.out[0].Payload)
	}
}

func TestReceiveLeadUnknownToken(t *testing.T) {
	svc := newReceiveSvc(entity.Source{ID: 10, Token: "hook-token", IsActive: true}, &stubLeadRepo{}, &memS3{})
	_, err := svc.Receive(context.Background(), ReceiveLeadInput{
		UserID: "1",
		Token:  "wrong",
		Phone:  "79001234567",
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("got %v", err)
	}
}

func TestReceiveLeadInactiveSource(t *testing.T) {
	svc := newReceiveSvc(entity.Source{ID: 10, Token: "hook-token", IsActive: false}, &stubLeadRepo{}, &memS3{})
	_, err := svc.Receive(context.Background(), ReceiveLeadInput{
		UserID: "1",
		Token:  "hook-token",
		Phone:  "79001234567",
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestReceiveLeadWrongUser(t *testing.T) {
	svc := newReceiveSvc(entity.Source{ID: 10, Token: "hook-token", IsActive: true}, &stubLeadRepo{}, &memS3{})
	_, err := svc.Receive(context.Background(), ReceiveLeadInput{
		UserID: "2",
		Token:  "hook-token",
		Phone:  "79001234567",
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("got %v", err)
	}
}

func TestReceiveLeadRequiresPhone(t *testing.T) {
	svc := newReceiveSvc(entity.Source{ID: 10, Token: "hook-token", IsActive: true}, &stubLeadRepo{}, &memS3{})
	_, err := svc.Receive(context.Background(), ReceiveLeadInput{
		UserID: "1",
		Token:  "hook-token",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestReceiveLeadCleansS3OnDBError(t *testing.T) {
	storage := &memS3{}
	svc := newReceiveSvc(entity.Source{ID: 10, Token: "hook-token", IsActive: true}, &stubLeadRepo{err: errors.New("db down")}, storage)
	_, err := svc.Receive(context.Background(), ReceiveLeadInput{
		UserID: "1",
		Token:  "hook-token",
		Phone:  "79001234567",
		Payload: map[string]any{
			"fields": map[string]any{"TITLE": "x"},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if storage.deletes != 1 || len(storage.files) != 0 {
		t.Fatalf("cleanup failed: puts=%d deletes=%d files=%+v", storage.puts, storage.deletes, storage.files)
	}
}

func TestReceiveLeadIdempotencyHit(t *testing.T) {
	leads := &stubLeadRepo{}
	storage := &memS3{}
	cache := &memRedis{}
	src := entity.Source{ID: 10, Token: "hook-token", IsActive: true, IdempotencyActive: true}
	svc := newReceiveSvcWithCache(src, leads, storage, cache)
	in := ReceiveLeadInput{
		UserID: "1",
		Token:  "hook-token",
		Phone:  "79001234567",
		WebID:  "ext-99",
	}

	first, err := svc.Receive(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Receive(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first != 1 {
		t.Fatalf("first=%d second=%d", first, second)
	}
	if storage.puts != 1 || len(leads.rows) != 1 {
		t.Fatalf("duplicate write: puts=%d rows=%d", storage.puts, len(leads.rows))
	}
	if cache.ttls[idempotencyKey(10, "ext-99")] != 24*time.Hour {
		t.Fatalf("ttl: %+v", cache.ttls)
	}
}

func TestReceiveLeadIdempotencyOffSkipsRedis(t *testing.T) {
	cache := &memRedis{}
	svc := newReceiveSvcWithCache(
		entity.Source{ID: 10, Token: "hook-token", IsActive: true},
		&stubLeadRepo{},
		&memS3{},
		cache,
	)
	_, err := svc.Receive(context.Background(), ReceiveLeadInput{
		UserID: "1",
		Token:  "hook-token",
		Phone:  "79001234567",
		WebID:  "ext-99",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cache.hashes) != 0 {
		t.Fatalf("redis written: %+v", cache.hashes)
	}
}

func TestReceiveLeadIdempotencyEmptyWebID(t *testing.T) {
	leads := &stubLeadRepo{}
	cache := &memRedis{}
	svc := newReceiveSvcWithCache(
		entity.Source{ID: 10, Token: "hook-token", IsActive: true, IdempotencyActive: true},
		leads,
		&memS3{},
		cache,
	)
	id, err := svc.Receive(context.Background(), ReceiveLeadInput{
		UserID: "1",
		Token:  "hook-token",
		Phone:  "79001234567",
	})
	if err != nil || id != 1 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if len(cache.hashes) != 0 {
		t.Fatalf("redis written: %+v", cache.hashes)
	}
}

func TestReceiveLeadIdempotencyRedisError(t *testing.T) {
	storage := &memS3{}
	svc := newReceiveSvcWithCache(
		entity.Source{ID: 10, Token: "hook-token", IsActive: true, IdempotencyActive: true},
		&stubLeadRepo{},
		storage,
		&memRedis{err: errors.New("redis down")},
	)
	_, err := svc.Receive(context.Background(), ReceiveLeadInput{
		UserID: "1",
		Token:  "hook-token",
		Phone:  "79001234567",
		WebID:  "ext-99",
	})
	if err == nil {
		t.Fatal("expected redis error")
	}
	if storage.puts != 0 {
		t.Fatalf("s3 written on redis error: %d", storage.puts)
	}
}

func TestRawObjectKey(t *testing.T) {
	at := time.Date(2026, 8, 24, 21, 0, 0, 0, time.UTC)
	got := rawObjectKey(143, at, "abc-uuid")
	want := "raw/143/2026/08/abc-uuid.json"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestReceiveLeadSourceIDMismatch(t *testing.T) {
	svc := newReceiveSvc(entity.Source{
		ID:         10,
		Token:      "hook-token",
		BitrixCode: "UC_L26K09",
		IsActive:   true,
	}, &stubLeadRepo{}, &memS3{})
	_, err := svc.Receive(context.Background(), ReceiveLeadInput{
		UserID:     "1",
		Token:      "hook-token",
		Phone:      "79001234567",
		SourceCode: "UC_OTHER",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}
