package port

import (
	"context"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type SkorozvonProjectRepository interface {
	List(ctx context.Context) ([]entity.SkorozvonProject, error)
	ListPage(ctx context.Context, offset int, name string) ([]entity.SkorozvonProject, int, error)
	Get(ctx context.Context, uuid string) (entity.SkorozvonProject, error)
	Create(ctx context.Context, in entity.SkorozvonProject) (entity.SkorozvonProject, error)
	CreateIfAbsent(ctx context.Context, in entity.SkorozvonProject) (entity.SkorozvonProject, bool, error)
	UpdateCheck(ctx context.Context, uuid string, at time.Time, ok bool) (entity.SkorozvonProject, error)
}
