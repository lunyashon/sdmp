package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

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
