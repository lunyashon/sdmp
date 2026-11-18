package usecase

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/clock"
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

const SkorozvonProjectListLimit = 5

func (s *SkorozvonProjectService) List(ctx context.Context) ([]entity.SkorozvonProject, error) {
	return s.repo.List(ctx)
}

func (s *SkorozvonProjectService) ListPage(ctx context.Context, offset int, name string) ([]entity.SkorozvonProject, int, error) {
	if offset < 0 {
		return nil, 0, fmt.Errorf("%w: offset must be >= 0", domain.ErrInvalidInput)
	}
	return s.repo.ListPage(ctx, offset, strings.TrimSpace(name))
}

func (s *SkorozvonProjectService) Sync(ctx context.Context) ([]entity.SkorozvonProject, error) {
	if s.api == nil {
		return nil, fmt.Errorf("skorozvon api is not configured")
	}
	remote, err := s.api.ListCallProjects(ctx)
	if err != nil {
		return nil, err
	}

	now := s.now()
	for _, item := range remote {
		if item.ID <= 0 {
			continue
		}
		name := strings.TrimSpace(item.Title)
		if name == "" {
			name = fmt.Sprintf("PROJECT_%d", item.ID)
		}
		uid, err := s.uuid()
		if err != nil {
			return nil, fmt.Errorf("uuid: %w", err)
		}
		if _, _, err := s.repo.CreateIfAbsent(ctx, entity.SkorozvonProject{
			UUID:      uid,
			ProjectID: item.ID,
			Name:      name,
			CreatedAt: now,
		}); err != nil {
			return nil, err
		}
	}
	return s.repo.List(ctx)
}

func (s *SkorozvonProjectService) Refresh(ctx context.Context, uuids []string) ([]entity.SkorozvonProject, error) {
	items, err := s.refreshTargets(ctx, uuids)
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

func (s *SkorozvonProjectService) refreshTargets(ctx context.Context, uuids []string) ([]entity.SkorozvonProject, error) {
	if len(uuids) == 0 {
		return s.repo.List(ctx)
	}
	out := make([]entity.SkorozvonProject, 0, len(uuids))
	seen := map[string]struct{}{}
	for _, raw := range uuids {
		uuid := strings.TrimSpace(raw)
		if uuid == "" {
			continue
		}
		if _, ok := seen[uuid]; ok {
			continue
		}
		seen[uuid] = struct{}{}
		p, err := s.repo.Get(ctx, uuid)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
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

var skorozvonProjectHours = []int{12, 17}

func (s *WorkerService) WithSkorozvonProjects(projects *SkorozvonProjectService) *WorkerService {
	s.projects = projects
	return s
}

func (s *WorkerService) loopSkorozvonProjects(ctx context.Context) {
	if err := s.SyncSkorozvonProjects(ctx); err != nil {
		s.log.ErrorContext(ctx, "skorozvon projects sync failed", "error", err)
	}
	for {
		waitFor := time.Until(nextSkorozvonProjectsAt(clock.Now()))
		if waitFor < 0 {
			waitFor = 0
		}
		if err := wait(ctx, waitFor); err != nil {
			return
		}
		if err := s.SyncSkorozvonProjects(ctx); err != nil {
			s.log.ErrorContext(ctx, "skorozvon projects sync failed", "error", err)
		}
	}
}

func (s *WorkerService) SyncSkorozvonProjects(ctx context.Context) error {
	if s.projects == nil {
		return nil
	}
	items, err := s.projects.Sync(ctx)
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "skorozvon projects synced", "projects", len(items))
	return nil
}

func nextSkorozvonProjectsAt(now time.Time) time.Time {
	now = now.In(clock.Location)
	for _, hour := range skorozvonProjectHours {
		at := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, clock.Location)
		if at.After(now) {
			return at
		}
	}
	next := now.AddDate(0, 0, 1)
	return time.Date(next.Year(), next.Month(), next.Day(), skorozvonProjectHours[0], 0, 0, 0, clock.Location)
}
