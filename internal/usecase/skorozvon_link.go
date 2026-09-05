package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type SkorozvonLinkService struct {
	repo port.SkorozvonLinkRepository
}

func NewSkorozvonLinkService(repo port.SkorozvonLinkRepository) *SkorozvonLinkService {
	return &SkorozvonLinkService{repo: repo}
}

type SkorozvonLinkInput struct {
	Name           string
	Type           string
	ProjectID      int64
	ChildSourceIDs []int
	RegionIDs      []int
}

func (s *SkorozvonLinkService) List(ctx context.Context) ([]entity.SkorozvonLink, error) {
	return s.repo.List(ctx)
}

func (s *SkorozvonLinkService) Create(ctx context.Context, in SkorozvonLinkInput) (entity.SkorozvonLink, error) {
	item, err := normalizeSkorozvonLink(0, in)
	if err != nil {
		return entity.SkorozvonLink{}, err
	}
	return s.repo.Create(ctx, item)
}

func (s *SkorozvonLinkService) Update(ctx context.Context, id int, in SkorozvonLinkInput) (entity.SkorozvonLink, error) {
	if id <= 0 {
		return entity.SkorozvonLink{}, fmt.Errorf("%w: link id is required", domain.ErrInvalidInput)
	}
	item, err := normalizeSkorozvonLink(id, in)
	if err != nil {
		return entity.SkorozvonLink{}, err
	}
	return s.repo.Update(ctx, item)
}

func normalizeSkorozvonLink(id int, in SkorozvonLinkInput) (entity.SkorozvonLink, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.SkorozvonLink{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	kind := strings.TrimSpace(in.Type)
	if kind != "dup" && kind != "uniq" {
		return entity.SkorozvonLink{}, fmt.Errorf("%w: type must be dup or uniq", domain.ErrInvalidInput)
	}
	if in.ProjectID <= 0 {
		return entity.SkorozvonLink{}, fmt.Errorf("%w: project_id is required", domain.ErrInvalidInput)
	}
	sources, err := uniquePositiveIDs(in.ChildSourceIDs, "source id")
	if err != nil {
		return entity.SkorozvonLink{}, err
	}
	if len(sources) == 0 {
		return entity.SkorozvonLink{}, fmt.Errorf("%w: at least one child source is required", domain.ErrInvalidInput)
	}
	regions, err := uniquePositiveIDs(in.RegionIDs, "region id")
	if err != nil {
		return entity.SkorozvonLink{}, err
	}
	return entity.SkorozvonLink{
		ID:             id,
		Name:           name,
		Type:           kind,
		ProjectID:      in.ProjectID,
		ChildSourceIDs: sources,
		RegionIDs:      regions,
	}, nil
}
