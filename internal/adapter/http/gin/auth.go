package gin

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	gongin "github.com/gin-gonic/gin"

	"github.com/lunyashon/sdmp/internal/adapter/http/response"
	"github.com/lunyashon/sdmp/internal/lib/auth"
	"github.com/lunyashon/sdmp/internal/lib/config"
)

var inboundLead = regexp.MustCompile(`^/rest/[^/]+/[^/]+/`)

func bearer(c *gongin.Context) string {
	h := strings.TrimSpace(c.GetHeader("Authorization"))
	const p = "Bearer "
	if len(h) < len(p) || !strings.EqualFold(h[:len(p)], p) {
		return ""
	}
	return strings.TrimSpace(h[len(p):])
}

// publicREST — входящие вебхуки и приём лидов. У них свой секрет, не учётка UI.
func publicREST(path string) bool {
	switch {
	case path == "/rest/auth/login":
		return true
	case path == "/rest/skorozvon/events":
		return true
	case path == "/rest/crmcalls/events":
		return true
	case strings.HasPrefix(path, "/rest/recordings/"):
		return true
	case inboundLead.MatchString(path):
		return true
	default:
		return false
	}
}

func requireAuth(cfg *config.Config) gongin.HandlerFunc {
	return func(c *gongin.Context) {
		if publicREST(c.Request.URL.Path) {
			c.Next()
			return
		}
		secret := ""
		if cfg != nil {
			secret = cfg.AuthPassword
		}
		if !auth.Valid(secret, bearer(c), time.Now()) {
			response.Error(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}
