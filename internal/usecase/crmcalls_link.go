package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type CrmcallsLinkService struct {
	repo     port.CrmcallsLinkRepository
	projects port.CrmcallsProjectRepository
}

func NewCrmcallsLinkService(repo port.CrmcallsLinkRepository, projects port.CrmcallsProjectRepository) *CrmcallsLinkService {
	return &CrmcallsLinkService{repo: repo, projects: projects}
}

type CrmcallsLinkInput struct {
	Name           string
	Type           string
	ProjectID      string
	ChildSourceIDs []int
	RegionIDs      []int
}

func (s *CrmcallsLinkService) List(ctx context.Context) ([]entity.CrmcallsLink, error) {
	return s.repo.List(ctx)
}

func (s *CrmcallsLinkService) Create(ctx context.Context, in CrmcallsLinkInput) (entity.CrmcallsLink, error) {
	item, err := normalizeCrmcallsLink(0, in)
	if err != nil {
		return entity.CrmcallsLink{}, err
	}
	if s.projects != nil {
		if err := s.projects.Ensure(ctx, item.ProjectID, item.Name); err != nil {
			return entity.CrmcallsLink{}, err
		}
	}
	return s.repo.Create(ctx, item)
}

func (s *CrmcallsLinkService) Update(ctx context.Context, id int, in CrmcallsLinkInput) (entity.CrmcallsLink, error) {
	if id <= 0 {
		return entity.CrmcallsLink{}, fmt.Errorf("%w: link id is required", domain.ErrInvalidInput)
	}
	item, err := normalizeCrmcallsLink(id, in)
	if err != nil {
		return entity.CrmcallsLink{}, err
	}
	if s.projects != nil {
		if err := s.projects.Ensure(ctx, item.ProjectID, item.Name); err != nil {
			return entity.CrmcallsLink{}, err
		}
	}
	return s.repo.Update(ctx, item)
}

func (s *CrmcallsLinkService) Delete(ctx context.Context, id int) error {
	if id <= 0 {
		return fmt.Errorf("%w: link id is required", domain.ErrInvalidInput)
	}
	return s.repo.Delete(ctx, id)
}

func normalizeCrmcallsLink(id int, in CrmcallsLinkInput) (entity.CrmcallsLink, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.CrmcallsLink{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	kind := strings.TrimSpace(in.Type)
	if kind != "dup" && kind != "uniq" {
		return entity.CrmcallsLink{}, fmt.Errorf("%w: type must be dup or uniq", domain.ErrInvalidInput)
	}
	projectID := strings.TrimSpace(in.ProjectID)
	if projectID == "" {
		return entity.CrmcallsLink{}, fmt.Errorf("%w: project_id is required", domain.ErrInvalidInput)
	}
	sources, err := uniquePositiveIDs(in.ChildSourceIDs, "source id")
	if err != nil {
		return entity.CrmcallsLink{}, err
	}
	if len(sources) == 0 {
		return entity.CrmcallsLink{}, fmt.Errorf("%w: at least one child source is required", domain.ErrInvalidInput)
	}
	regions, err := uniquePositiveIDs(in.RegionIDs, "region id")
	if err != nil {
		return entity.CrmcallsLink{}, err
	}
	return entity.CrmcallsLink{
		ID:             id,
		Name:           name,
		Type:           kind,
		ProjectID:      projectID,
		Provider:       "crmcalls",
		ChildSourceIDs: sources,
		RegionIDs:      regions,
	}, nil
}
