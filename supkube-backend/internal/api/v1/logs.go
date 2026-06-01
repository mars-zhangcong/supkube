// Package v1: GET /api/v1/logs — replaces "kubectl logs satisfied scroll"
// for SupKube operators. Tab is "可观测性 → 日志查看器" in the UI.
//
// # Why this exists (task #79)
//
// On every demo a customer hits some snag — backup stuck, restore failing,
// node-agent NotReady — and the first thing they need is "what does the
// log say?" Before this endpoint they had three bad options:
//
//  1. SSH into a bastion + run `kubectl logs -n supkube ...`
//     (assumes a kubectl + RBAC + the customer can spell the pod name)
//  2. Open the K8s dashboard (not always installed, no severity filter)
//  3. Wait for SupKube support to ask them for a tarball
//
// All three break demo flow. This endpoint puts every relevant component
// (SupKube backend, frontend, Velero server, node-agent, Dex) one click
// away with component + severity + time-window filters, returning a
// uniform LogLine shape the UI can colorize and grep client-side.
//
// # Scope of this v1
//
//   - Read-only, latest N lines (cap 2000) per pod, merged across replicas
//   - Filters: component, sinceSeconds, tailLines, grep (server-side, case-insensitive)
//   - Optional ?download=1 returns text/plain with attachment header
//
// # NOT done in v1 (intentional, push to v1.x)
//
//   - SSE / WebSocket live tail: 5s client poll is sufficient for demo
//     debugging and avoids dragging a streaming framework into the gin
//     handler. Add later if customers complain.
//   - Multi-cluster: this calls the local k8s client only. Remote-cluster
//     logs go through the existing cross-cluster API router.
//   - Persisted log aggregation (Loki / Elastic): we're a backup product,
//     not a log platform; integrate, don't own.
package v1

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// LogLine is one rendered line as the UI consumes it. The frontend
// colorizes by Severity and groups by Component, so those are extracted
// server-side rather than reparsed in JS.
type LogLine struct {
	Timestamp string `json:"timestamp"` // RFC3339 if available, else empty
	Component string `json:"component"` // logical component key (backend/velero/...)
	Pod       string `json:"pod"`       // source pod name (multi-replica fan-in)
	Severity  string `json:"severity"`  // ERROR / WARN / INFO / DEBUG / UNKNOWN
	Message   string `json:"message"`   // log body (timestamp stripped)
}

// LogsResponse wraps lines with the parameters echoed back, so the UI
// can compute "what to download" without re-deriving the query state.
type LogsResponse struct {
	Component     string    `json:"component"`
	PodCount      int       `json:"podCount"`
	Lines         []LogLine `json:"lines"`
	TruncatedAt   int       `json:"truncatedAt"` // 0 if no cap hit
	GeneratedAt   time.Time `json:"generatedAt"`
	WarningNotice string    `json:"warningNotice,omitempty"`
}

// componentSpec maps a UI-facing component key onto the namespace +
// label selector that locates its pods. Keeping this in code (not
// config) is deliberate — it's tied to our Helm chart's labeling.
// If you add a new chart-installed component, add it here AND in the
// frontend `LogViewer.vue` component dropdown.
//
// SELF-AWARENESS NOTE (2026-05-30, task #79 follow-up):
// The customer cannot accept "Is SupKube Backend actually installed?"
// when they're currently INSIDE SupKube Backend. SupKube-owned
// components (backend/frontend/dex) flip `isSelf=true` so the empty-
// state messaging blames the right thing (SA RBAC, controller crash)
// instead of asking the user a question whose answer is self-evident.
// The selectors below are also the *actual* labels emitted by our
// helm chart (verified 2026-05-30 against both docker-desktop and AKS
// installs), not the guess I shipped first.
type componentSpec struct {
	Namespace string
	Selector  string // K8s label selector, comma-separated
	Display   string // for error messages
	IsSelf    bool   // True when this component IS SupKube; empty result is a bug, not "not installed"
}

var componentRegistry = map[string]componentSpec{
	// SupKube's own helm chart sets `app.kubernetes.io/name=supkube` on
	// every component (it's the chart name, by Helm convention) and
	// uses `app.kubernetes.io/component` to distinguish backend/frontend/
	// dex. The earlier guess `app.kubernetes.io/name=supkube-backend`
	// matched ZERO pods — see the customer screenshot bug.
	"backend":  {Namespace: "supkube", Selector: "app.kubernetes.io/name=supkube,app.kubernetes.io/component=backend", Display: "SupKube Backend", IsSelf: true},
	"frontend": {Namespace: "supkube", Selector: "app.kubernetes.io/name=supkube,app.kubernetes.io/component=frontend", Display: "SupKube Frontend", IsSelf: true},
	"dex":      {Namespace: "supkube", Selector: "app.kubernetes.io/name=supkube,app.kubernetes.io/component=dex", Display: "Dex (OIDC)", IsSelf: true},
	// Velero ships pods with `deploy=velero` selector (per the upstream
	// velero helm chart's Deployment.spec.selector.matchLabels). node-agent
	// is a DaemonSet, labelled `name=node-agent`. Both verified.
	"velero":     {Namespace: "velero", Selector: "deploy=velero", Display: "Velero Server"},
	"node-agent": {Namespace: "velero", Selector: "name=node-agent", Display: "Velero node-agent (DaemonSet)"},
}

