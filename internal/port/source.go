package port

import (
	"context"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type SourceRepository interface {
	List(ctx context.Context) ([]entity.Source, error)
	GetByID(ctx context.Context, id int) (entity.Source, error)
	GetByBitrixCode(ctx context.Context, code string) (entity.Source, error)
}
