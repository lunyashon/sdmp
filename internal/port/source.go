package port

import (
	"context"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type SourceRepository interface {
	List(ctx context.Context) ([]entity.Source, error)
	GetByID(ctx context.Context, id int) (entity.Source, error)
	GetByBitrixCode(ctx context.Context, code string) (entity.Source, error)
	GetByToken(ctx context.Context, token string) (entity.Source, error)
	ListFamily(ctx context.Context, rootID int) ([]entity.Source, error)
	SetToken(ctx context.Context, id int, token string) error
	SetActive(ctx context.Context, id int, active bool) (entity.Source, error)
	Create(ctx context.Context, src entity.Source) (entity.Source, error)
	Delete(ctx context.Context, id int) error
}
