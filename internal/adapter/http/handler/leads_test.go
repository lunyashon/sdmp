package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/dto"
	httpgin "github.com/lunyashon/sdmp/internal/adapter/http/gin"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/recordinglink"
	"github.com/lunyashon/sdmp/internal/usecase"
)

type stubReceiver struct {
	in  usecase.ReceiveLeadInput
	id  int64
	err error
}

func (s *stubReceiver) Receive(_ context.Context, in usecase.ReceiveLeadInput) (int64, error) {
	s.in = in
	return s.id, s.err
}

type stubLeadFilter struct {
	q         usecase.LeadFilterQuery
	columns   []string
	res       usecase.LeadFilterResult
	export    usecase.LeadExportFile
	importP   usecase.LeadImportPreview
	importN   string
	importB   []byte
	importIn  usecase.LeadImportInput
	importRes usecase.LeadImportResult
	detail    entity.LeadDetail
	raw       usecase.LeadRaw
	rec       usecase.LeadRecording
	err       error
	getErr    error
	skIDs     []int64
	sk        []entity.LeadSkDispatch
}

func (s *stubLeadFilter) Filter(_ context.Context, q usecase.LeadFilterQuery) (usecase.LeadFilterResult, error) {
	s.q = q
	return s.res, s.err
}

func (s *stubLeadFilter) ListSkDispatch(_ context.Context, ids []int64) ([]entity.LeadSkDispatch, error) {
	s.skIDs = ids
	if s.err != nil {
		return nil, s.err
	}
	if s.sk != nil {
		return s.sk, nil
	}
	return []entity.LeadSkDispatch{}, nil
}

func (s *stubLeadFilter) PreviewImport(filename string, body []byte) (usecase.LeadImportPreview, error) {
	s.importN = filename
	s.importB = body
	if s.err != nil {
		return usecase.LeadImportPreview{}, s.err
	}
	if s.importP.Filename != "" {
		return s.importP, nil
	}
	return usecase.LeadImportPreview{
		Filename:  filename,
		RowCount:  1,
		Columns:   []usecase.LeadImportColumn{{Index: 0, Name: "Телефон", Samples: []string{"79001234567"}}},
		Rows:      [][]string{{"79001234567"}},
		Fields:    []usecase.LeadImportField{{ID: "phone", Column: "phone", Label: "Телефон"}},
		Suggested: map[string]string{"0": "phone"},
	}, nil
}

func (s *stubLeadFilter) Import(_ context.Context, in usecase.LeadImportInput) (usecase.LeadImportResult, error) {
	s.importIn = in
	if s.err != nil {
		return usecase.LeadImportResult{}, s.err
	}
	if s.importRes.Queued != 0 || s.importRes.Failed != 0 {
		return s.importRes, nil
	}
	return usecase.LeadImportResult{Queued: len(in.Rows)}, nil
}

func (s *stubLeadFilter) ExportXLSX(_ context.Context, q usecase.LeadFilterQuery, columns []string) (usecase.LeadExportFile, error) {
	s.q = q
	s.columns = columns
	if s.err != nil {
		return usecase.LeadExportFile{}, s.err
	}
	if s.export.Body != nil {
		return s.export, nil
	}
	return usecase.LeadExportFile{
		Filename:    "leads_test.xlsx",
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Body:        []byte("PK"),
	}, nil
}

func (s *stubLeadFilter) IncomingStats(context.Context, string, string) ([]entity.LeadSourceCount, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []entity.LeadSourceCount{{SourceID: 1, Count: 12}}, nil
}

func (s *stubLeadFilter) EnrichmentStats(context.Context, string, string) ([]entity.LeadEnrichmentStat, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []entity.LeadEnrichmentStat{{
		SourceID: 1, Total: 10,
		Region:   entity.LeadGeoShare{Supplier: 6, Provider: 3, Unknown: 1},
		Operator: entity.LeadGeoShare{Supplier: 5, Provider: 4, Unknown: 1},
	}}, nil
}

func (s *stubLeadFilter) ZaryaStats(context.Context, string, string) ([]entity.LeadZaryaStat, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []entity.LeadZaryaStat{{SourceID: 1, Total: 8, Unique: 5, Dup: 3}}, nil
}

