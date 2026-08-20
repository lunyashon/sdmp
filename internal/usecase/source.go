package usecase

import (
	"context"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type SourceService struct {
	repo port.SourceRepository
}

func NewSourceService(repo port.SourceRepository) *SourceService {
	return &SourceService{repo: repo}
}

func (s *SourceService) List(ctx context.Context) ([]entity.Source, error) {
	return s.repo.List(ctx)
}
