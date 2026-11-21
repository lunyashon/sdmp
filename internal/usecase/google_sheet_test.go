package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type stubGoogleSheets struct {
	items map[int]entity.GoogleSheet
	creds map[int]json.RawMessage
	next  int
}

func (s *stubGoogleSheets) List(context.Context) ([]entity.GoogleSheet, error) {
	out := make([]entity.GoogleSheet, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	return out, nil
}
func (s *stubGoogleSheets) Get(_ context.Context, id int) (entity.GoogleSheet, error) {
	item, ok := s.items[id]
	if !ok {
		return entity.GoogleSheet{}, domain.ErrNotFound
	}
	return item, nil
}
func (s *stubGoogleSheets) Create(_ context.Context, in entity.GoogleSheet, credentials json.RawMessage) (entity.GoogleSheet, error) {
	s.next++
	in.ID = s.next
	in.HasKey = len(credentials) > 0
	s.items[in.ID] = in
	s.creds[in.ID] = credentials
	return in, nil
}
func (s *stubGoogleSheets) Update(_ context.Context, in entity.GoogleSheet, credentials json.RawMessage) (entity.GoogleSheet, error) {
	cur, ok := s.items[in.ID]
	if !ok {
		return entity.GoogleSheet{}, domain.ErrNotFound
	}
	if len(credentials) > 0 {
		s.creds[in.ID] = credentials
		in.HasKey = true
	} else {
		in.HasKey = cur.HasKey
	}
	s.items[in.ID] = in
	return in, nil
}
func (s *stubGoogleSheets) Delete(_ context.Context, id int) error {
	if _, ok := s.items[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.items, id)
	delete(s.creds, id)
	return nil
}
func (s *stubGoogleSheets) ListRoutable(context.Context, string, *int, int) ([]entity.GoogleSheetTarget, error) {
	return nil, nil
}
func (s *stubGoogleSheets) UpdateCheck(context.Context, int, time.Time, bool, int, string) error {
	return nil
}
func (s *stubGoogleSheets) Credentials(_ context.Context, id int) (json.RawMessage, error) {
	raw, ok := s.creds[id]
	if !ok || len(raw) == 0 {
		return nil, domain.ErrNotFound
	}
	return raw, nil
}

func TestCreateGoogleSheetStoresPublicFields(t *testing.T) {
	repo := &stubGoogleSheets{items: map[int]entity.GoogleSheet{}, creds: map[int]json.RawMessage{}}
	item, err := NewGoogleSheetService(repo).Create(context.Background(), GoogleSheetInput{
		Name:          "Телефоны",
		SpreadsheetID: "1qouV48ZEZiyXurBdOEgKXG2n4a53eCnguo7K7YMI0s4",
		SheetName:     "Телефоны",
		IsActive:      true,
		TestMode:      true,
		Credentials:   json.RawMessage(`{"client_email":"dev-service@example.iam.gserviceaccount.com","private_key":"-----BEGIN PRIVATE KEY-----"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "Телефоны" || !item.IsActive || !item.TestMode || !item.HasKey {
		t.Fatalf("%+v", item)
	}
	if string(repo.creds[item.ID]) == "" {
		t.Fatal("credentials not stored")
	}
}

func TestCreateGoogleSheetRejectsEmptyKey(t *testing.T) {
	_, err := NewGoogleSheetService(&stubGoogleSheets{items: map[int]entity.GoogleSheet{}, creds: map[int]json.RawMessage{}}).
		Create(context.Background(), GoogleSheetInput{
			Name:          "Телефоны",
			SpreadsheetID: "abc",
			SheetName:     "Лист",
			Credentials:   json.RawMessage(`{"client_email":"x@y"}`),
		})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateGoogleSheetCopiesExistingAccount(t *testing.T) {
	key := json.RawMessage(`{"client_email":"dev-service@example.iam.gserviceaccount.com","private_key":"-----BEGIN PRIVATE KEY-----"}`)
	repo := &stubGoogleSheets{
		items: map[int]entity.GoogleSheet{1: {ID: 1, Name: "Телефоны", ClientEmail: "dev-service@example.iam.gserviceaccount.com", HasKey: true}},
		creds: map[int]json.RawMessage{1: key},
		next:  1,
	}
	svc := NewGoogleSheetService(repo)
	accounts, err := svc.Accounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].ID != 1 || accounts[0].ClientEmail != "dev-service@example.iam.gserviceaccount.com" {
		t.Fatalf("accounts %+v", accounts)
	}
	item, err := svc.Create(context.Background(), GoogleSheetInput{
		Name:              "Вторая",
		SpreadsheetID:     "abc",
		SheetName:         "Лист",
		CredentialsFromID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(repo.creds[item.ID]) != string(key) {
		t.Fatalf("copied %s", repo.creds[item.ID])
	}
}

type stubSheetsAPI struct {
	status int
}

func (s stubSheetsAPI) HeaderRow(context.Context, json.RawMessage, string, string) (string, []string, []string, error) {
	return "Лиды", []string{"Телефон", "Город"}, []string{"Лиды"}, nil
}

func (s stubSheetsAPI) AppendRow(context.Context, json.RawMessage, string, string, []string) (int, []byte, error) {
	return s.status, []byte("appended"), nil
}

func TestSendGoogleSheetTest(t *testing.T) {
	repo := &stubGoogleSheets{
		items: map[int]entity.GoogleSheet{1: {ID: 1, Name: "Телефоны", SpreadsheetID: "abc", SheetName: "Лист"}},
		creds: map[int]json.RawMessage{1: json.RawMessage(`{"client_email":"a@b","private_key":"k"}`)},
	}
	got, err := NewGoogleSheetService(repo).WithAPI(stubSheetsAPI{status: 200}).SendTest(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.HTTPStatus != 200 || got.Name != "Телефоны" {
		t.Fatalf("%+v", got)
	}
}

func TestGoogleSheetRowUsesPlaceholders(t *testing.T) {
	row := GoogleSheetRow([]entity.GoogleSheetColumn{
		{Header: "Телефон", Value: "{{phone}}"},
		{Header: "Город", Value: "{{city}}"},
		{Header: "Метка", Value: "лид"},
	}, PartnerVars{Phone: "79001112233", City: "Химки", Region: "Москва", PhoneValid: true})
	if strings.Join(row, "|") != "79001112233|Химки|лид" {
		t.Fatalf("%v", row)
	}
	fallback := GoogleSheetRow(nil, PartnerVars{Phone: "7900"})
	if len(fallback) != 1 || fallback[0] != "7900" {
		t.Fatalf("%v", fallback)
	}
}

func TestPreviewGoogleSheet(t *testing.T) {
	repo := &stubGoogleSheets{creds: map[int]json.RawMessage{1: json.RawMessage(`{"client_email":"a@b","private_key":"k"}`)}}
	got, err := NewGoogleSheetService(repo).WithAPI(stubSheetsAPI{status: 200}).Preview(context.Background(), GoogleSheetInput{
		SpreadsheetID:     "https://docs.google.com/spreadsheets/d/abc123/edit",
		CredentialsFromID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.SheetName != "Лиды" || len(got.Columns) != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestUpdateGoogleSheetKeepsKeyWhenOmitted(t *testing.T) {
	repo := &stubGoogleSheets{
		items: map[int]entity.GoogleSheet{1: {ID: 1, Name: "Старый", HasKey: true}},
		creds: map[int]json.RawMessage{1: json.RawMessage(`{"private_key":"keep"}`)},
		next:  1,
	}
	item, err := NewGoogleSheetService(repo).Update(context.Background(), 1, GoogleSheetInput{
		Name:          "Новый",
		SpreadsheetID: "sid",
		SheetName:     "Лист",
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "Новый" || !item.HasKey {
		t.Fatalf("%+v", item)
	}
	if string(repo.creds[1]) != `{"private_key":"keep"}` {
		t.Fatalf("key replaced: %s", repo.creds[1])
	}
}
