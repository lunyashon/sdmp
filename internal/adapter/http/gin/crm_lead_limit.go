package gin

import (
	"net/http"
	"strconv"
	"strings"

	gongin "github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/lib/config"
	"github.com/lunyashon/sdmp/internal/lib/ratelimit"
)

func crmLeadAddLimit(cfg *config.Config) gongin.HandlerFunc {
	if cfg == nil || cfg.CRMLeadAddRPS <= 0 {
		return nil
	}
	burst := cfg.CRMLeadAddBurst
	if burst <= 0 {
		burst = 32
	}
	return crmLeadAddRateLimit(ratelimit.New(cfg.CRMLeadAddRPS, burst))
}

func crmLeadAddRateLimit(lim *ratelimit.Limiter) gongin.HandlerFunc {
	if lim == nil {
		return nil
	}
	return func(c *gongin.Context) {
		key := strings.TrimSpace(c.Param("token"))
		if key == "" {
			key = "_"
		}
		ok, retryAfter := lim.Allow(key)
		if ok {
			c.Next()
			return
		}
		sec := int(retryAfter.Seconds())
		if sec < 1 {
			sec = 1
		}
		c.Header("Retry-After", strconv.Itoa(sec))
		response.BitrixError(c, http.StatusTooManyRequests, "QUERY_LIMIT_EXCEEDED", "Too many requests")
		c.Abort()
	}
}
