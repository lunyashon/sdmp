package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type GoogleSheetService struct {
	repo port.GoogleSheetRepository
	api  port.GoogleSheets
	now  func() time.Time
}

func NewGoogleSheetService(repo port.GoogleSheetRepository) *GoogleSheetService {
	return &GoogleSheetService{repo: repo, now: time.Now}
}

func (s *GoogleSheetService) WithAPI(api port.GoogleSheets) *GoogleSheetService {
	s.api = api
	return s
}

type GoogleSheetInput struct {
	Name              string
	SpreadsheetID     string
	SheetName         string
	IsActive          bool
	TestMode          bool
	ZaryaResults      []string
	RegionIDs         []int
	StopSourceIDs     []int
	Credentials       json.RawMessage
	CredentialsFromID int
	Columns           []entity.GoogleSheetColumn
}

func (s *GoogleSheetService) List(ctx context.Context) ([]entity.GoogleSheet, error) {
	return s.repo.List(ctx)
}

func (s *GoogleSheetService) Accounts(ctx context.Context) ([]entity.GoogleAccount, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	out := make([]entity.GoogleAccount, 0)
	for _, item := range items {
		email := strings.TrimSpace(item.ClientEmail)
		if !item.HasKey || email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		out = append(out, entity.GoogleAccount{ID: item.ID, ClientEmail: email})
	}
	return out, nil
}

func (s *GoogleSheetService) Create(ctx context.Context, in GoogleSheetInput) (entity.GoogleSheet, error) {
	item, err := normalizeGoogleSheet(0, in)
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	creds, err := s.credentialsFor(ctx, in, true)
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	return s.repo.Create(ctx, item, creds)
}

func (s *GoogleSheetService) Update(ctx context.Context, id int, in GoogleSheetInput) (entity.GoogleSheet, error) {
	if id <= 0 {
		return entity.GoogleSheet{}, fmt.Errorf("%w: sheet id is required", domain.ErrInvalidInput)
	}
	item, err := normalizeGoogleSheet(id, in)
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	creds, err := s.credentialsFor(ctx, in, false)
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	return s.repo.Update(ctx, item, creds)
}

func (s *GoogleSheetService) Preview(ctx context.Context, in GoogleSheetInput) (entity.GoogleSheetPreview, error) {
	if s.api == nil {
		return entity.GoogleSheetPreview{}, fmt.Errorf("google sheets client is not configured")
	}
	spreadsheetID := spreadsheetKey(in.SpreadsheetID)
	if spreadsheetID == "" {
		return entity.GoogleSheetPreview{}, fmt.Errorf("%w: spreadsheet_id is required", domain.ErrInvalidInput)
	}
	creds, err := s.credentialsFor(ctx, in, true)
	if err != nil {
		return entity.GoogleSheetPreview{}, err
	}
	sheetName, columns, sheets, err := s.api.HeaderRow(ctx, creds, spreadsheetID, strings.TrimSpace(in.SheetName))
	if err != nil {
		return entity.GoogleSheetPreview{}, fmt.Errorf("%w: %s", domain.ErrInvalidInput, err.Error())
	}
	if columns == nil {
		columns = []string{}
	}
	if sheets == nil {
		sheets = []string{}
	}
	return entity.GoogleSheetPreview{SheetName: sheetName, Sheets: sheets, Columns: columns}, nil
}

func (s *GoogleSheetService) SendTest(ctx context.Context, id int) (entity.GoogleSheetCheck, error) {
	if id <= 0 {
		return entity.GoogleSheetCheck{}, fmt.Errorf("%w: sheet id is required", domain.ErrInvalidInput)
	}
	sheet, err := s.repo.Get(ctx, id)
	if err != nil {
		return entity.GoogleSheetCheck{}, err
	}
	return s.checkSheet(ctx, sheet)
}

func (s *GoogleSheetService) Refresh(ctx context.Context) ([]entity.GoogleSheetCheck, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]entity.GoogleSheetCheck, 0, len(items))
	for _, item := range items {
		check, err := s.checkSheet(ctx, item)
		if err != nil {
			return nil, err
		}
		out = append(out, check)
	}
	return out, nil
}

func (s *GoogleSheetService) checkSheet(ctx context.Context, sheet entity.GoogleSheet) (entity.GoogleSheetCheck, error) {
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	check := entity.GoogleSheetCheck{ID: sheet.ID, Name: sheet.Name, CheckedAt: now}
	if s.api == nil {
		return entity.GoogleSheetCheck{}, fmt.Errorf("google sheets client is not configured")
	}
	creds, err := s.repo.Credentials(ctx, sheet.ID)
	if err != nil {
		return entity.GoogleSheetCheck{}, err
	}
	values := GoogleSheetRow(sheet.Columns, PartnerVars{
		Phone:      partnerTestPhone,
		PhoneValid: true,
		Region:     "тест",
		City:       "тест",
	})
	if len(values) == 0 {
		return entity.GoogleSheetCheck{}, fmt.Errorf("%w: укажите колонки таблицы", domain.ErrInvalidInput)
	}
	status, raw, err := s.api.AppendRow(ctx, creds, sheet.SpreadsheetID, sheet.SheetName, values)
	check.HTTPStatus = status
	check.OK = err == nil && status >= 200 && status < 300
	check.Response = strings.TrimSpace(string(raw))
	if check.Response == "" && err != nil {
		check.Response = err.Error()
	}
	if saveErr := s.repo.UpdateCheck(ctx, sheet.ID, now, check.OK, status, check.Response); saveErr != nil {
		return entity.GoogleSheetCheck{}, saveErr
	}
	return check, nil
}

