package port

import (
	"context"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type DictRepository interface {
	ListRegions(ctx context.Context) ([]entity.NamedRef, error)
	ListOperators(ctx context.Context) ([]entity.NamedRef, error)
	ListStatuses(ctx context.Context) ([]entity.NamedRef, error)
	ListOperatorGroups(ctx context.Context) ([]entity.OperatorGroup, error)
	ResolveRegion(ctx context.Context, alias string) (int, bool, error)
	ResolveOperator(ctx context.Context, alias string) (int, bool, error)
	OperatorByName(ctx context.Context, name string) (int, bool, error)
	AddOperatorAlias(ctx context.Context, operatorID int, alias string) error
	RemoveOperatorAlias(ctx context.Context, operatorID int, alias string) error
}
