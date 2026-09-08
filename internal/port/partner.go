package port

import (
	"context"
	"net/url"
	"time"

	"github.com/lunyashon/sdmp/internal/domain/entity"
)

type HTTPFormPoster interface {
	PostForm(ctx context.Context, rawURL string, data url.Values, contentType string) ([]byte, int, error)
}

type PartnerRepository interface {
	List(ctx context.Context) ([]entity.Partner, error)
	Get(ctx context.Context, id int) (entity.Partner, error)
	Create(ctx context.Context, in entity.Partner) (entity.Partner, error)
	Update(ctx context.Context, in entity.Partner) (entity.Partner, error)
	UpdateCheck(ctx context.Context, id int, at time.Time, ok bool, status int, response string) (entity.Partner, error)
	ListRoutable(ctx context.Context, zarya string, regionID *int, sourceID int) ([]entity.Partner, error)
}
