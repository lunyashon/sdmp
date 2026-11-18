package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.CrmcallsSettingsRepository = (*CrmcallsSettingsRepo)(nil)

type CrmcallsSettingsRepo struct {
	pool *pgxpool.Pool
}

func NewCrmcallsSettingsRepo(pool *pgxpool.Pool) *CrmcallsSettingsRepo {
	return &CrmcallsSettingsRepo{pool: pool}
}

func (r *CrmcallsSettingsRepo) Get(ctx context.Context) (entity.CrmcallsSettings, error) {
	var item entity.CrmcallsSettings
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(api_key, ''), COALESCE(NULLIF(btrim(base_url), ''), 'https://cc.crmcalls.ru')
		FROM crmcalls_settings
		WHERE id = 1
	`).Scan(&item.APIKey, &item.BaseURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.CrmcallsSettings{BaseURL: "https://cc.crmcalls.ru"}, nil
	}
	if err != nil {
		return entity.CrmcallsSettings{}, fmt.Errorf("get crmcalls settings: %w", err)
	}
	return item, nil
}

func (r *CrmcallsSettingsRepo) Save(ctx context.Context, in entity.CrmcallsSettings) error {
	base := strings.TrimSpace(in.BaseURL)
	if base == "" {
		base = "https://cc.crmcalls.ru"
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crmcalls_settings (id, api_key, base_url, updated_at)
		VALUES (1, $1, $2, now())
		ON CONFLICT (id) DO UPDATE SET
			api_key = EXCLUDED.api_key,
			base_url = EXCLUDED.base_url,
			updated_at = now()
	`, strings.TrimSpace(in.APIKey), strings.TrimRight(base, "/"))
	if err != nil {
		return fmt.Errorf("save crmcalls settings: %w", err)
	}
	return nil
}
