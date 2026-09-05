package port

import (
	"context"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type SkorozvonProjectRepository interface {
	List(ctx context.Context) ([]entity.SkorozvonProject, error)
	Get(ctx context.Context, uuid string) (entity.SkorozvonProject, error)
	Create(ctx context.Context, in entity.SkorozvonProject) (entity.SkorozvonProject, error)
	UpdateCheck(ctx context.Context, uuid string, at time.Time, ok bool) (entity.SkorozvonProject, error)
}
