package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/domain/entity"
	"github.com/lunyashon/sdmp/internal/port"
)

type stubCrmcallsProjectRepo struct {
	items []entity.CrmcallsProject
}

func (s *stubCrmcallsProjectRepo) List(context.Context) ([]entity.CrmcallsProject, error) {
	return append([]entity.CrmcallsProject(nil), s.items...), nil
}

func (s *stubCrmcallsProjectRepo) Get(_ context.Context, id string) (entity.CrmcallsProject, error) {
	for _, item := range s.items {
		if item.ID == id {
			return item, nil
		}
	}
	return entity.CrmcallsProject{}, domain.ErrNotFound
}

func (s *stubCrmcallsProjectRepo) Upsert(_ context.Context, items []entity.CrmcallsProject) error {
	s.items = append([]entity.CrmcallsProject(nil), items...)
	return nil
}

func (s *stubCrmcallsProjectRepo) Ensure(context.Context, string, string) error { return nil }

func (s *stubCrmcallsProjectRepo) UpdateCheck(_ context.Context, id string, at time.Time, ok bool, status, name string) (entity.CrmcallsProject, error) {
	for i, item := range s.items {
		if item.ID != id {
			continue
		}
		item.LastDateRequest = &at
		item.LastRequest = &ok
		if status != "" {
			item.Status = status
		}
		if name != "" {
			item.Name = name
		}
		s.items[i] = item
		return item, nil
	}
	return entity.CrmcallsProject{}, domain.ErrNotFound
}

type stubCrmcallsSettingsRepo struct {
	got entity.CrmcallsSettings
}

func (s *stubCrmcallsSettingsRepo) Get(context.Context) (entity.CrmcallsSettings, error) {
	return s.got, nil
}

func (s *stubCrmcallsSettingsRepo) Save(_ context.Context, in entity.CrmcallsSettings) error {
	s.got = in
	return nil
}

type stubCrmcallsAPI struct {
	key   string
	items []entity.CrmcallsProject
	err   error
}

func (s *stubCrmcallsAPI) SendLead(_ context.Context, lead port.CrmcallsLead) (port.CrmcallsSendResult, error) {
	s.key = lead.ProjectID
	if s.err != nil {
		return port.CrmcallsSendResult{Status: 422}, s.err
	}
	return port.CrmcallsSendResult{ContactID: "ctc-test", Status: 200}, nil
}

func (s *stubCrmcallsAPI) ListProjects(context.Context) ([]entity.CrmcallsProject, error) {
	return s.items, s.err
}

func (s *stubCrmcallsAPI) DownloadRecording(context.Context, string) ([]byte, string, error) {
	return nil, "", s.err
}

func TestConnectCrmcallsSavesKeyAndProjects(t *testing.T) {
	repo := &stubCrmcallsProjectRepo{}
	settings := &stubCrmcallsSettingsRepo{}
	var gotKey, gotURL string
	svc := NewCrmcallsProjectService(repo, nil).WithSettings(settings, "", "", func(baseURL, apiKey string) port.Crmcalls {
		gotURL, gotKey = baseURL, apiKey
		return &stubCrmcallsAPI{items: []entity.CrmcallsProject{{ID: "p1", Name: "Обзвон", Status: "running"}}}
	})
	items, err := svc.Connect(context.Background(), "crmc_live_testkey1234", "https://cc.crmcalls.ru")
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "crmc_live_testkey1234" || gotURL != "https://cc.crmcalls.ru" {
		t.Fatalf("dial %s %s", gotURL, gotKey)
	}
	if settings.got.APIKey != "crmc_live_testkey1234" {
		t.Fatalf("saved %+v", settings.got)
	}
	if len(items) != 1 || items[0].ID != "p1" {
		t.Fatalf("%+v", items)
	}
}

func TestConnectCrmcallsRequiresKey(t *testing.T) {
	_, err := NewCrmcallsProjectService(&stubCrmcallsProjectRepo{}, nil).Connect(context.Background(), "", "")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("%v", err)
	}
}

func TestConnectCrmcallsKeepsStoredKey(t *testing.T) {
	settings := &stubCrmcallsSettingsRepo{got: entity.CrmcallsSettings{APIKey: "stored-key", BaseURL: "https://cc.crmcalls.ru"}}
	var gotKey string
	svc := NewCrmcallsProjectService(&stubCrmcallsProjectRepo{}, nil).WithSettings(settings, "", "", func(_, apiKey string) port.Crmcalls {
		gotKey = apiKey
		return &stubCrmcallsAPI{items: []entity.CrmcallsProject{{ID: "p2", Name: "B"}}}
	})
	if _, err := svc.Connect(context.Background(), "  ", ""); err != nil {
		t.Fatal(err)
	}
	if gotKey != "stored-key" {
		t.Fatalf("key %s", gotKey)
	}
}

func TestConnectionMasksKey(t *testing.T) {
	repo := &stubCrmcallsProjectRepo{items: []entity.CrmcallsProject{{ID: "p1"}}}
	settings := &stubCrmcallsSettingsRepo{got: entity.CrmcallsSettings{
		APIKey:  "crmc_live_abcdefghijklmnop",
		BaseURL: "https://cc.crmcalls.ru",
	}}
	got, err := NewCrmcallsProjectService(repo, nil).WithSettings(settings, "", "", nil).Connection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasKey || got.ProjectCount != 1 || got.APIKeyMasked != "crmc_live_••••mnop" {
		t.Fatalf("%+v", got)
	}
}

func TestSyncCrmcallsWrapsAPIError(t *testing.T) {
	svc := NewCrmcallsProjectService(&stubCrmcallsProjectRepo{}, &stubCrmcallsAPI{err: errors.New("crmcalls HTTP 401")})
	_, err := svc.Sync(context.Background())
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("%v", err)
	}
}

func TestRefreshCrmcallsMarksInactive(t *testing.T) {
	ok := true
	repo := &stubCrmcallsProjectRepo{items: []entity.CrmcallsProject{
		{ID: "active-1", Name: "A", Status: "draft"},
		{ID: "gone-1", Name: "B", LastRequest: &ok},
	}}
	api := &stubCrmcallsAPI{items: []entity.CrmcallsProject{{ID: "active-1", Name: "A", Status: "active"}}}
	got, err := NewCrmcallsProjectService(repo, api).Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].LastRequest == nil || !*got[0].LastRequest || got[0].Status != "active" {
		t.Fatalf("active %+v", got[0])
	}
	if got[1].LastRequest == nil || *got[1].LastRequest {
		t.Fatalf("gone %+v", got[1])
	}
}

func TestSendCrmcallsTest(t *testing.T) {
	repo := &stubCrmcallsProjectRepo{items: []entity.CrmcallsProject{{ID: "p1", Name: "тест"}}}
	api := &stubCrmcallsAPI{}
	got, err := NewCrmcallsProjectService(repo, api).SendTest(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ContactID != "ctc-test" || api.key != "p1" {
		t.Fatalf("%+v key=%s", got, api.key)
	}
}
