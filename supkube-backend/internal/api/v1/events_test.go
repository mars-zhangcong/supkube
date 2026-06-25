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

// End-to-end: subscribe via the SSE handler, publish on the bus, assert the
// event is streamed onto the wire. No cluster needed.
func TestEventsHandlerStreams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/events", nil).WithContext(ctx)

	before := events.Default.SubscriberCount()
	done := make(chan struct{})
	go func() { EventsHandler(c); close(done) }()

	// wait until the handler has subscribed
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

	events.Publish(events.Event{Type: events.TypeBackupCreated, Subject: "b-test"})
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := w.Body.String()
	if !strings.Contains(body, "event: backup.created") || !strings.Contains(body, "b-test") {
		t.Fatalf("stream missing event; body=%q", body)
	}
}

func TestEventsHandlerTypeFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/events?type=restore.failed", nil).WithContext(ctx)

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

	events.Publish(events.Event{Type: events.TypeBackupCreated, Subject: "should-be-filtered"})
	events.Publish(events.Event{Type: events.TypeRestoreFailed, Subject: "should-pass"})
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := w.Body.String()
	if strings.Contains(body, "should-be-filtered") {
		t.Fatalf("filter leaked non-matching event; body=%q", body)
	}
	if !strings.Contains(body, "should-pass") {
		t.Fatalf("filter dropped matching event; body=%q", body)
	}
}
