package port

import (
	"context"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type SkorozvonMapRepository interface {
	ReplaceAll(ctx context.Context, rows []entity.SkorozvonMap) error
	Get(ctx context.Context, fieldID, optionID int64) (entity.SkorozvonMap, bool, error)
	ListFields(ctx context.Context) ([]entity.SkorozvonMap, error)
	SetBitrixCodes(ctx context.Context, rows []entity.SkorozvonMap) error
}
