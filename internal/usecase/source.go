package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/lib/id"
	"github.com/lunyashon/sdmp/internal/port"
)

const maxTokenAttempts = 8

type SourceService struct {
	repo  port.SourceRepository
	token func() (string, error)
}

func NewSourceService(repo port.SourceRepository) *SourceService {
	return &SourceService{repo: repo, token: id.NewWebhookToken}
}

type CreateChildInput struct {
	ParentID   int
	Name       string
	BitrixCode string
	ColdLabel  string
	BaseLabel  string
}

type CreateSourceInput struct {
	Name       string
	BitrixCode string
	ColdLabel  string
	BaseLabel  string
}

func (s *SourceService) List(ctx context.Context) ([]entity.Source, error) {
	return s.repo.List(ctx)
}

func (s *SourceService) Create(ctx context.Context, in CreateSourceInput) (entity.Source, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.Source{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	return s.repo.Create(ctx, entity.Source{
		Name:       name,
		BitrixCode: strings.TrimSpace(in.BitrixCode),
		ColdLabel:  strings.TrimSpace(in.ColdLabel),
		BaseLabel:  strings.TrimSpace(in.BaseLabel),
		IsActive:   true,
	})
}

func (s *SourceService) CreateChild(ctx context.Context, in CreateChildInput) (entity.Source, error) {
	if in.ParentID <= 0 {
		return entity.Source{}, fmt.Errorf("%w: parent id is required", domain.ErrInvalidInput)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.Source{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}

	parent, err := s.repo.GetByID(ctx, in.ParentID)
	if err != nil {
		return entity.Source{}, err
	}
	if parent.ParentID != nil {
		return entity.Source{}, fmt.Errorf("%w: child source cannot have children", domain.ErrInvalidInput)
	}

	parentID := parent.ID
	return s.repo.Create(ctx, entity.Source{
		ParentID:   &parentID,
		Name:       name,
		BitrixCode: strings.TrimSpace(in.BitrixCode),
		ColdLabel:  strings.TrimSpace(in.ColdLabel),
		BaseLabel:  strings.TrimSpace(in.BaseLabel),
		IsActive:   true,
	})
}

func (s *SourceService) CreateToken(ctx context.Context, sourceID int) (entity.Source, string, error) {
	if sourceID <= 0 {
		return entity.Source{}, "", fmt.Errorf("%w: source id is required", domain.ErrInvalidInput)
	}

	src, err := s.repo.GetByID(ctx, sourceID)
	if err != nil {
		return entity.Source{}, "", err
	}
	if src.ParentID != nil {
		return entity.Source{}, "", fmt.Errorf("%w: token is only for root sources", domain.ErrInvalidInput)
	}

	var token string
	for i := 0; i < maxTokenAttempts; i++ {
		token, err = s.token()
		if err != nil {
			return entity.Source{}, "", fmt.Errorf("generate token: %w", err)
		}
		_, err = s.repo.GetByToken(ctx, token)
		if errors.Is(err, domain.ErrNotFound) {
			break
		}
		if err != nil {
			return entity.Source{}, "", err
		}
		token = ""
	}
	if token == "" {
		return entity.Source{}, "", fmt.Errorf("generate unique token")
	}

	if err := s.repo.SetToken(ctx, src.ID, token); err != nil {
		return entity.Source{}, "", err
	}
	src.Token = token
	return src, token, nil
}

func (s *SourceService) SetActive(ctx context.Context, sourceID int, active bool) (entity.Source, error) {
	if sourceID <= 0 {
		return entity.Source{}, fmt.Errorf("%w: source id is required", domain.ErrInvalidInput)
	}
	src, err := s.repo.GetByID(ctx, sourceID)
	if err != nil {
		return entity.Source{}, err
	}
	if src.ParentID != nil {
		return entity.Source{}, fmt.Errorf("%w: is_active is only for root sources", domain.ErrInvalidInput)
	}
	return s.repo.SetActive(ctx, src.ID, active)
}

func (s *SourceService) Delete(ctx context.Context, sourceID int) error {
	if sourceID <= 0 {
		return fmt.Errorf("%w: source id is required", domain.ErrInvalidInput)
	}
	if _, err := s.repo.GetByID(ctx, sourceID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, sourceID)
}

func (s *SourceService) DeleteToken(ctx context.Context, sourceID int) error {
	if sourceID <= 0 {
		return fmt.Errorf("%w: source id is required", domain.ErrInvalidInput)
	}

	src, err := s.repo.GetByID(ctx, sourceID)
	if err != nil {
		return err
	}
	if src.ParentID != nil {
		return fmt.Errorf("%w: token is only for root sources", domain.ErrInvalidInput)
	}
	return s.repo.SetToken(ctx, src.ID, "")
}

func (s *SourceService) ListByToken(ctx context.Context, token string) ([]entity.Source, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("%w: token is required", domain.ErrInvalidInput)
	}

	root, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	family, err := s.repo.ListFamily(ctx, root.ID)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return []entity.Source{root}, nil
	}
	return family, nil
}