func (s *stubLeadFilter) SkorozvonBitrixStats(context.Context, string, string) ([]entity.LeadSkorozvonBitrixStat, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []entity.LeadSkorozvonBitrixStat{{
		SourceID: 1, Updated: 10, UpdatedSent: 4, Created: 6, CreatedSent: 2,
	}}, nil
}

func (s *stubLeadFilter) OutgoingStats(context.Context, string, string) (entity.LeadOutgoingStats, error) {
	if s.err != nil {
		return entity.LeadOutgoingStats{}, s.err
	}
	return entity.LeadOutgoingStats{
		Partners:  []entity.LeadPartnerDispatchCount{{PartnerID: 3, Count: 18}},
		Skorozvon: []entity.LeadSkorozvonDispatchCount{{ProjectID: "50000191177", Count: 9}},
	}, nil
}

func (s *stubLeadFilter) Get(_ context.Context, id int64) (entity.LeadDetail, error) {
	if s.getErr != nil {
		return entity.LeadDetail{}, s.getErr
	}
	if s.detail.ID == 0 {
		s.detail.ID = id
	}
	return s.detail, nil
}

func (s *stubLeadFilter) GetByUUID(_ context.Context, uuid string) (entity.LeadDetail, error) {
	if s.getErr != nil {
		return entity.LeadDetail{}, s.getErr
	}
	if s.detail.UUID == "" {
		s.detail.UUID = uuid
	}
	return s.detail, nil
}

func (s *stubLeadFilter) ResendPartner(_ context.Context, _ int64, _ string, partnerID int) (usecase.PartnerResendResult, error) {
	if s.getErr != nil {
		return usecase.PartnerResendResult{}, s.getErr
	}
	if s.err != nil {
		return usecase.PartnerResendResult{}, s.err
	}
	return usecase.PartnerResendResult{
		PartnerID:   partnerID,
		PartnerName: "КР",
		SentOK:      true,
		HTTPStatus:  200,
		RawResponse: "accepted",
		Attempts:    1,
	}, nil
}

func (s *stubLeadFilter) ResendSkorozvon(_ context.Context, ids []int64) (usecase.SkorozvonResendResult, error) {
	s.skIDs = ids
	if s.err != nil {
		return usecase.SkorozvonResendResult{}, s.err
	}
	return usecase.SkorozvonResendResult{Queued: len(ids), Missing: 0}, nil
}

func (s *stubLeadFilter) Raw(_ context.Context, _ int64, _ string) (usecase.LeadRaw, error) {
	if s.getErr != nil {
		return usecase.LeadRaw{}, s.getErr
	}
	if s.raw.Body == nil {
		return usecase.LeadRaw{}, domain.ErrNotFound
	}
	return s.raw, nil
}

func (s *stubLeadFilter) Recording(_ context.Context, _ int64, _ string) (usecase.LeadRecording, error) {
	if s.getErr != nil {
		return usecase.LeadRecording{}, s.getErr
	}
	if s.rec.Body == nil {
		return usecase.LeadRecording{}, domain.ErrNotFound
	}
	return s.rec, nil
}

func leadsRouter(svc *stubLeadFilter) http.Handler {
	gin.SetMode(gin.TestMode)
	return httpgin.NewRouter(httpgin.Handlers{
		Leads:   handler.NewLeadsHandler(svc, nil),
		Sources: handler.NewSourcesHandler(nil, nil),
		CRMLead: handler.NewCRMLeadHandler(&stubReceiver{id: 1}, nil),
	}, nil)
}

