// Package auth: audit.go — request audit middleware + audit log query.
//
// Storage model (v0.8.5 step 4)
// ─────────────────────────────
// Audit records are written to TWO places, each serving a different audience:
//
//  1. stdout — for the customer's existing log aggregation (Loki/EFK/Splunk).
//     Standard structured prefix so SIEM rules can match. This is the
//     long-term retention path. We don't try to be the log database.
//
//  2. K8s Events (`events.k8s.io/v1`) — for in-cluster visibility + the
//     UI's audit page. Events have default 1-hour TTL but k8s-events
//     controller respects --event-ttl flag if customers want longer.
//     We label them `supkube.io/audit=true` so the UI's query can find
//     them via label selector without scanning unrelated events.
//
// We only audit MUTATING requests (POST/PUT/PATCH/DELETE) + login/logout +
// failed authentication. Read operations would inflate the event store
// 100× without proportional value — if customers want full request logs
// they should turn on nginx access logs, not bloat audit.
//
// Schema (matches what the UI consumes):
//
//	user          — email or username, fall back to "anonymous"
//	action        — derived: "Create" / "Update" / "Delete" / "Login" / ...
//	resource      — derived from URL: "Backup" / "Restore" / ...
//	resourceName  — captured from path param if available
//	namespace     — captured from request body / query when applicable
//	method        — verbatim HTTP method
//	path          — Gin pattern (e.g. /api/v1/backups/:name)
//	result        — "success" / "denied" / "error"
//	statusCode    — HTTP status response
//	sourceIP      — c.ClientIP() — may be inaccurate behind multiple proxies
//	timestamp     — server-side wall clock when the request completed
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supkube/supkube-backend/internal/k8s"
)

const (
	auditLabelKey   = "supkube.io/audit"
	auditLabelValue = "true"
	auditNamespace  = "supkube"
)

