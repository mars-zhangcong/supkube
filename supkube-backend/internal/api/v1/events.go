package v1

// events.go — Supkube SSE event stream (the "subscribe" half of the
// agent-friendly surface; MCP is the "call" half). GET /api/v1/events streams
// server-sent events so external agents react to Supkube state changes (backup
// completed / restore failed / run progress / posture changed) without polling.
//
// In-process bus (internal/events), no broker — see bus.go for the rationale.

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/supkube/supkube-backend/internal/events"
)

// EventsHandler streams events.Default as text/event-stream.
// Optional filter: ?type=backup.created,restore.failed (comma-separated).
func EventsHandler(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // disable nginx buffering so events flush

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

	// Open the stream (also lets the client know it's connected).
	_, _ = c.Writer.Write([]byte(": connected\n\n"))
	c.Writer.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done(): // client disconnected
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
