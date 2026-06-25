package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/supkube/supkube-backend/internal/events"
)

// End-to-end: subscribe via the SSE handler, publish a terminal event on the
// bus, assert it streams onto the wire (this is what SupInsight's adapter waits
// on). No cluster needed.
func TestEventsHandlerStreamsTerminal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/events?type=restore.completed", nil).WithContext(ctx)

	before := events.Default.SubscriberCount()
	done := make(chan struct{})
	go func() { EventsHandler(c); close(done) }()

	deadline := time.After(2 * time.Second)
	for events.Default.SubscriberCount() <= before {
		select {
		case <-deadline:
			cancel()
			t.Fatal("handler did not subscribe")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	events.Publish(events.Event{Type: events.TypeBackupCompleted, Subject: "filtered-out"})
	events.Publish(events.Event{Type: events.TypeRestoreCompleted, Subject: "r-done"})
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := w.Body.String()
	if strings.Contains(body, "filtered-out") {
		t.Fatalf("type filter leaked: %q", body)
	}
	if !strings.Contains(body, "event: restore.completed") || !strings.Contains(body, "r-done") {
		t.Fatalf("terminal event not streamed: %q", body)
	}
}