func TestListLeads(t *testing.T) {
	created := time.Date(2026, 8, 27, 16, 27, 56, 0, time.Local)
	svc := &stubLeadFilter{res: usecase.LeadFilterResult{
		Total:  15,
		Limit:  10,
		Offset: 0,
		Leads: []entity.LeadListItem{{
			DateCreate:  created,
			Source:      "GDMP",
			ChildSource: "GDMP / МТС",
			ColdLabel:   "ВР",
			Region:      "Москва",
			Operator:    "БИЛАЙН",
			Stage:       "обогащена",
			Phone:       "79009876539",
			Zarya:       "Чистый",
			ViaUs:       false,
		}},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads?date_from=01.08.2026+00:00:00&date_to=30.08.2026+23:59:59&sources=1&sources=12&child_sources=20&cold_label=ВР&region=1&region=5&operator=2&operator=3&zarya=Чистый&zarya=Дубль&phone=79009876539&via_us=false&status=2&status=4&ids=42&ids=43", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[dto.LeadsResponse](t, w.Body.Bytes())
	if got.Total != 15 || got.Count != 1 || got.Leads[0].Source != "GDMP" || got.Leads[0].ChildSource != "GDMP / МТС" || got.Leads[0].ColdLabel != "ВР" {
		t.Fatalf("%+v", got)
	}
	if got.Leads[0].DateCreate != "27.08.2026 16:27:56" {
		t.Fatalf("date %q", got.Leads[0].DateCreate)
	}
	if len(svc.q.Sources) != 2 || svc.q.Sources[0] != 1 || len(svc.q.ChildSources) != 1 || svc.q.ChildSources[0] != 20 || svc.q.ColdLabels[0] != "ВР" || len(svc.q.Regions) != 2 || svc.q.Regions[1] != 5 {
		t.Fatalf("query %+v", svc.q)
	}
	if len(svc.q.Operators) != 2 || svc.q.Operators[1] != 3 || len(svc.q.Zarya) != 2 || svc.q.Zarya[0] != "Чистый" {
		t.Fatalf("query %+v", svc.q)
	}
	if got.Leads[0].ViaUs {
		t.Fatalf("via_us %+v", got.Leads[0])
	}
	if svc.q.ViaUs == nil || *svc.q.ViaUs {
		t.Fatalf("query via_us %+v", svc.q.ViaUs)
	}
	if len(svc.q.Statuses) != 2 || svc.q.Statuses[0] != 2 || svc.q.Statuses[1] != 4 {
		t.Fatalf("query status %+v", svc.q.Statuses)
	}
	if len(svc.q.IDs) != 2 || svc.q.IDs[0] != 42 || svc.q.IDs[1] != 43 {
		t.Fatalf("query ids %+v", svc.q.IDs)
	}
}

func TestListLeadsSkSendFilter(t *testing.T) {
	ok := false
	status := 429
	svc := &stubLeadFilter{res: usecase.LeadFilterResult{
		Total: 1, Limit: 10,
		Leads: []entity.LeadListItem{{
			ID: 9, UUID: "u-9", DateCreate: time.Date(2026, 9, 21, 12, 0, 0, 0, time.Local),
			SkSentOK: &ok, SkHTTPStatus: &status, SkError: "rate limit",
		}},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads?date_from=01.08.2026+00:00:00&date_to=30.08.2026+23:59:59&sk_send=error&sk_send=fail", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if len(svc.q.SkSend) != 1 || svc.q.SkSend[0] != "error" {
		t.Fatalf("query %+v", svc.q.SkSend)
	}
	got := decodeData[dto.LeadsResponse](t, w.Body.Bytes())
	if got.Leads[0].SkSentOK == nil || *got.Leads[0].SkSentOK || got.Leads[0].SkError != "rate limit" {
		t.Fatalf("%+v", got.Leads[0])
	}
	if got.Leads[0].SkHTTPStatus == nil || *got.Leads[0].SkHTTPStatus != 429 {
		t.Fatalf("%+v", got.Leads[0])
	}
}

func TestListSkDispatch(t *testing.T) {
	status := 200
	svc := &stubLeadFilter{sk: []entity.LeadSkDispatch{{
		LeadRowID: 11, SentOK: true, HTTPStatus: &status,
	}}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/sk-dispatch?ids=11&ids=12&ids=11", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if len(svc.skIDs) != 3 || svc.skIDs[0] != 11 || svc.skIDs[1] != 12 || svc.skIDs[2] != 11 {
		t.Fatalf("ids %+v", svc.skIDs)
	}
	got := decodeData[struct {
		Count int                     `json:"count"`
		Items []dto.LeadSkDispatchDTO `json:"items"`
	}](t, w.Body.Bytes())
	if got.Count != 1 || got.Items[0].ID != 11 || !got.Items[0].SentOK {
		t.Fatalf("%+v", got)
	}
}

func TestExportLeadsXLSX(t *testing.T) {
	svc := &stubLeadFilter{export: usecase.LeadExportFile{
		Filename:    "leads_2026-09-09.xlsx",
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Body:        []byte("PK\x03\x04"),
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/export.xlsx?date_from=01.08.2026+00:00:00&date_to=30.08.2026+23:59:59&ids=42&ids=43&columns=uuid&columns=phone", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != svc.export.ContentType {
		t.Fatalf("type %s", w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "leads_2026-09-09.xlsx") {
		t.Fatalf("disp %s", w.Header().Get("Content-Disposition"))
	}
	if w.Body.String() != "PK\x03\x04" {
		t.Fatalf("body %q", w.Body.String())
	}
	if len(svc.q.IDs) != 2 || svc.q.IDs[0] != 42 || len(svc.columns) != 2 || svc.columns[0] != "uuid" {
		t.Fatalf("query %+v cols %v", svc.q, svc.columns)
	}
}

func TestPreviewImport(t *testing.T) {
	svc := &stubLeadFilter{}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "leads.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("Телефон\n79001234567\n")); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/leads/import/preview", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	leadsRouter(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	got := decodeData[usecase.LeadImportPreview](t, rec.Body.Bytes())
	if got.Filename != "leads.csv" || got.Suggested["0"] != "phone" {
		t.Fatalf("%+v", got)
	}
	if svc.importN != "leads.csv" || !strings.Contains(string(svc.importB), "79001234567") {
		t.Fatalf("svc %q %q", svc.importN, svc.importB)
	}
}

func TestPreviewImportRequiresFile(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/leads/import/preview", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestImportLeads(t *testing.T) {
	svc := &stubLeadFilter{}
	body, err := json.Marshal(dto.LeadImportRequest{
		SourceID: 10,
		Mapping:  map[string]string{"0": "phone"},
		Rows:     [][]string{{"79001234567"}, {"79007654321"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/leads/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	leadsRouter(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	got := decodeData[usecase.LeadImportResult](t, rec.Body.Bytes())
	if got.Queued != 2 || len(svc.importIn.Rows) != 2 || svc.importIn.SourceID != 10 {
		t.Fatalf("%+v in %+v", got, svc.importIn)
	}
}

func TestListLeadsRequiresDates(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestGetLead(t *testing.T) {
	created := time.Date(2026, 8, 27, 16, 27, 56, 0, time.Local)
	srcID := 12
	svc := &stubLeadFilter{detail: entity.LeadDetail{
		ID:          42,
		UUID:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Phone:       "79001234567",
		PhoneValid:  true,
		SourceID:    &srcID,
		Source:      "ВонтРезалт",
		Stage:       "обогащена",
		StatusID:    2,
		CreatedAt:   created,
		CallFormRaw: `{"imya":"Иван","summa_dolga":"150000"}`,
		Events: []entity.LeadEvent{{
			TypeID:     1,
			TypeName:   "принята",
			OccurredAt: created,
			Source:     3,
		}},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/42", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[dto.LeadDetailResponse](t, w.Body.Bytes())
	if got.Lead.ID != 42 || got.Lead.Phone != "79001234567" || got.Lead.Source != "ВонтРезалт" {
		t.Fatalf("%+v", got.Lead)
	}
	form, ok := got.Lead.CallForm.(map[string]any)
	if !ok || form["imya"] != "Иван" {
		t.Fatalf("call_form %+v", got.Lead.CallForm)
	}
	if len(got.Lead.Events) != 1 || got.Lead.Events[0].Type != "принята" {
		t.Fatalf("events %+v", got.Lead.Events)
	}
	if got.Lead.Queues == nil {
		t.Fatalf("queues nil")
	}
	if got.Lead.SkorozvonAnketaFields == nil {
		t.Fatalf("anketa fields nil")
	}
}

func TestGetLeadQueues(t *testing.T) {
	svc := &stubLeadFilter{detail: entity.LeadDetail{
		ID:    42,
		UUID:  "3ad82cac-e095-4f19-8bd3-c055ed52ff80",
		Phone: "79869527036",
		Queues: []entity.LeadOutboxQueue{
			{UUID: "outbox-sk-1", ChannelID: 1, Channel: "Скорозвон", CountRetry: 2},
			{UUID: "outbox-bx-1", ChannelID: 3, Channel: "Битрикс"},
		},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/42", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[dto.LeadDetailResponse](t, w.Body.Bytes())
	if len(got.Lead.Queues) != 2 {
		t.Fatalf("queues %+v", got.Lead.Queues)
	}
	if got.Lead.Queues[0].UUID != "outbox-sk-1" || got.Lead.Queues[0].Channel != "Скорозвон" || got.Lead.Queues[0].Status != "в очереди на отправку" || got.Lead.Queues[0].CountRetry != 2 {
		t.Fatalf("sk %+v", got.Lead.Queues[0])
	}
	if got.Lead.Queues[1].UUID != "outbox-bx-1" || got.Lead.Queues[1].Channel != "Битрикс" || got.Lead.Queues[1].Status != "в очереди на отправку" {
		t.Fatalf("bx %+v", got.Lead.Queues[1])
	}
}

func TestGetLeadAnketaFields(t *testing.T) {
	svc := &stubLeadFilter{detail: entity.LeadDetail{
		ID:    42,
		Phone: "79001234567",
		AnketaFields: []entity.SkorozvonAnketaFieldItem{
			{FieldID: 50000011777, FieldName: "Город клиента", EnumID: 50000004560, EnumName: "Краснодар"},
			{FieldID: 50000011777, FieldName: "Город клиента", EnumID: 50000004578, EnumName: "Сочи"},
			{FieldID: 1, EnumID: 2},
		},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/42", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[dto.LeadDetailResponse](t, w.Body.Bytes())
	if len(got.Lead.SkorozvonAnketaFields) != 2 {
		t.Fatalf("fields %+v", got.Lead.SkorozvonAnketaFields)
	}
	city := got.Lead.SkorozvonAnketaFields[0]
	if city.FieldID != 50000011777 || city.Name != "Город клиента" || strings.Join(city.Values, ",") != "Краснодар,Сочи" {
		t.Fatalf("city %+v", city)
	}
	if got.Lead.SkorozvonAnketaFields[1].Name != "FIELD_1" || got.Lead.SkorozvonAnketaFields[1].Values[0] != "2" {
		t.Fatalf("fallback %+v", got.Lead.SkorozvonAnketaFields[1])
	}
}

func TestGetLeadByUUID(t *testing.T) {
	svc := &stubLeadFilter{detail: entity.LeadDetail{
		ID:    42,
		UUID:  "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Phone: "79001234567",
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[dto.LeadDetailResponse](t, w.Body.Bytes())
	if got.Lead.ID != 42 || got.Lead.UUID != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("%+v", got.Lead)
	}
}

func TestGetLeadNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/9", nil)
	leadsRouter(&stubLeadFilter{getErr: domain.ErrNotFound}).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestIncomingStats(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/incoming-stats?date_from=2026-08-14&date_to=2026-08-20", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[struct {
		Count int                      `json:"count"`
		Items []entity.LeadSourceCount `json:"items"`
	}](t, w.Body.Bytes())
	if got.Count != 1 || got.Items[0].SourceID != 1 || got.Items[0].Count != 12 {
		t.Fatalf("%+v", got)
	}
}

func TestEnrichmentStats(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/enrichment-stats?date_from=2026-08-14&date_to=2026-08-20", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[struct {
		Count int                         `json:"count"`
		Items []entity.LeadEnrichmentStat `json:"items"`
	}](t, w.Body.Bytes())
	if got.Count != 1 || got.Items[0].Region.Supplier != 6 || got.Items[0].Operator.Provider != 4 {
		t.Fatalf("%+v", got)
	}
}

func TestZaryaStats(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/zarya-stats?date_from=2026-08-14&date_to=2026-08-20", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[struct {
		Count int                    `json:"count"`
		Items []entity.LeadZaryaStat `json:"items"`
	}](t, w.Body.Bytes())
	if got.Count != 1 || got.Items[0].Unique != 5 || got.Items[0].Dup != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestSkorozvonBitrixStats(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/skorozvon-bitrix-stats?date_from=2026-08-14&date_to=2026-08-20", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[struct {
		Count int                              `json:"count"`
		Items []entity.LeadSkorozvonBitrixStat `json:"items"`
	}](t, w.Body.Bytes())
	if got.Count != 1 || got.Items[0].Updated != 10 || got.Items[0].CreatedSent != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestOutgoingStats(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/outgoing-stats?date_from=2026-08-14&date_to=2026-08-20", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[entity.LeadOutgoingStats](t, w.Body.Bytes())
	if len(got.Partners) != 1 || got.Partners[0].PartnerID != 3 || got.Partners[0].Count != 18 {
		t.Fatalf("%+v", got)
	}
	if len(got.Skorozvon) != 1 || got.Skorozvon[0].ProjectID != "50000191177" || got.Skorozvon[0].Count != 9 {
		t.Fatalf("%+v", got)
	}
}

func TestGetLeadRaw(t *testing.T) {
	svc := &stubLeadFilter{raw: usecase.LeadRaw{
		Filename:    "9822217c-e2ff-4aa4-b16b-005db28a876c.json",
		ContentType: "application/json",
		Body:        []byte(`{"phone":"79001234567"}`),
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/42/raw", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type %q", w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "9822217c-e2ff-4aa4-b16b-005db28a876c.json") {
		t.Fatalf("disposition %q", w.Header().Get("Content-Disposition"))
	}
	if w.Body.String() != `{"phone":"79001234567"}` {
		t.Fatalf("body %s", w.Body.String())
	}
}

func TestGetLeadRecording(t *testing.T) {
	svc := &stubLeadFilter{rec: usecase.LeadRecording{
		Filename:    "51616666186.mp3",
		ContentType: "audio/mpeg",
		Body:        []byte("ID3"),
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/42/recording", nil)
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "audio/mpeg") {
		t.Fatalf("content-type %q", w.Header().Get("Content-Type"))
	}
	if w.Body.String() != "ID3" {
		t.Fatalf("body %s", w.Body.String())
	}
}

func TestPublicRecordingSigned(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubLeadFilter{rec: usecase.LeadRecording{
		Filename:    "call.mp3",
		ContentType: "audio/mpeg",
		Body:        []byte("ID3"),
	}}
	h := handler.NewLeadsHandler(svc, nil).WithRecordingSecret("sek")
	r := gin.New()
	r.GET("/rest/recordings/:ref", h.PublicRecording)
	id := "9822217c-e2ff-4aa4-b16b-005db28a876c"
	sig := recordinglink.Sign("sek", id)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/recordings/"+id+".mp3?sig="+sig, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "ID3" {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/rest/recordings/"+id+".mp3?sig=bad", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bad sig status %d", w.Code)
	}
}

func TestGetLeadRecordingNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/9/recording", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestGetLeadRawNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/9/raw", nil)
	leadsRouter(&stubLeadFilter{getErr: domain.ErrNotFound}).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestResendPartner(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/leads/42/partners/2/resend", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[usecase.PartnerResendResult](t, w.Body.Bytes())
	if got.PartnerID != 2 || !got.SentOK || got.RawResponse != "accepted" {
		t.Fatalf("%+v", got)
	}
}

func TestResendPartnerBadPartnerID(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/leads/42/partners/0/resend", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestResendSkorozvon(t *testing.T) {
	svc := &stubLeadFilter{}
	body, _ := json.Marshal(dto.SkorozvonResendRequest{IDs: []int64{11, 12}})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/leads/skorozvon/resend", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	leadsRouter(svc).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	got := decodeData[usecase.SkorozvonResendResult](t, w.Body.Bytes())
	if got.Queued != 2 || len(svc.skIDs) != 2 || svc.skIDs[0] != 11 {
		t.Fatalf("%+v ids %+v", got, svc.skIDs)
	}
}

func TestResendSkorozvonBadJSON(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/rest/leads/skorozvon/resend", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestGetLeadBadID(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/leads/abc", nil)
	leadsRouter(&stubLeadFilter{}).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