// AuditRecord is the shape the UI consumes via /api/v1/audit-logs.
type AuditRecord struct {
	User         string    `json:"user"`
	Action       string    `json:"action"`
	Resource     string    `json:"resource"`
	ResourceName string    `json:"resourceName,omitempty"`
	Namespace    string    `json:"namespace,omitempty"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Result       string    `json:"result"`
	StatusCode   int       `json:"statusCode"`
	SourceIP     string    `json:"sourceIP,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// AuditMiddleware wraps the response writer so we can inspect status code
// after the handler runs. INSTALLED FIRST in the middleware chain so its
// post-Next() code runs even when later middleware (auth, rbac) abort.
// That's how we capture failed authentication attempts and RBAC denials.
//
// Performance: writing a K8s Event per mutating request is fine at our
// scale (target: <10 mutations/sec sustained). For higher throughput we'd
// batch + flush via channel — leaving that for v0.9.
func (c *Config) AuditMiddleware() gin.HandlerFunc {
	return func(gc *gin.Context) {
		// Bypass paths — these aren't worth auditing (no state change,
		// no security signal). Note this is a SHORTER list than
		// isAuthBypassPath: we DO audit /auth/callback (login attempt)
		// and /auth/logout (session end) because those are interesting
		// for compliance.
		if isAuditBypassPath(gc.Request.URL.Path) {
			gc.Next()
			return
		}
		// Read operations skipped — see file header for rationale.
		if !isAuditable(gc.Request.Method) {
			gc.Next()
			return
		}

		// Capture request body so we can extract ns later (the handler
		// will also need to read it — we buffer + replace c.Request.Body).
		var bodyCopy []byte
		if gc.Request.Body != nil {
			bodyCopy, _ = io.ReadAll(gc.Request.Body)
			gc.Request.Body = io.NopCloser(bytes.NewBuffer(bodyCopy))
		}

		gc.Next()

		// Build the audit record AFTER the handler has run (status known).
		rec := buildAuditRecord(gc, bodyCopy)

		// 1. stdout — Loki/EFK ingestion.
		logAuditToStdout(&rec)

		// 2. K8s Event — UI display.
		writeAuditEvent(&rec)
	}
}

// isAuditBypassPath is audit's own bypass list — shorter than the auth
// bypass. Status checks and the provider list aren't worth recording.
// Login (callback) and logout ARE worth recording — they tell us
// authentication patterns and session boundaries.
func isAuditBypassPath(p string) bool {
	switch p {
	case "/api/v1/status",
		"/api/v1/auth/providers":
		return true
	}
	return false
}

// isAuditable returns true for HTTP methods that mutate state.
func isAuditable(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func buildAuditRecord(gc *gin.Context, bodyCopy []byte) AuditRecord {
	user := FromContext(gc)
	rec := AuditRecord{
		Method:     gc.Request.Method,
		Path:       gc.FullPath(),
		StatusCode: gc.Writer.Status(),
		SourceIP:   gc.ClientIP(),
		Timestamp:  time.Now().UTC(),
	}
	if user != nil {
		rec.User = user.Email
		if rec.User == "" {
			rec.User = user.Username
		}
	}
	if rec.User == "" {
		rec.User = "anonymous"
	}
	// result classification based on status code.
	switch {
	case rec.StatusCode >= 200 && rec.StatusCode < 300:
		rec.Result = "success"
	case rec.StatusCode == http.StatusForbidden || rec.StatusCode == http.StatusUnauthorized:
		rec.Result = "denied"
	default:
		rec.Result = "error"
	}
	// Action verb from HTTP method.
	rec.Action = methodToAction(gc.Request.Method)
	// Resource + name from URL.
	rec.Resource, rec.ResourceName = parseResourceFromPath(gc.FullPath(), gc.Param("name"))
	// Namespace — best-effort: try URL path, then body.
	rec.Namespace = extractNamespace(gc, bodyCopy)
	return rec
}

func methodToAction(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodPost:
		return "Create"
	case http.MethodPut:
		return "Update"
	case http.MethodPatch:
		return "Patch"
	case http.MethodDelete:
		return "Delete"
	}
	return method
}

// parseResourceFromPath turns "/api/v1/backups/:name" → ("Backup", actualName).
// Falls back to the path segment if we don't have a curated mapping.
func parseResourceFromPath(pattern, name string) (string, string) {
	if pattern == "" {
		return "Unknown", name
	}
	// Strip /api/v1/ prefix, take first segment as resource type.
	trimmed := strings.TrimPrefix(pattern, "/api/v1/")
	first := strings.SplitN(trimmed, "/", 2)[0]
	mapping := map[string]string{
		"backups":                   "Backup",
		"restores":                  "Restore",
		"schedules":                 "Schedule",
		"transform-sets":            "TransformSet",
		"storage-locations":         "StorageProfile",
		"volume-snapshot-locations": "SnapshotProfile",
		"namespaces":                "Namespace",
		"auth":                      "Auth",
	}
	if r, ok := mapping[first]; ok {
		return r, name
	}
	return strings.Title(first), name
}

// extractNamespace pulls a namespace value out of the request URL params
// or JSON body. Best-effort — empty string is acceptable.
func extractNamespace(gc *gin.Context, bodyCopy []byte) string {
	if ns := gc.Param("namespace"); ns != "" {
		return ns
	}
	if ns := gc.Query("namespace"); ns != "" {
		return ns
	}
	if len(bodyCopy) == 0 {
		return ""
	}
	// Try common body shapes (includedNamespaces[0], namespace, name for ns endpoint).
	var probe struct {
		Namespace          string   `json:"namespace"`
		Name               string   `json:"name"`
		IncludedNamespaces []string `json:"includedNamespaces"`
		TargetNamespace    string   `json:"targetNamespace"`
	}
	if err := json.Unmarshal(bodyCopy, &probe); err != nil {
		return ""
	}
	if probe.TargetNamespace != "" {
		return probe.TargetNamespace
	}
	if probe.Namespace != "" {
		return probe.Namespace
	}
	if len(probe.IncludedNamespaces) > 0 {
		return probe.IncludedNamespaces[0]
	}
	// For /namespaces POST, body.name IS the new ns name.
	if strings.Contains(gc.FullPath(), "/namespaces") && probe.Name != "" {
		return probe.Name
	}
	return ""
}

func logAuditToStdout(rec *AuditRecord) {
	data, _ := json.Marshal(rec)
	log.Printf("[audit] %s", data)
}

// writeAuditEvent fires a K8s Event for UI consumption. Best-effort —
// we never block the request on this. Errors are logged but not surfaced.
func writeAuditEvent(rec *AuditRecord) {
	cl, err := k8s.GetClient()
	if err != nil {
		log.Printf("[audit] cannot get k8s client to write Event: %v", err)
		return
	}
	// Build the Event. Use core/v1 Event (legacy) for broadest K8s
	// compatibility — events.k8s.io/v1 is preferred from 1.21+ but the
	// schema is more verbose and we don't need the new fields.
	now := metav1.Time{Time: rec.Timestamp}
	evType := corev1.EventTypeNormal
	if rec.Result != "success" {
		evType = corev1.EventTypeWarning
	}
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "supkube-audit-",
			Namespace:    auditNamespace,
			Labels: map[string]string{
				auditLabelKey:               auditLabelValue,
				"supkube.io/audit-user":     sanitizeLabel(rec.User),
				"supkube.io/audit-result":   rec.Result,
				"supkube.io/audit-resource": rec.Resource,
			},
			Annotations: map[string]string{
				"supkube.io/audit-method":        rec.Method,
				"supkube.io/audit-path":          rec.Path,
				"supkube.io/audit-status":        strconv.Itoa(rec.StatusCode),
				"supkube.io/audit-source-ip":     rec.SourceIP,
				"supkube.io/audit-namespace":     rec.Namespace,
				"supkube.io/audit-resource-name": rec.ResourceName,
				"supkube.io/audit-user-full":     rec.User,
			},
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: rec.Resource,
			Name: rec.ResourceName,
			// K8s validation requires involvedObject.namespace to match
			// event.namespace when the event lives in a namespace. Audit
			// events live in auditNamespace (supkube) regardless of which
			// target resource they describe; many records (e.g. Auth/
			// callback, cluster-scoped resources) carry an empty
			// rec.Namespace. Fall back to auditNamespace so the validator
			// is satisfied — the original target ns is still preserved in
			// the supkube.io/audit-namespace annotation above.
			Namespace: func() string {
				if rec.Namespace != "" {
					return rec.Namespace
				}
				return auditNamespace
			}(),
		},
		Reason:         fmt.Sprintf("%s%s", rec.Action, rec.Resource),
		Message:        fmt.Sprintf("user=%s method=%s path=%s ns=%s result=%s status=%d", rec.User, rec.Method, rec.Path, rec.Namespace, rec.Result, rec.StatusCode),
		Type:           evType,
		Source:         corev1.EventSource{Component: "supkube"},
		FirstTimestamp: now,
		LastTimestamp:  now,
		EventTime:      metav1.MicroTime{Time: rec.Timestamp},
		Action:         rec.Action,
		// K8s 1.27+ Event validation requires non-empty
		// reportingComponent + reportingInstance for events with an
		// EventTime set. Without these we get "Event is invalid"
		// rejections. The values are arbitrary but must be DNS-safe.
		ReportingController: "supkube.io/backend",
		ReportingInstance:   "supkube-backend",
	}
	if _, err := cl.CoreV1().Events(auditNamespace).Create(context.Background(), ev, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			log.Printf("[audit] write Event failed: %v", err)
		}
	}
}

