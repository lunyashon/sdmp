package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type stubCrmcallsLinks struct {
	items map[int]entity.CrmcallsLink
	next  int
}

func (s *stubCrmcallsLinks) List(context.Context) ([]entity.CrmcallsLink, error) { return nil, nil }
func (s *stubCrmcallsLinks) Get(_ context.Context, id int) (entity.CrmcallsLink, error) {
	return s.items[id], nil
}
func (s *stubCrmcallsLinks) Create(_ context.Context, in entity.CrmcallsLink) (entity.CrmcallsLink, error) {
	s.next++
	in.ID = s.next
	s.items[in.ID] = in
	return in, nil
}
func (s *stubCrmcallsLinks) Update(context.Context, entity.CrmcallsLink) (entity.CrmcallsLink, error) {
	return entity.CrmcallsLink{}, nil
}
func (s *stubCrmcallsLinks) Delete(context.Context, int) error { return nil }
func (s *stubCrmcallsLinks) ListRoutable(context.Context, string, int, *int) ([]entity.CrmcallsLink, error) {
	return nil, nil
}

type stubCrmcallsProjects struct {
	ensured []string
}

func (s *stubCrmcallsProjects) List(context.Context) ([]entity.CrmcallsProject, error) {
	return nil, nil
}
func (s *stubCrmcallsProjects) Upsert(context.Context, []entity.CrmcallsProject) error { return nil }
func (s *stubCrmcallsProjects) Ensure(_ context.Context, id, _ string) error {
	s.ensured = append(s.ensured, id)
	return nil
}
func (s *stubCrmcallsProjects) Get(context.Context, string) (entity.CrmcallsProject, error) {
	return entity.CrmcallsProject{}, domain.ErrNotFound
}
func (s *stubCrmcallsProjects) UpdateCheck(context.Context, string, time.Time, bool, string, string) (entity.CrmcallsProject, error) {
	return entity.CrmcallsProject{}, nil
}

func TestCreateCrmcallsLink(t *testing.T) {
	repo := &stubCrmcallsLinks{items: map[int]entity.CrmcallsLink{}}
	projects := &stubCrmcallsProjects{}
	item, err := NewCrmcallsLinkService(repo, projects).Create(context.Background(), CrmcallsLinkInput{
		Name:           "Поток CRMCalls",
		Type:           "dup",
		ProjectID:      "proj-uuid",
		ChildSourceIDs: []int{12},
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Provider != "crmcalls" || item.ProjectID != "proj-uuid" || len(projects.ensured) != 1 {
		t.Fatalf("%+v ensured=%v", item, projects.ensured)
	}
}

func TestCreateCrmcallsLinkRejectsType(t *testing.T) {
	_, err := NewCrmcallsLinkService(&stubCrmcallsLinks{items: map[int]entity.CrmcallsLink{}}, nil).Create(context.Background(), CrmcallsLinkInput{
		Name:           "x",
		Type:           "all",
		ProjectID:      "p",
		ChildSourceIDs: []int{1},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}
