package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type GoogleSheetService struct {
	repo port.GoogleSheetRepository
}

func NewGoogleSheetService(repo port.GoogleSheetRepository) *GoogleSheetService {
	return &GoogleSheetService{repo: repo}
}

type GoogleSheetInput struct {
	Name          string
	SpreadsheetID string
	SheetName     string
	IsActive      bool
	TestMode      bool
	ZaryaResults  []string
	RegionIDs     []int
	StopSourceIDs []int
	Credentials   json.RawMessage
}

func (s *GoogleSheetService) List(ctx context.Context) ([]entity.GoogleSheet, error) {
	return s.repo.List(ctx)
}

func (s *GoogleSheetService) Create(ctx context.Context, in GoogleSheetInput) (entity.GoogleSheet, error) {
	item, creds, err := normalizeGoogleSheet(0, in, true)
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	return s.repo.Create(ctx, item, creds)
}

func (s *GoogleSheetService) Update(ctx context.Context, id int, in GoogleSheetInput) (entity.GoogleSheet, error) {
	if id <= 0 {
		return entity.GoogleSheet{}, fmt.Errorf("%w: sheet id is required", domain.ErrInvalidInput)
	}
	item, creds, err := normalizeGoogleSheet(id, in, false)
	if err != nil {
		return entity.GoogleSheet{}, err
	}
	return s.repo.Update(ctx, item, creds)
}

func (s *GoogleSheetService) Delete(ctx context.Context, id int) error {
	if id <= 0 {
		return fmt.Errorf("%w: sheet id is required", domain.ErrInvalidInput)
	}
	return s.repo.Delete(ctx, id)
}

func normalizeGoogleSheet(id int, in GoogleSheetInput, requireCreds bool) (entity.GoogleSheet, json.RawMessage, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.GoogleSheet{}, nil, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	spreadsheetID := strings.TrimSpace(in.SpreadsheetID)
	if spreadsheetID == "" {
		return entity.GoogleSheet{}, nil, fmt.Errorf("%w: spreadsheet_id is required", domain.ErrInvalidInput)
	}
	sheetName := strings.TrimSpace(in.SheetName)
	if sheetName == "" {
		return entity.GoogleSheet{}, nil, fmt.Errorf("%w: sheet_name is required", domain.ErrInvalidInput)
	}
	creds, err := normalizeServiceAccount(in.Credentials, requireCreds)
	if err != nil {
		return entity.GoogleSheet{}, nil, err
	}
	zarya, err := normalizeZarya(in.ZaryaResults)
	if err != nil {
		return entity.GoogleSheet{}, nil, err
	}
	regions, err := uniquePositiveIDs(in.RegionIDs, "region id")
	if err != nil {
		return entity.GoogleSheet{}, nil, err
	}
	stops, err := uniquePositiveIDs(in.StopSourceIDs, "source id")
	if err != nil {
		return entity.GoogleSheet{}, nil, err
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
	}, creds, nil
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
