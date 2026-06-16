package handlers

import (
	"murmur/middleware"

	"github.com/gin-gonic/gin"
)

// WS upgrades the connection (auth already enforced by middleware) and hands it
// to the hub.
func (h *H) WS(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		return
	}
	_ = h.Hub.Upgrade(c.Writer, c.Request, u)
}
