package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type stubSkorozvonLinks struct {
	items   map[int]entity.SkorozvonLink
	deleted []int
}

func (s *stubSkorozvonLinks) List(context.Context) ([]entity.SkorozvonLink, error) { return nil, nil }
func (s *stubSkorozvonLinks) Get(_ context.Context, id int) (entity.SkorozvonLink, error) {
	item, ok := s.items[id]
	if !ok {
		return entity.SkorozvonLink{}, domain.ErrNotFound
	}
	return item, nil
}
func (s *stubSkorozvonLinks) Create(context.Context, entity.SkorozvonLink) (entity.SkorozvonLink, error) {
	return entity.SkorozvonLink{}, nil
}
func (s *stubSkorozvonLinks) Update(context.Context, entity.SkorozvonLink) (entity.SkorozvonLink, error) {
	return entity.SkorozvonLink{}, nil
}
func (s *stubSkorozvonLinks) Delete(_ context.Context, id int) error {
	if _, ok := s.items[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.items, id)
	s.deleted = append(s.deleted, id)
	return nil
}
func (s *stubSkorozvonLinks) ListRoutable(context.Context, string, int, *int) ([]entity.SkorozvonLink, error) {
	return nil, nil
}

func TestDeleteSkorozvonLink(t *testing.T) {
	repo := &stubSkorozvonLinks{items: map[int]entity.SkorozvonLink{
		7: {ID: 7, Name: "Поток Билайн", Type: "dup", ProjectID: 50000192706},
	}}
	if err := NewSkorozvonLinkService(repo).Delete(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != 7 {
		t.Fatalf("deleted = %v", repo.deleted)
	}
	if _, ok := repo.items[7]; ok {
		t.Fatal("link still present")
	}
}

func TestDeleteSkorozvonLinkRejectsID(t *testing.T) {
	err := NewSkorozvonLinkService(&stubSkorozvonLinks{}).Delete(context.Background(), 0)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteSkorozvonLinkNotFound(t *testing.T) {
	err := NewSkorozvonLinkService(&stubSkorozvonLinks{items: map[int]entity.SkorozvonLink{}}).Delete(context.Background(), 9)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
