package port

import (
	"context"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type BitrixMapRepository interface {
	ReplaceFields(ctx context.Context, rows []entity.BitrixLeadField) error
	ReplaceStatuses(ctx context.Context, entityID string, rows []entity.BitrixStatus) error
	ReplaceMaps(ctx context.Context, rows []entity.BitrixLeadMap) error
	ListFields(ctx context.Context) ([]entity.BitrixLeadField, error)
	ListStatuses(ctx context.Context, entityID string) ([]entity.BitrixStatus, error)
	ListMaps(ctx context.Context) ([]entity.BitrixLeadMap, error)
}
