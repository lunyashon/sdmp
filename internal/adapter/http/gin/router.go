package gin

import (
	gongin "github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/handler"
)

type Handlers struct {
	Leads   *handler.LeadsHandler
	Sources *handler.SourcesHandler
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
		rest.GET("/sources", h.Sources.List)
	}

	return router
}
