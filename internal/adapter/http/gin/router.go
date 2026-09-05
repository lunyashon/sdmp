package gin

import (
	gongin "github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
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

func NewRouter(h Handlers) *gongin.Engine {
	router := gongin.New()
	router.Use(gongin.Logger())
	router.Use(gongin.Recovery())

	router.GET("/api/v1/health", func(c *gongin.Context) {
		c.JSON(200, gongin.H{"status": "ok"})
	})

	rest := router.Group("/rest")
	{
		rest.GET("/leads", h.Leads.List)
		rest.GET("/leads/incoming-stats", h.Leads.IncomingStats)
		rest.GET("/leads/enrichment-stats", h.Leads.EnrichmentStats)
		rest.GET("/leads/zarya-stats", h.Leads.ZaryaStats)
		rest.GET("/leads/:id", h.Leads.Get)
		rest.GET("/sources", h.Sources.List)
		if h.Dicts != nil {
			rest.GET("/regions", h.Dicts.Regions)
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
