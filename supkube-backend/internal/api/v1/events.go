package v1

// events.go — SSE event stream (GET /api/v1/events). Agents (SupInsight, the
// MCP server) subscribe to react to Supkube state changes without polling.
// In-process bus (internal/events); terminal events fed by eventwatch.
//
// NOTE (F1 lesson): this route MUST be registered in auth.permissionTable, else
// the RBAC middleware fail-closes it to 403 in production. See server.go + the
// TestEventsRouteInRBAC guard.

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/supkube/supkube-backend/internal/events"
)

func EventsHandler(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")

	var filter map[string]bool
	if q := c.Query("type"); q != "" {
		filter = make(map[string]bool)
		for _, t := range strings.Split(q, ",") {
			if t = strings.TrimSpace(t); t != "" {
				filter[t] = true
			}
		}
	}

	sub := events.Default.Subscribe(64)
	defer sub.Close()

	_, _ = c.Writer.Write([]byte(": connected\n\n"))
	c.Writer.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := c.Writer.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			c.Writer.Flush()
		case e, ok := <-sub.C:
			if !ok {
				return
			}
			if filter != nil && !filter[e.Type] {
				continue
			}
			blob, _ := json.Marshal(e)
			if _, err := c.Writer.Write([]byte("event: " + e.Type + "\ndata: " + string(blob) + "\n\n")); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}