// GetLogComponents returns the list of components for the UI dropdown.
// Avoids the frontend hardcoding a parallel list that drifts.
func GetLogComponents(c *gin.Context) {
	out := make([]gin.H, 0, len(componentRegistry))
	for key, spec := range componentRegistry {
		out = append(out, gin.H{
			"key":       key,
			"display":   spec.Display,
			"namespace": spec.Namespace,
		})
	}
	// Stable order — UI dropdown should never reshuffle randomly.
	sort.Slice(out, func(i, j int) bool { return out[i]["key"].(string) < out[j]["key"].(string) })
	c.JSON(http.StatusOK, gin.H{"components": out})
}

// GetLogs is the workhorse — fans out to N pods, merges, filters, returns.
//
// Query params:
//
//	component     required, one of componentRegistry keys
//	sinceSeconds  optional, default 3600 (1h), max 86400 (24h)
//	tailLines     optional, default 500, max 2000 per pod
//	grep          optional, case-insensitive substring filter on Message
//	severity      optional, one of ERROR/WARN/INFO/DEBUG to filter
//	download      optional, "1" returns text/plain with attachment header
func GetLogs(c *gin.Context) {
	componentKey := c.Query("component")
	spec, ok := componentRegistry[componentKey]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "unknown component",
			"validValues": componentRegistryKeys(),
		})
		return
	}
	sinceSeconds := parseIntDefault(c.Query("sinceSeconds"), 3600, 60, 86400)
	tailLines := parseIntDefault(c.Query("tailLines"), 500, 50, 2000)
	grep := strings.ToLower(strings.TrimSpace(c.Query("grep")))
	severity := strings.ToUpper(strings.TrimSpace(c.Query("severity")))
	isDownload := c.Query("download") == "1"

	k8sCli, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Step 1: list matching pods. If zero, return helpful empty payload
	// rather than 404 — the UI shows "no pods match this component" in
	// the empty-state, which is more debuggable than a hard error.
	pods, err := k8sCli.CoreV1().Pods(spec.Namespace).List(c.Request.Context(), metav1.ListOptions{
		LabelSelector: spec.Selector,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "list pods failed: " + err.Error(),
			"component": componentKey,
			"hint":      "this often means RBAC is missing get/list pods in " + spec.Namespace,
		})
		return
	}

	resp := LogsResponse{
		Component:   componentKey,
		PodCount:    len(pods.Items),
		GeneratedAt: time.Now().UTC(),
	}
	if len(pods.Items) == 0 {
		// Empty-result UX matters here: asking the user "is X
		// installed?" when X is the very software they're using is
		// self-defeating. Split by IsSelf:
		//   - SupKube-owned (backend/frontend/dex): empty = bug,
		//     almost certainly the ServiceAccount lacks get/list
		//     pods in `supkube`. Point the customer at the fix.
		//   - Third-party (velero/node-agent): empty CAN legitimately
		//     mean "not installed", so the legacy phrasing is fine.
		if spec.IsSelf {
			resp.WarningNotice = fmt.Sprintf(
				"%s pods are not visible to SupKube's ServiceAccount in namespace %q. "+
					"This is a SupKube install issue (the SA likely lacks get/list pods), "+
					"NOT a 'did you install it' question — you obviously did, you're using it. "+
					"Workaround: kubectl auth can-i list pods -n %s --as=system:serviceaccount:supkube:supkube-backend",
				spec.Display, spec.Namespace, spec.Namespace)
		} else {
			resp.WarningNotice = fmt.Sprintf(
				"No %s pods found in namespace %q (selector %q). %s may not be installed in this cluster.",
				spec.Display, spec.Namespace, spec.Selector, spec.Display)
		}
		if isDownload {
			c.String(http.StatusOK, resp.WarningNotice)
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	// Step 2: fan out — for each pod, stream the last N lines with
	// timestamps. We do this sequentially to keep the K8s API call
	// rate reasonable; parallel goroutines saved <1s for typical 1-3
	// replicas and added complexity, so skipped.
	tailLines64 := int64(tailLines)
	sinceSeconds64 := int64(sinceSeconds)
	for _, pod := range pods.Items {
		opts := &corev1.PodLogOptions{
			Timestamps:   true,
			TailLines:    &tailLines64,
			SinceSeconds: &sinceSeconds64,
		}
		req := k8sCli.CoreV1().Pods(spec.Namespace).GetLogs(pod.Name, opts)
		stream, serr := req.Stream(c.Request.Context())
		if serr != nil {
			// Don't fail the whole response on one pod's log fetch —
			// e.g. a pod stuck in CrashLoopBackOff often denies log
			// reads. Inject a synthetic line and move on.
			resp.Lines = append(resp.Lines, LogLine{
				Timestamp: resp.GeneratedAt.Format(time.RFC3339),
				Component: componentKey,
				Pod:       pod.Name,
				Severity:  "ERROR",
				Message:   "[supkube] failed to read logs from this pod: " + serr.Error(),
			})
			continue
		}
		reader := bufio.NewReader(stream)
		for {
			line, rerr := reader.ReadString('\n')
			if line != "" {
				ll := parseLine(line, componentKey, pod.Name)
				if grep != "" && !strings.Contains(strings.ToLower(ll.Message), grep) {
					goto next
				}
				if severity != "" && severity != "ANY" && ll.Severity != severity {
					goto next
				}
				resp.Lines = append(resp.Lines, ll)
			}
		next:
			if rerr != nil {
				break // io.EOF or read error — end of this pod
			}
		}
		_ = stream.Close()
	}

	// Step 3: sort across pods by timestamp so the UI shows a single
	// merged stream. Pods without timestamps (rare with Timestamps=true)
	// sort to the end of their respective ties.
	sort.SliceStable(resp.Lines, func(i, j int) bool {
		return resp.Lines[i].Timestamp < resp.Lines[j].Timestamp
	})

	// Step 4: enforce a hard global cap so the UI never has to render
	// >10k lines. If we hit it, signal so the user knows to narrow the
	// query rather than scrolling forever.
	const hardCap = 10000
	if len(resp.Lines) > hardCap {
		resp.TruncatedAt = hardCap
		resp.Lines = resp.Lines[len(resp.Lines)-hardCap:]
		resp.WarningNotice = fmt.Sprintf(
			"Output truncated to last %d lines. Narrow with --since-seconds, --tail-lines, or --grep.", hardCap)
	}

	// Step 5: optional download mode — emit text/plain instead of JSON.
	if isDownload {
		filename := fmt.Sprintf("supkube-%s-logs-%s.txt", componentKey, resp.GeneratedAt.Format("20060102-150405"))
		c.Writer.Header().Set("Content-Disposition", "attachment; filename="+filename)
		c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		c.Writer.WriteHeader(http.StatusOK)
		if resp.WarningNotice != "" {
			fmt.Fprintln(c.Writer, "# "+resp.WarningNotice)
		}
		fmt.Fprintf(c.Writer, "# component=%s pods=%d generated=%s\n", resp.Component, resp.PodCount, resp.GeneratedAt.Format(time.RFC3339))
		for _, ll := range resp.Lines {
			fmt.Fprintf(c.Writer, "%s [%s] %s/%s %s\n", ll.Timestamp, ll.Severity, ll.Component, ll.Pod, ll.Message)
		}
		return
	}
	c.JSON(http.StatusOK, resp)
}

// parseLine takes a raw `<RFC3339-timestamp> <body>\n` line (PodLogOptions
// Timestamps=true) and extracts the timestamp + heuristic severity. The
// severity heuristic intentionally favors common patterns over a regex
// grammar — covering 99% of the Go and Vue logger outputs we ship.
func parseLine(raw, component, pod string) LogLine {
	body := strings.TrimRight(raw, "\r\n")
	ll := LogLine{Component: component, Pod: pod, Severity: "UNKNOWN", Message: body}
	if idx := strings.IndexByte(body, ' '); idx > 0 {
		ts := body[:idx]
		// Validate it parses as RFC3339(Nano) — guards against logs
		// that start with non-timestamp content.
		if _, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			ll.Timestamp = ts
			ll.Message = body[idx+1:]
		}
	}
	ll.Severity = detectSeverity(ll.Message)
	return ll
}

