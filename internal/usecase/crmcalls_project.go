package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

const defaultCrmcallsBaseURL = "https://cc.crmcalls.ru"
const crmcallsTestPhone = "79800010203"

type CrmcallsDial func(baseURL, apiKey string) port.Crmcalls

type CrmcallsProjectService struct {
	repo     port.CrmcallsProjectRepository
	api      port.Crmcalls
	settings port.CrmcallsSettingsRepository
	envKey   string
	envURL   string
	dial     CrmcallsDial
}

func NewCrmcallsProjectService(repo port.CrmcallsProjectRepository, api port.Crmcalls) *CrmcallsProjectService {
	return &CrmcallsProjectService{repo: repo, api: api}
}

func (s *CrmcallsProjectService) WithSettings(settings port.CrmcallsSettingsRepository, envKey, envURL string, dial CrmcallsDial) *CrmcallsProjectService {
	s.settings = settings
	s.envKey = envKey
	s.envURL = envURL
	s.dial = dial
	return s
}

func (s *CrmcallsProjectService) List(ctx context.Context) ([]entity.CrmcallsProject, error) {
	return s.repo.List(ctx)
}

func (s *CrmcallsProjectService) Connection(ctx context.Context) (entity.CrmcallsConnection, error) {
	key, url := s.storedCreds(ctx)
	if key == "" {
		key = strings.TrimSpace(s.envKey)
	}
	items, err := s.repo.List(ctx)
	if err != nil {
		return entity.CrmcallsConnection{}, err
	}
	return entity.CrmcallsConnection{
		HasKey:       strings.TrimSpace(key) != "",
		BaseURL:      url,
		APIKeyMasked: maskAPIKey(key),
		ProjectCount: len(items),
	}, nil
}

func (s *CrmcallsProjectService) Connect(ctx context.Context, apiKey, baseURL string) ([]entity.CrmcallsProject, error) {
	storedKey, storedURL := s.storedCreds(ctx)
	key := strings.TrimSpace(apiKey)
	if key == "" {
		key = storedKey
	}
	if key == "" {
		key = strings.TrimSpace(s.envKey)
	}
	if key == "" {
		return nil, fmt.Errorf("%w: укажите API-ключ CRMCalls", domain.ErrInvalidInput)
	}
	url := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if url == "" {
		url = storedURL
	}
	client := s.client(url, key)
	if client == nil {
		return nil, fmt.Errorf("%w: crmcalls api is not configured", domain.ErrInvalidInput)
	}
	remote, err := client.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidInput, err.Error())
	}
	if s.settings != nil {
		if err := s.settings.Save(ctx, entity.CrmcallsSettings{APIKey: key, BaseURL: url}); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Upsert(ctx, remote); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *CrmcallsProjectService) Sync(ctx context.Context) ([]entity.CrmcallsProject, error) {
	client, err := s.live(ctx)
	if err != nil {
		return nil, err
	}
	remote, err := client.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidInput, err.Error())
	}
	if err := s.repo.Upsert(ctx, remote); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *CrmcallsProjectService) Refresh(ctx context.Context) ([]entity.CrmcallsProject, error) {
	local, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(local) == 0 {
		return local, nil
	}
	client, err := s.live(ctx)
	if err != nil {
		return nil, err
	}
	remote, err := client.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidInput, err.Error())
	}
	byID := make(map[string]entity.CrmcallsProject, len(remote))
	for _, item := range remote {
		byID[strings.TrimSpace(item.ID)] = item
	}
	now := time.Now().UTC()
	out := make([]entity.CrmcallsProject, 0, len(local))
	for _, item := range local {
		found, ok := byID[item.ID]
		status := ""
		name := ""
		active := false
		if ok {
			status = found.Status
			name = found.Name
			active = crmcallsStatusActive(found.Status)
		}
		updated, err := s.repo.UpdateCheck(ctx, item.ID, now, active, status, name)
		if err != nil {
			return nil, err
		}
		out = append(out, updated)
	}
	return out, nil
}

func (s *CrmcallsProjectService) SendTest(ctx context.Context, projectID string) (entity.CrmcallsTestResult, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return entity.CrmcallsTestResult{}, fmt.Errorf("%w: project id is required", domain.ErrInvalidInput)
	}
	if _, err := s.repo.Get(ctx, projectID); err != nil {
		return entity.CrmcallsTestResult{}, err
	}
	client, err := s.live(ctx)
	if err != nil {
		return entity.CrmcallsTestResult{}, err
	}
	res, err := client.SendLead(ctx, port.CrmcallsLead{
		Phone:      crmcallsTestPhone,
		Name:       "Тест SDMP",
		ProjectID:  projectID,
		ExternalID: "sdmp-test-" + projectID,
	})
	if err != nil {
		return entity.CrmcallsTestResult{HTTPStatus: res.Status}, fmt.Errorf("%w: %s", domain.ErrInvalidInput, err.Error())
	}
	return entity.CrmcallsTestResult{ContactID: res.ContactID, HTTPStatus: res.Status}, nil
}

func crmcallsStatusActive(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "running", "enabled", "online":
		return true
	default:
		return false
	}
}

func (s *CrmcallsProjectService) live(ctx context.Context) (port.Crmcalls, error) {
	client := s.api
	if s.dial != nil {
		key, url := s.storedCreds(ctx)
		if key == "" {
			key = strings.TrimSpace(s.envKey)
		}
		if key != "" {
			client = s.dial(url, key)
		}
	}
	if client == nil {
		return nil, fmt.Errorf("%w: crmcalls api is not configured", domain.ErrInvalidInput)
	}
	return client, nil
}

func (s *CrmcallsProjectService) storedCreds(ctx context.Context) (string, string) {
	url := strings.TrimRight(strings.TrimSpace(s.envURL), "/")
	if url == "" {
		url = defaultCrmcallsBaseURL
	}
	if s.settings == nil {
		return "", url
	}
	st, err := s.settings.Get(ctx)
	if err != nil {
		return "", url
	}
	if base := strings.TrimRight(strings.TrimSpace(st.BaseURL), "/"); base != "" {
		url = base
	}
	return strings.TrimSpace(st.APIKey), url
}

func (s *CrmcallsProjectService) client(baseURL, apiKey string) port.Crmcalls {
	if s.dial != nil {
		return s.dial(baseURL, apiKey)
	}
	return s.api
}

func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) < 16 {
		return "••••"
	}
	return key[:10] + "••••" + key[len(key)-4:]
}