// sanitizeLabel converts an arbitrary string into a K8s label value.
// K8s label values must be 0-63 chars, alphanumeric / dash / underscore /
// dot, must start and end alphanumeric.
//
// User emails / "anonymous" / random IDs — anything goes in. We replace
// non-conforming chars with '-' and truncate.
func sanitizeLabel(in string) string {
	if in == "" {
		return "unknown"
	}
	var sb strings.Builder
	for i, r := range in {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.' || r == '-' || r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteByte('-')
		}
		if i >= 62 {
			break
		}
	}
	out := sb.String()
	// Ensure start + end alphanumeric.
	out = strings.Trim(out, "-_.")
	if out == "" {
		return "sanitized"
	}
	if len(out) > 63 {
		out = out[:63]
	}
	return out
}

// ListAuditLogs returns recent audit records, parsed from K8s Events.
//
// Admin-only — the central permission table enforces this.
//
// Query params (all optional):
//
//	user      — filter to a specific user
//	result    — "success" / "denied" / "error"
//	resource  — "Backup" / "Restore" / ...
//	since     — RFC3339 timestamp (only return records after this time)
//	limit     — default 200, max 1000
//
// Returns newest first.
func ListAuditLogs(gc *gin.Context) {
	cl, err := k8s.GetClient()
	if err != nil {
		gc.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	selector := auditLabelKey + "=" + auditLabelValue
	events, err := cl.CoreV1().Events(auditNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		gc.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	limit := 200
	if l := gc.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	filterUser := gc.Query("user")
	filterResult := gc.Query("result")
	filterResource := gc.Query("resource")
	var sinceT time.Time
	if s := gc.Query("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			sinceT = t
		}
	}
	records := make([]AuditRecord, 0, len(events.Items))
	for i := range events.Items {
		ev := &events.Items[i]
		rec := eventToAuditRecord(ev)
		if filterUser != "" && rec.User != filterUser {
			continue
		}
		if filterResult != "" && rec.Result != filterResult {
			continue
		}
		if filterResource != "" && rec.Resource != filterResource {
			continue
		}
		if !sinceT.IsZero() && rec.Timestamp.Before(sinceT) {
			continue
		}
		records = append(records, rec)
	}
	// Sort newest first.
	for i := 0; i < len(records); i++ {
		for j := i + 1; j < len(records); j++ {
			if records[j].Timestamp.After(records[i].Timestamp) {
				records[i], records[j] = records[j], records[i]
			}
		}
	}
	if len(records) > limit {
		records = records[:limit]
	}
	gc.JSON(http.StatusOK, gin.H{"items": records, "total": len(records)})
}

func eventToAuditRecord(ev *corev1.Event) AuditRecord {
	an := ev.Annotations
	if an == nil {
		an = map[string]string{}
	}
	statusCode, _ := strconv.Atoi(an["supkube.io/audit-status"])
	ts := ev.LastTimestamp.Time
	if !ev.EventTime.IsZero() {
		ts = ev.EventTime.Time
	}
	if ts.IsZero() {
		ts = ev.CreationTimestamp.Time
	}
	return AuditRecord{
		User:         an["supkube.io/audit-user-full"],
		Action:       ev.Action,
		Resource:     ev.InvolvedObject.Kind,
		ResourceName: an["supkube.io/audit-resource-name"],
		Namespace:    an["supkube.io/audit-namespace"],
		Method:       an["supkube.io/audit-method"],
		Path:         an["supkube.io/audit-path"],
		Result:       ev.Labels["supkube.io/audit-result"],
		StatusCode:   statusCode,
		SourceIP:     an["supkube.io/audit-source-ip"],
		Timestamp:    ts,
	}
}

// ListRBACBindings returns the Helm-injected bindings list for the admin
// audit UI. Read-only — we never let UI mutate (per ADR-004).
func (c *Config) ListRBACBindings(gc *gin.Context) {
	gc.JSON(http.StatusOK, gin.H{
		"enabled":     c.RBACEnabled,
		"defaultRole": c.DefaultRole,
		"bindings":    c.RoleBindings,
	})
}