func (s *GoogleSheetService) Delete(ctx context.Context, id int) error {
	if id <= 0 {
		return fmt.Errorf("%w: sheet id is required", domain.ErrInvalidInput)
	}
	return s.repo.Delete(ctx, id)
}

func (s *GoogleSheetService) credentialsFor(ctx context.Context, in GoogleSheetInput, required bool) (json.RawMessage, error) {
	if strings.TrimSpace(string(in.Credentials)) != "" && string(in.Credentials) != "null" {
		return normalizeServiceAccount(in.Credentials, true)
	}
	if in.CredentialsFromID > 0 {
		raw, err := s.repo.Credentials(ctx, in.CredentialsFromID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, fmt.Errorf("%w: service account not found", domain.ErrInvalidInput)
			}
			return nil, err
		}
		return normalizeServiceAccount(raw, true)
	}
	if required {
		return nil, fmt.Errorf("%w: service account is required", domain.ErrInvalidInput)
	}
	return nil, nil
}

func normalizeGoogleSheet(id int, in GoogleSheetInput) (entity.GoogleSheet, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.GoogleSheet{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	spreadsheetID := spreadsheetKey(in.SpreadsheetID)
	if spreadsheetID == "" {
		return entity.GoogleSheet{}, fmt.Errorf("%w: spreadsheet_id is required", domain.ErrInvalidInput)
	}
	columns, err := normalizeGoogleColumns(in.Columns)
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	sheetName := strings.TrimSpace(in.SheetName)
	if sheetName == "" {
		return entity.GoogleSheet{}, fmt.Errorf("%w: sheet_name is required", domain.ErrInvalidInput)
	}
	zarya, err := normalizeZarya(in.ZaryaResults)
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	regions, err := uniquePositiveIDs(in.RegionIDs, "region id")
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	stops, err := uniquePositiveIDs(in.StopSourceIDs, "source id")
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	return entity.GoogleSheet{
		ID:            id,
		Name:          name,
		SpreadsheetID: spreadsheetID,
		SheetName:     sheetName,
		IsActive:      in.IsActive,
		TestMode:      in.TestMode,
		ZaryaResults:  zarya,
		RegionIDs:     regions,
		StopSourceIDs: stops,
		Columns:       columns,
	}, nil
}

func normalizeGoogleColumns(in []entity.GoogleSheetColumn) ([]entity.GoogleSheetColumn, error) {
	if len(in) > 60 {
		return nil, fmt.Errorf("%w: too many columns", domain.ErrInvalidInput)
	}
	out := make([]entity.GoogleSheetColumn, 0, len(in))
	for _, col := range in {
		header := strings.TrimSpace(col.Header)
		if header == "" {
			continue
		}
		header = clipRunes(header, 200)
		out = append(out, entity.GoogleSheetColumn{Header: header, Value: clipRunes(strings.TrimSpace(col.Value), 500)})
	}
	return out, nil
}

func spreadsheetKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "/spreadsheets/d/"); i >= 0 {
		raw = raw[i+len("/spreadsheets/d/"):]
		if j := strings.IndexAny(raw, "/?#"); j >= 0 {
			raw = raw[:j]
		}
	}
	return strings.TrimSpace(raw)
}

func clipRunes(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// GoogleSheetRow собирает строку по колонкам. Пустой список колонок — прежняя отправка одного телефона.
func GoogleSheetRow(columns []entity.GoogleSheetColumn, vars PartnerVars) []string {
	if len(columns) == 0 {
		if strings.TrimSpace(vars.Phone) == "" {
			return nil
		}
		return []string{vars.Phone}
	}
	out := make([]string, len(columns))
	for i, col := range columns {
		out[i] = replacePartnerVars(col.Value, vars)
	}
	return out
}

func normalizeServiceAccount(raw json.RawMessage, required bool) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		if required {
			return nil, fmt.Errorf("%w: credentials json is required", domain.ErrInvalidInput)
		}
		return nil, nil
	}
	var creds map[string]any
	if err := json.Unmarshal([]byte(trimmed), &creds); err != nil {
		return nil, fmt.Errorf("%w: credentials must be json", domain.ErrInvalidInput)
	}
	email, _ := creds["client_email"].(string)
	key, _ := creds["private_key"].(string)
	if strings.TrimSpace(email) == "" || strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("%w: credentials need client_email and private_key", domain.ErrInvalidInput)
	}
	out, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("%w: credentials json", domain.ErrInvalidInput)
	}
	return out, nil
}
