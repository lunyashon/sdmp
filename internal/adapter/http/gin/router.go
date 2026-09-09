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
}

func NewRouter(h Handlers, config *config.Config) *gongin.Engine {
	router := gongin.New()
	router.Use(gongin.Logger())
	router.Use(gongin.Recovery())

	router.GET("/api/v1/health", response.Health)
	mountSwagger(router, config)

	rest := router.Group("/rest")
	{
		rest.GET("/leads", h.Leads.List)
		rest.GET("/leads/export.xlsx", h.Leads.ExportXLSX)
		rest.GET("/leads/incoming-stats", h.Leads.IncomingStats)
		rest.GET("/leads/enrichment-stats", h.Leads.EnrichmentStats)
		rest.GET("/leads/zarya-stats", h.Leads.ZaryaStats)
		rest.GET("/leads/skorozvon-bitrix-stats", h.Leads.SkorozvonBitrixStats)
		rest.GET("/leads/:id", h.Leads.Get)
		rest.GET("/leads/:id/raw", h.Leads.Raw)
		rest.POST("/leads/:id/partners/:partnerId/resend", h.Leads.ResendPartner)
		rest.GET("/sources", h.Sources.List)
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
			rest.GET("/skorozvon/projects", h.SkorozvonProjects.List)
			rest.POST("/skorozvon/projects", h.SkorozvonProjects.Create)
			rest.POST("/skorozvon/projects/refresh", h.SkorozvonProjects.Refresh)
			rest.POST("/skorozvon/projects/:uuid/test", h.SkorozvonProjects.SendTest)
		}
		if h.SkorozvonLinks != nil {
			rest.GET("/skorozvon/links", h.SkorozvonLinks.List)
			rest.POST("/skorozvon/links", h.SkorozvonLinks.Create)
			rest.PUT("/skorozvon/links/:id", h.SkorozvonLinks.Update)
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
		if h.CRMLead != nil {
			rest.POST("/:user_id/:token/crm.lead.add.json", h.CRMLead.Add)
			rest.GET("/:user_id/:token/crm.lead.add.json", h.CRMLead.Add)
			rest.POST("/:user_id/:token/crm.lead.add", h.CRMLead.Add)
			rest.GET("/:user_id/:token/crm.lead.add", h.CRMLead.Add)
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
