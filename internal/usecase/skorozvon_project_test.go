package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
	"github.com/lunyashon/sdmp/internal/port"
)

type stubProjectRepo struct {
	items   []entity.SkorozvonProject
	created []entity.SkorozvonProject
}

func (s *stubProjectRepo) List(context.Context) ([]entity.SkorozvonProject, error) {
	return append([]entity.SkorozvonProject(nil), s.items...), nil
}
func (s *stubProjectRepo) ListPage(_ context.Context, offset int, name string) ([]entity.SkorozvonProject, int, error) {
	filtered := make([]entity.SkorozvonProject, 0, len(s.items))
	for _, item := range s.items {
		if name != "" && !strings.Contains(strings.ToLower(item.Name), strings.ToLower(name)) {
			continue
		}
		filtered = append(filtered, item)
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + SkorozvonProjectListLimit
	if end > len(filtered) {
		end = len(filtered)
	}
	return append([]entity.SkorozvonProject(nil), filtered[offset:end]...), len(filtered), nil
}
func (s *stubProjectRepo) Get(_ context.Context, uuid string) (entity.SkorozvonProject, error) {
	for _, item := range s.items {
		if item.UUID == uuid {
			return item, nil
		}
	}
	return entity.SkorozvonProject{}, fmt.Errorf("missing %s", uuid)
}
func (s *stubProjectRepo) Create(_ context.Context, in entity.SkorozvonProject) (entity.SkorozvonProject, error) {
	s.items = append(s.items, in)
	s.created = append(s.created, in)
	return in, nil
}
func (s *stubProjectRepo) CreateIfAbsent(_ context.Context, in entity.SkorozvonProject) (entity.SkorozvonProject, bool, error) {
	for _, item := range s.items {
		if item.ProjectID == in.ProjectID {
			return item, false, nil
		}
	}
	s.items = append(s.items, in)
	s.created = append(s.created, in)
	return in, true, nil
}
func (s *stubProjectRepo) UpdateCheck(_ context.Context, uuid string, at time.Time, ok bool) (entity.SkorozvonProject, error) {
	for i, item := range s.items {
		if item.UUID != uuid {
			continue
		}
		s.items[i].LastDateRequest = &at
		s.items[i].LastRequest = &ok
		return s.items[i], nil
	}
	return entity.SkorozvonProject{}, fmt.Errorf("missing %s", uuid)
}

type stubProjectAPI struct {
	projects []port.SkorozvonCallProject
	err      error
}

func (s stubProjectAPI) SendLead(context.Context, port.SkorozvonLead) (port.SkorozvonSendResult, error) {
	return port.SkorozvonSendResult{}, nil
}
func (s stubProjectAPI) DeleteLead(context.Context, int) error { return nil }
func (s stubProjectAPI) CheckProject(context.Context, int64) bool {
	return true
}
func (s stubProjectAPI) ListCustomFields(context.Context) ([]port.SkorozvonCustomField, error) {
	return nil, nil
}
func (s stubProjectAPI) DownloadCallRecording(context.Context, string) ([]byte, string, error) {
	return nil, "", nil
}
func (s stubProjectAPI) ListCallProjects(context.Context) ([]port.SkorozvonCallProject, error) {
	return s.projects, s.err
}

func TestSyncInsertsMissingProjectsOnly(t *testing.T) {
	repo := &stubProjectRepo{items: []entity.SkorozvonProject{{
		UUID:      "old",
		ProjectID: 1,
		Name:      "Уже есть",
	}}}
	svc := NewSkorozvonProjectService(repo, stubProjectAPI{projects: []port.SkorozvonCallProject{
		{ID: 1, Title: "Переименованный"},
		{ID: 2, Title: "Новый"},
		{ID: 0, Title: "skip"},
	}})
	n := 0
	svc.uuid = func() (string, error) {
		n++
		return fmt.Sprintf("new-%d", n), nil
	}

	got, err := svc.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.created) != 1 || repo.created[0].ProjectID != 2 || repo.created[0].Name != "Новый" {
		t.Fatalf("created %+v", repo.created)
	}
	if len(got) != 2 {
		t.Fatalf("list %+v", got)
	}
	if repo.items[0].Name != "Уже есть" {
		t.Fatalf("existing renamed %+v", repo.items[0])
	}
}

func TestSyncRequiresAPI(t *testing.T) {
	_, err := NewSkorozvonProjectService(&stubProjectRepo{}, nil).Sync(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListPageFiltersByName(t *testing.T) {
	repo := &stubProjectRepo{items: []entity.SkorozvonProject{
		{UUID: "1", ProjectID: 1, Name: "ВР КАБИНЕТ"},
		{UUID: "2", ProjectID: 2, Name: "testest"},
		{UUID: "3", ProjectID: 3, Name: "лидстех"},
	}}
	got, total, err := NewSkorozvonProjectService(repo, nil).ListPage(context.Background(), 0, "test")
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(got) != 1 || got[0].Name != "testest" {
		t.Fatalf("got %+v total %d", got, total)
	}
}

func TestListPageRejectsNegativeOffset(t *testing.T) {
	_, _, err := NewSkorozvonProjectService(&stubProjectRepo{}, nil).ListPage(context.Background(), -1, "")
	if err == nil || !strings.Contains(err.Error(), domain.ErrInvalidInput.Error()) {
		t.Fatalf("err %v", err)
	}
}

func TestRefreshOnlyGivenUUIDs(t *testing.T) {
	repo := &stubProjectRepo{items: []entity.SkorozvonProject{
		{UUID: "a", ProjectID: 1, Name: "один"},
		{UUID: "b", ProjectID: 2, Name: "два"},
	}}
	api := &recordingCheckAPI{okIDs: map[int64]bool{2: true}}
	got, err := NewSkorozvonProjectService(repo, api).Refresh(context.Background(), []string{"b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(api.checked) != 1 || api.checked[0] != 2 {
		t.Fatalf("checked %+v", api.checked)
	}
	if len(got) != 1 || got[0].UUID != "b" || got[0].LastRequest == nil || !*got[0].LastRequest {
		t.Fatalf("got %+v", got)
	}
	if repo.items[0].LastRequest != nil {
		t.Fatalf("other project checked %+v", repo.items[0])
	}
}

type recordingCheckAPI struct {
	stubProjectAPI
	checked []int64
	okIDs   map[int64]bool
}

func (s *recordingCheckAPI) CheckProject(_ context.Context, id int64) bool {
	s.checked = append(s.checked, id)
	return s.okIDs[id]
}

func TestNextSkorozvonProjectsAt(t *testing.T) {
	cases := []struct {
		hour, min int
		wantHour  int
		wantDay   int
	}{
		{11, 59, 12, 8},
		{12, 0, 17, 8},
		{16, 59, 17, 8},
		{17, 0, 12, 9},
		{17, 1, 12, 9},
	}
	for _, tc := range cases {
		now := time.Date(2026, 9, 8, tc.hour, tc.min, 0, 0, clock.Location)
		got := nextSkorozvonProjectsAt(now)
		if got.Hour() != tc.wantHour || got.Day() != tc.wantDay {
			t.Fatalf("%s -> %s, want %02d:00 day %d", now.Format("15:04"), got.Format("02 15:04"), tc.wantHour, tc.wantDay)
		}
		if !got.After(now) {
			t.Fatalf("%s not after %s", got, now)
		}
	}
}
