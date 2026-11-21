package gin

import (
	"net/http"
	"strings"

	gongin "github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/lunyashon/sdmp/docs"
	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/lib/config"
)

type Handlers struct {
	Leads             *handler.LeadsHandler
	Sources           *handler.SourcesHandler
	Dicts             *handler.DictsHandler
	Partners          *handler.PartnersHandler
	Splits            *handler.SplitsHandler
	SkorozvonProjects *handler.SkorozvonProjectsHandler
	SkorozvonLinks    *handler.SkorozvonLinksHandler
	SkorozvonEvents   *handler.SkorozvonEventsHandler
	CRMLead           *handler.CRMLeadHandler
	Docs              *handler.DocsHandler
	BitrixMaps        *handler.BitrixMapsHandler
	GoogleSheets      *handler.GoogleSheetsHandler
	Crmcalls          *handler.CrmcallsHandler
}

func NewRouter(h Handlers, config *config.Config) *gongin.Engine {
	router := gongin.New()
	router.Use(gongin.Logger())
	router.Use(gongin.Recovery())

	router.GET("/api/v1/health", response.Health)
	mountSwagger(router, config)

	rest := router.Group("/rest")
	if config != nil && strings.TrimSpace(config.AuthPassword) != "" {
		rest.POST("/auth/login", handler.NewAuthHandler(config).Login)
		rest.Use(requireAuth(config))
	}
	{
		rest.GET("/leads", h.Leads.List)
		rest.GET("/leads/export.xlsx", h.Leads.ExportXLSX)
		rest.POST("/leads/import/preview", h.Leads.PreviewImport)
		rest.POST("/leads/import", h.Leads.Import)
		rest.POST("/leads/skorozvon/resend", h.Leads.ResendSkorozvon)
		rest.GET("/leads/incoming-stats", h.Leads.IncomingStats)
		rest.GET("/leads/enrichment-stats", h.Leads.EnrichmentStats)
		rest.GET("/leads/zarya-stats", h.Leads.ZaryaStats)
		rest.GET("/leads/skorozvon-bitrix-stats", h.Leads.SkorozvonBitrixStats)
		rest.GET("/leads/outgoing-stats", h.Leads.OutgoingStats)
		rest.GET("/leads/sk-dispatch", h.Leads.ListSkDispatch)
		rest.GET("/leads/:id", h.Leads.Get)
		rest.GET("/leads/:id/raw", h.Leads.Raw)
		rest.GET("/leads/:id/recording", h.Leads.Recording)
		rest.GET("/recordings/:ref", h.Leads.PublicRecording)
		rest.POST("/leads/:id/partners/:partnerId/resend", h.Leads.ResendPartner)
		if h.Docs != nil {
			rest.GET("/docs/crm.lead.add.docx", h.Docs.CRMLeadAddDocx)
		}
		rest.GET("/sources", h.Sources.List)
		rest.POST("/sources", h.Sources.Create)
		if h.Dicts != nil {
			rest.GET("/regions", h.Dicts.Regions)
			rest.GET("/statuses", h.Dicts.Statuses)
			rest.GET("/operators", h.Dicts.Operators)
			rest.POST("/operators/:id/aliases", h.Dicts.AddOperatorAlias)
			rest.DELETE("/operators/:id/aliases", h.Dicts.RemoveOperatorAlias)
		}
		if h.Partners != nil {
			rest.GET("/partners", h.Partners.List)
			rest.POST("/partners", h.Partners.Create)
			rest.POST("/partners/refresh", h.Partners.Refresh)
			rest.POST("/partners/:id/test", h.Partners.SendTest)
			rest.PUT("/partners/:id", h.Partners.Update)
		}
		if h.Splits != nil {
			rest.GET("/splits", h.Splits.List)
			rest.GET("/sources/:id/split", h.Splits.Get)
			rest.PUT("/sources/:id/split", h.Splits.Replace)
		}
		if h.SkorozvonProjects != nil {
			rest.GET("/skorozvon/projects/list", h.SkorozvonProjects.List)
			rest.POST("/skorozvon/projects/sync", h.SkorozvonProjects.Sync)
			rest.POST("/skorozvon/projects/refresh", h.SkorozvonProjects.Refresh)
			rest.POST("/skorozvon/projects/:uuid/test", h.SkorozvonProjects.SendTest)
		}
		if h.SkorozvonLinks != nil {
			rest.GET("/skorozvon/links", h.SkorozvonLinks.List)
			rest.POST("/skorozvon/links", h.SkorozvonLinks.Create)
			rest.PUT("/skorozvon/links/:id", h.SkorozvonLinks.Update)
			rest.DELETE("/skorozvon/links/:id", h.SkorozvonLinks.Delete)
		}
		if h.SkorozvonEvents != nil {
			rest.GET("/skorozvon/events", h.SkorozvonEvents.Receive)
			rest.POST("/skorozvon/events", h.SkorozvonEvents.Receive)
			rest.PUT("/skorozvon/events", h.SkorozvonEvents.Receive)
			rest.PATCH("/skorozvon/events", h.SkorozvonEvents.Receive)
		}
		rest.POST("/sources/:id/children", h.Sources.CreateChild)
		rest.DELETE("/sources/:id", h.Sources.Delete)
		rest.PATCH("/sources/:id", h.Sources.SetActive)
		rest.POST("/sources/:id/token", h.Sources.CreateToken)
		rest.DELETE("/sources/:id/token", h.Sources.DeleteToken)
		rest.GET("/sources/by-token/:token", h.Sources.ListByToken)
		if h.BitrixMaps != nil {
			rest.GET("/bitrix/lead-map", h.BitrixMaps.Get)
			rest.PUT("/bitrix/lead-map", h.BitrixMaps.Save)
			rest.POST("/bitrix/lead-map/refresh", h.BitrixMaps.Refresh)
		}
		if h.GoogleSheets != nil {
			rest.GET("/google/accounts", h.GoogleSheets.Accounts)
			rest.GET("/google/sheets", h.GoogleSheets.List)
			rest.POST("/google/sheets/refresh", h.GoogleSheets.Refresh)
			rest.POST("/google/sheets/preview", h.GoogleSheets.Preview)
			rest.POST("/google/sheets/:id/test", h.GoogleSheets.SendTest)
			rest.POST("/google/sheets", h.GoogleSheets.Create)
			rest.PUT("/google/sheets/:id", h.GoogleSheets.Update)
			rest.DELETE("/google/sheets/:id", h.GoogleSheets.Delete)
		}
		if h.Crmcalls != nil {
			rest.GET("/crmcalls/links", h.Crmcalls.ListLinks)
			rest.POST("/crmcalls/links", h.Crmcalls.CreateLink)
			rest.PUT("/crmcalls/links/:id", h.Crmcalls.UpdateLink)
			rest.DELETE("/crmcalls/links/:id", h.Crmcalls.DeleteLink)
			rest.GET("/crmcalls/projects", h.Crmcalls.ListProjects)
			rest.POST("/crmcalls/projects/sync", h.Crmcalls.SyncProjects)
			rest.POST("/crmcalls/projects/refresh", h.Crmcalls.RefreshProjects)
			rest.POST("/crmcalls/projects/:id/test", h.Crmcalls.SendTest)
			rest.GET("/crmcalls/settings", h.Crmcalls.GetSettings)
			rest.PUT("/crmcalls/settings", h.Crmcalls.SaveSettings)
			rest.GET("/crmcalls/events", h.Crmcalls.Receive)
			rest.POST("/crmcalls/events", h.Crmcalls.Receive)
			rest.PUT("/crmcalls/events", h.Crmcalls.Receive)
			rest.PATCH("/crmcalls/events", h.Crmcalls.Receive)
		}
		if h.CRMLead != nil {
			add := []gongin.HandlerFunc{h.CRMLead.Add}
			if limit := crmLeadAddLimit(config); limit != nil {
				add = []gongin.HandlerFunc{limit, h.CRMLead.Add}
			}
			rest.POST("/:user_id/:token/crm.lead.add.json", add...)
			rest.GET("/:user_id/:token/crm.lead.add.json", add...)
			rest.POST("/:user_id/:token/crm.lead.add", add...)
			rest.GET("/:user_id/:token/crm.lead.add", add...)
			rest.GET("/:user_id/:token/scope.json", h.CRMLead.Scope)
			rest.POST("/:user_id/:token/scope.json", h.CRMLead.Scope)
			rest.GET("/:user_id/:token/scope", h.CRMLead.Scope)
			rest.POST("/:user_id/:token/scope", h.CRMLead.Scope)
			rest.GET("/:user_id/:token/:method", h.CRMLead.Mimic)
			rest.POST("/:user_id/:token/:method", h.CRMLead.Mimic)
		}
	}

	return router
}

func mountSwagger(router *gongin.Engine, cfg *config.Config) {
	if cfg == nil {
		return
	}
	version := strings.Trim(strings.TrimSpace(cfg.SwaggerVersion), `"'`)
	token := strings.Trim(strings.TrimSpace(cfg.SwaggerToken), `"'`)
	if version == "" || token == "" {
		return
	}
	mountSwaggerUI(router, "/swagger/crm/"+version+"/"+token, "crm")
	mountSwaggerUI(router, "/swagger/"+version+"/"+token, "swagger")
}

func mountSwaggerUI(router *gongin.Engine, prefix, instance string) {
	opts := []func(*ginSwagger.Config){ginSwagger.URL(prefix + "/doc.json")}
	if instance != "" && instance != "swagger" {
		opts = append(opts, ginSwagger.InstanceName(instance))
	}
	ui := ginSwagger.WrapHandler(swaggerFiles.Handler, opts...)
	redirect := func(c *gongin.Context) {
		c.Redirect(http.StatusFound, prefix+"/index.html")
	}
	router.GET(prefix, redirect)
	router.GET(prefix+"/*any", func(c *gongin.Context) {
		if strings.Trim(c.Param("any"), "/") == "" {
			redirect(c)
			return
		}
		ui(c)
	})
}
