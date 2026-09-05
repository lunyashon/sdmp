package usecase

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/id"
	"github.com/lunyashon/sdmp/internal/port"
)

type SkorozvonProjectService struct {
	repo port.SkorozvonProjectRepository
	api  port.Skorozvon
	uuid func() (string, error)
	now  func() time.Time
}

func NewSkorozvonProjectService(repo port.SkorozvonProjectRepository, api port.Skorozvon) *SkorozvonProjectService {
	return &SkorozvonProjectService{
		repo: repo,
		api:  api,
		uuid: id.NewUUID,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

const skorozvonTestPhone = "79800010203"

type SkorozvonProjectInput struct {
	Name      string
	ProjectID int64
}

func (s *SkorozvonProjectService) List(ctx context.Context) ([]entity.SkorozvonProject, error) {
	return s.repo.List(ctx)
}

func (s *SkorozvonProjectService) Create(ctx context.Context, in SkorozvonProjectInput) (entity.SkorozvonProject, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.SkorozvonProject{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	if in.ProjectID <= 0 {
		return entity.SkorozvonProject{}, fmt.Errorf("%w: project_id is required", domain.ErrInvalidInput)
	}

	uid, err := s.uuid()
	if err != nil {
		return entity.SkorozvonProject{}, fmt.Errorf("uuid: %w", err)
	}

	now := s.now()
	ok := s.check(ctx, in.ProjectID)
	return s.repo.Create(ctx, entity.SkorozvonProject{
		UUID:            uid,
		ProjectID:       in.ProjectID,
		Name:            name,
		CreatedAt:       now,
		LastDateRequest: &now,
		LastRequest:     &ok,
	})
}

func (s *SkorozvonProjectService) Refresh(ctx context.Context) ([]entity.SkorozvonProject, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}

	now := s.now()
	out := make([]entity.SkorozvonProject, len(items))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var first error

	wg.Add(len(items))
	for i, item := range items {
		i, item := i, item
		go func() {
			defer wg.Done()
			ok := s.check(ctx, item.ProjectID)
			p, err := s.repo.UpdateCheck(ctx, item.UUID, now, ok)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if first == nil {
					first = err
				}
				return
			}
			out[i] = p
		}()
	}
	wg.Wait()
	if first != nil {
		return nil, first
	}
	return out, nil
}

func (s *SkorozvonProjectService) SendTest(ctx context.Context, uuid string) (int, error) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return 0, fmt.Errorf("%w: project uuid is required", domain.ErrInvalidInput)
	}
	if s.api == nil {
		return 0, fmt.Errorf("skorozvon api is not configured")
	}
	p, err := s.repo.Get(ctx, uuid)
	if err != nil {
		return 0, err
	}
	res, err := s.api.SendLead(ctx, port.SkorozvonLead{
		Phone:     skorozvonTestPhone,
		ProjectID: p.ProjectID,
	})
	if err != nil {
		return 0, err
	}
	return res.LeadID, nil
}

func (s *SkorozvonProjectService) check(ctx context.Context, projectID int64) bool {
	if s.api == nil {
		return false
	}
	return s.api.CheckProject(ctx, projectID)
}
