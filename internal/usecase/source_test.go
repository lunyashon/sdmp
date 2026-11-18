package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type stubSources struct {
	byID map[int]entity.Source
}

func (s stubSources) List(context.Context) ([]entity.Source, error) { return nil, nil }
func (s stubSources) GetByBitrixCode(context.Context, string) (entity.Source, error) {
	return entity.Source{}, domain.ErrNotFound
}
func (s stubSources) GetByToken(_ context.Context, token string) (entity.Source, error) {
	for _, src := range s.byID {
		if src.Token == token {
			return src, nil
		}
	}
	return entity.Source{}, domain.ErrNotFound
}
func (s stubSources) GetByID(_ context.Context, id int) (entity.Source, error) {
	src, ok := s.byID[id]
	if !ok {
		return entity.Source{}, domain.ErrNotFound
	}
	return src, nil
}
func (s stubSources) ListFamily(_ context.Context, rootID int) ([]entity.Source, error) {
	out := make([]entity.Source, 0)
	root, ok := s.byID[rootID]
	if ok {
		out = append(out, root)
	}
	for _, src := range s.byID {
		if src.ParentID != nil && *src.ParentID == rootID {
			out = append(out, src)
		}
	}
	return out, nil
}
func (s stubSources) SetToken(_ context.Context, id int, token string) error {
	src, ok := s.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	src.Token = token
	s.byID[id] = src
	return nil
}

func (s stubSources) SetActive(_ context.Context, id int, active bool) (entity.Source, error) {
	src, ok := s.byID[id]
	if !ok {
		return entity.Source{}, domain.ErrNotFound
	}
	src.IsActive = active
	s.byID[id] = src
	return src, nil
}

func (s stubSources) Delete(_ context.Context, id int) error {
	if _, ok := s.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.byID, id)
	return nil
}

func (s stubSources) Create(_ context.Context, src entity.Source) (entity.Source, error) {
	id := 1
	for k := range s.byID {
		if k >= id {
			id = k + 1
		}
	}
	src.ID = id
	if !src.IsActive {
		src.IsActive = true
	}
	s.byID[id] = src
	return src, nil
}

func TestCreateTokenForRoot(t *testing.T) {
	repo := stubSources{byID: map[int]entity.Source{
		12: {ID: 12, Name: "ВонтРезалт", BitrixCode: "UC_P8EDM3"},
	}}
	svc := NewSourceService(repo)
	svc.token = func() (string, error) { return "abcdefghijklmnop", nil }

	src, token, err := svc.CreateToken(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if token != "abcdefghijklmnop" || src.ID != 12 || repo.byID[12].Token != token {
		t.Fatalf("src=%+v token=%s stored=%s", src, token, repo.byID[12].Token)
	}
}

func TestDeleteToken(t *testing.T) {
	repo := stubSources{byID: map[int]entity.Source{
		12: {ID: 12, Name: "ВонтРезалт", Token: "abcdefghijklmnop"},
	}}
	if err := NewSourceService(repo).DeleteToken(context.Background(), 12); err != nil {
		t.Fatal(err)
	}
	if repo.byID[12].Token != "" {
		t.Fatalf("token still %q", repo.byID[12].Token)
	}
}

func TestCreateTokenRejectsChild(t *testing.T) {
	parent := 12
	svc := NewSourceService(stubSources{byID: map[int]entity.Source{
		230: {ID: 230, ParentID: &parent, Name: "ВР Билайн", BitrixCode: "UC_MD8A2D"},
	}})
	_, _, err := svc.CreateToken(context.Background(), 230)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestListByTokenReturnsFamily(t *testing.T) {
	parent := 12
	svc := NewSourceService(stubSources{byID: map[int]entity.Source{
		12:  {ID: 12, Name: "ВонтРезалт", BitrixCode: "UC_P8EDM3", Token: "abcdefghijklmnop"},
		230: {ID: 230, ParentID: &parent, Name: "ВР Билайн", BitrixCode: "UC_MD8A2D"},
		231: {ID: 231, ParentID: &parent, Name: "ВР МТС", BitrixCode: "UC_HQ2ZO9"},
	}})

	items, err := svc.ListByToken(context.Background(), "abcdefghijklmnop")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items: %+v", len(items), items)
	}
	ids := map[int]bool{}
	for _, src := range items {
		ids[src.ID] = true
	}
	if !ids[12] || !ids[230] || !ids[231] {
		t.Fatalf("family %+v", items)
	}
}

func TestListByTokenUnknown(t *testing.T) {
	svc := NewSourceService(stubSources{byID: map[int]entity.Source{}})
	_, err := svc.ListByToken(context.Background(), "missingtokenxxxx")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateChild(t *testing.T) {
	repo := stubSources{byID: map[int]entity.Source{
		12: {ID: 12, Name: "ВонтРезалт", BitrixCode: "UC_P8EDM3"},
	}}
	svc := NewSourceService(repo)

	src, err := svc.CreateChild(context.Background(), CreateChildInput{
		ParentID: 12,
		Name:     "  ВР Теле2  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if src.ID == 0 || src.Name != "ВР Теле2" || src.ParentID == nil || *src.ParentID != 12 {
		t.Fatalf("%+v", src)
	}
	if !src.IsActive || src.Token != "" {
		t.Fatalf("child must be active without token: %+v", src)
	}
}

func TestCreateParent(t *testing.T) {
	repo := stubSources{byID: map[int]entity.Source{}}
	src, err := NewSourceService(repo).Create(context.Background(), CreateSourceInput{
		Name:       "  Новый  ",
		BitrixCode: " UC_TEST ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if src.ID == 0 || src.Name != "Новый" || src.ParentID != nil || src.BitrixCode != "UC_TEST" {
		t.Fatalf("%+v", src)
	}
	if !src.IsActive || src.Token != "" {
		t.Fatalf("parent must be active without token: %+v", src)
	}
}

func TestCreateParentRequiresName(t *testing.T) {
	_, err := NewSourceService(stubSources{byID: map[int]entity.Source{}}).Create(context.Background(), CreateSourceInput{Name: "  "})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateChildRejectsNested(t *testing.T) {
	parent := 12
	svc := NewSourceService(stubSources{byID: map[int]entity.Source{
		230: {ID: 230, ParentID: &parent, Name: "ВР Билайн"},
	}})
	_, err := svc.CreateChild(context.Background(), CreateChildInput{ParentID: 230, Name: "внук"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestSetActive(t *testing.T) {
	repo := stubSources{byID: map[int]entity.Source{
		12: {ID: 12, Name: "TECT", IsActive: true},
	}}
	src, err := NewSourceService(repo).SetActive(context.Background(), 12, false)
	if err != nil {
		t.Fatal(err)
	}
	if src.IsActive || repo.byID[12].IsActive {
		t.Fatalf("%+v", src)
	}
}

func TestSetActiveRejectsChild(t *testing.T) {
	parent := 12
	_, err := NewSourceService(stubSources{byID: map[int]entity.Source{
		35: {ID: 35, ParentID: &parent, Name: "ТЕСТ_подисточник", IsActive: true},
	}}).SetActive(context.Background(), 35, false)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateChildRequiresName(t *testing.T) {
	svc := NewSourceService(stubSources{byID: map[int]entity.Source{
		12: {ID: 12, Name: "ВонтРезалт"},
	}})
	_, err := svc.CreateChild(context.Background(), CreateChildInput{ParentID: 12, Name: "  "})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}
