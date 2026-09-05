package usecase

import (
	"context"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type SplitService struct {
	splits  port.SplitRepository
	sources port.SourceRepository
	dicts   port.DictRepository
}

func NewSplitService(splits port.SplitRepository, sources port.SourceRepository, dicts port.DictRepository) *SplitService {
	return &SplitService{splits: splits, sources: sources, dicts: dicts}
}

func (s *SplitService) List(ctx context.Context) ([]entity.SourceSplit, error) {
	return s.splits.List(ctx)
}

func (s *SplitService) Get(ctx context.Context, sourceID int) (entity.SourceSplit, error) {
	return s.splits.Get(ctx, sourceID)
}

func (s *SplitService) Replace(ctx context.Context, in entity.SourceSplit) (entity.SourceSplit, error) {
	return s.splits.Replace(ctx, in)
}
