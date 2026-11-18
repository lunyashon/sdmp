package crmcalls

import (
	"context"
	"strings"

	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.Crmcalls = (*Dynamic)(nil)

type SettingsLookup interface {
	Get(ctx context.Context) (entity.CrmcallsSettings, error)
}

// Dynamic берёт ключ из БД (вкладка «Исходящий трафик»), иначе из env.
type Dynamic struct {
	env Config
	src SettingsLookup
}

func NewDynamic(env Config, src SettingsLookup) *Dynamic {
	return &Dynamic{env: env, src: src}
}

func (d *Dynamic) SendLead(ctx context.Context, lead port.CrmcallsLead) (port.CrmcallsSendResult, error) {
	return New(d.config(ctx)).SendLead(ctx, lead)
}

func (d *Dynamic) ListProjects(ctx context.Context) ([]entity.CrmcallsProject, error) {
	return New(d.config(ctx)).ListProjects(ctx)
}

func (d *Dynamic) DownloadRecording(ctx context.Context, callIDOrURL string) ([]byte, string, error) {
	return New(d.config(ctx)).DownloadRecording(ctx, callIDOrURL)
}

func (d *Dynamic) config(ctx context.Context) Config {
	cfg := d.env
	if d == nil || d.src == nil {
		return cfg
	}
	st, err := d.src.Get(ctx)
	if err != nil {
		return cfg
	}
	if key := strings.TrimSpace(st.APIKey); key != "" {
		cfg.APIKey = key
	}
	if base := strings.TrimSpace(st.BaseURL); base != "" {
		cfg.BaseURL = base
	}
	return cfg
}
