package port

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type GoogleSheetRepository interface {
	List(ctx context.Context) ([]entity.GoogleSheet, error)
	Get(ctx context.Context, id int) (entity.GoogleSheet, error)
	Credentials(ctx context.Context, id int) (json.RawMessage, error)
	UpdateCheck(ctx context.Context, id int, at time.Time, ok bool, status int, response string) error
	Create(ctx context.Context, in entity.GoogleSheet, credentials json.RawMessage) (entity.GoogleSheet, error)
	Update(ctx context.Context, in entity.GoogleSheet, credentials json.RawMessage) (entity.GoogleSheet, error)
	Delete(ctx context.Context, id int) error
	ListRoutable(ctx context.Context, zarya string, regionID *int, sourceID int) ([]entity.GoogleSheetTarget, error)
}

type GoogleSheets interface {
	AppendRow(ctx context.Context, credentials json.RawMessage, spreadsheetID, sheetName string, values []string) (status int, body []byte, err error)
	HeaderRow(ctx context.Context, credentials json.RawMessage, spreadsheetID, sheetName string) (resolvedSheet string, columns []string, sheets []string, err error)
}
