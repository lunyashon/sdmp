package port

import (
	"context"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type SplitRepository interface {
	List(ctx context.Context) ([]entity.SourceSplit, error)
	Get(ctx context.Context, sourceID int) (entity.SourceSplit, error)
	Replace(ctx context.Context, in entity.SourceSplit) (entity.SourceSplit, error)
	Resolve(ctx context.Context, sourceID, operatorID int) (entity.SplitResolve, bool, error)
}
