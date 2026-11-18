package port

import (
	"context"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type SkorozvonLinkRepository interface {
	List(ctx context.Context) ([]entity.SkorozvonLink, error)
	Get(ctx context.Context, id int) (entity.SkorozvonLink, error)
	Create(ctx context.Context, in entity.SkorozvonLink) (entity.SkorozvonLink, error)
	Update(ctx context.Context, in entity.SkorozvonLink) (entity.SkorozvonLink, error)
	Delete(ctx context.Context, id int) error
	ListRoutable(ctx context.Context, kind string, childSourceID int, regionID *int) ([]entity.SkorozvonLink, error)
}