// detectSeverity is the heuristic. Add patterns as we see new loggers;
// "UNKNOWN" is fine — UI greys those out, doesn't lose them.
func detectSeverity(msg string) string {
	upper := strings.ToUpper(msg)
	switch {
	case strings.Contains(upper, "ERROR") || strings.Contains(upper, "FATAL") ||
		strings.Contains(upper, "PANIC") || strings.Contains(upper, "\"LEVEL\":\"ERROR\""):
		return "ERROR"
	case strings.Contains(upper, "WARN") || strings.Contains(upper, "WARNING") ||
		strings.Contains(upper, "\"LEVEL\":\"WARN\""):
		return "WARN"
	case strings.Contains(upper, "DEBUG") || strings.Contains(upper, "\"LEVEL\":\"DEBUG\""):
		return "DEBUG"
	case strings.Contains(upper, "INFO") || strings.Contains(upper, "\"LEVEL\":\"INFO\""):
		return "INFO"
	}
	return "UNKNOWN"
}

func parseIntDefault(s string, def, min, max int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func componentRegistryKeys() []string {
	out := make([]string, 0, len(componentRegistry))
	for k := range componentRegistry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Compile-time guard: io.Reader is imported transitively via bufio but
// the linter sometimes flags the bufio.NewReader path. Touch it here so
// the import line never gets auto-pruned if we refactor.
var _ io.Reader = (*bufio.Reader)(nil)
