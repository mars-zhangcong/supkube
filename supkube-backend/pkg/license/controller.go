package license

import (
	"context"
	"log"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	namespace   = "supkube"
	secretName  = "supkube-license"
	statusCM    = "supkube-license-status" // event target object
	graceWindow = 7 * 24 * time.Hour       // keep "writes" alive 7d past expiry
	pollEvery   = 5 * time.Minute
)

// State enum values returned to /status and used by the write gate.
const (
	StateLicensed = "licensed" // valid + not expired
	StateGrace    = "grace"    // valid but expired within the grace window
	StateDegraded = "degraded" // valid but expired past grace (writes blocked)
	StateMissing  = "missing"  // no Secret (writes blocked)
	StateInvalid  = "invalid"  // Secret present, signature bad (writes blocked)
)

// Status is the cached, thread-safe snapshot the controller keeps current and
// the HTTP handler / metrics collector / write gate read.
type Status struct {
	License      *License
	State        string
	NodeCount    int      // billable worker nodes
	NodeExcluded int      // control-plane / tainted (infra) nodes
	Violations   []string // e.g. "NodeCountExceeded"
	LastChecked  time.Time
}

var (
	mu      sync.RWMutex
	current = Status{State: StateMissing}
)

// Snapshot returns a copy of the current license status.
func Snapshot() Status {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

func setStatus(s Status) {
	mu.Lock()
	current = s
	mu.Unlock()
}

// Allowed reports whether write operations (creating new backup tasks) are
// permitted. Writes are allowed while licensed or within the grace window;
// missing / invalid / degraded block writes. Reads and restores are NEVER
// gated by this — the controller never calls Allowed for those.
func Allowed() bool {
	s := Snapshot()
	return s.State == StateLicensed || s.State == StateGrace
}

// DaysLeft returns whole days until expiry (negative if expired). 0 if no license.
func (s Status) DaysLeft() int {
	if s.License == nil {
		return 0
	}
	return int(time.Until(s.License.DateEnd).Hours() / 24)
}

type controller struct {
	k8sCli   kubernetes.Interface
	recorder record.EventRecorder
	target   *corev1.ConfigMap // event object

	// event throttle state (avoid spamming every 5m tick)
	lastState        string
	lastLoadedID     string
	lastExceeded     bool
	lastExpSoonDate  string // yyyy-mm-dd of last LicenseExpiringSoon
}

// Run starts the license controller loop: an immediate check then every 5
// minutes. Blocks until ctx is cancelled. Mirrors the other server.Run()
// background controllers (gc/eventwatch/clusterhealth).
func Run(ctx context.Context, k8sCli kubernetes.Interface) {
	c := &controller{k8sCli: k8sCli}
	c.recorder = newRecorder(k8sCli)
	c.target = ensureStatusObject(ctx, k8sCli)

	c.check(ctx)
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.check(ctx)
		}
	}
}

func (c *controller) check(ctx context.Context) {
	st := Status{State: StateMissing, LastChecked: time.Now()}

	sec, err := c.k8sCli.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		st.State = StateMissing
	case err != nil:
		// Transient API error — log and treat as missing for this tick (we do
		// not want a flaky apiserver to look like a valid license).
		log.Printf("[license] read Secret %s/%s failed: %v", namespace, secretName, err)
		st.State = StateMissing
	default:
		lic, verr := Verify(sec.Data["license"])
		if verr != nil {
			st.State = StateInvalid
			log.Printf("[license] signature verification failed: %v", verr)
		} else {
			st.License = lic
			st.State = classifyExpiry(lic, time.Now())
		}
	}

	// Node count is independent of license validity (Kasten does this warn-only).
	st.NodeCount, st.NodeExcluded = countBillableNodes(ctx, c.k8sCli)
	if st.License != nil && st.License.Restrictions.Nodes > 0 && st.NodeCount > st.License.Restrictions.Nodes {
		st.Violations = append(st.Violations, "NodeCountExceeded")
	}

	setStatus(st)
	c.emitEvents(st)
	logStatus(st)
}

// classifyExpiry maps a cryptographically-valid license to a lifecycle state.
func classifyExpiry(lic *License, now time.Time) string {
	if now.Before(lic.DateEnd) {
		return StateLicensed
	}
	if now.Sub(lic.DateEnd) <= graceWindow {
		return StateGrace
	}
	return StateDegraded
}

// countBillableNodes returns (billable, excluded). Billable = worker nodes;
// excluded = control-plane or infra nodes (tainted NoSchedule/NoExecute) —
// aligning with Kasten K10's "License Exclusion Nodes".
func countBillableNodes(ctx context.Context, k8sCli kubernetes.Interface) (billable, excluded int) {
	nl, err := k8sCli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[license] list nodes failed: %v", err)
		return 0, 0
	}
	for i := range nl.Items {
		if isExcludedNode(&nl.Items[i]) {
			excluded++
		} else {
			billable++
		}
	}
	return billable, excluded
}

func isExcludedNode(n *corev1.Node) bool {
	if _, ok := n.Labels["node-role.kubernetes.io/control-plane"]; ok {
		return true
	}
	if _, ok := n.Labels["node-role.kubernetes.io/master"]; ok {
		return true
	}
	for _, t := range n.Spec.Taints {
		if t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute {
			return true
		}
	}
	return false
}

func logStatus(st Status) {
	if st.License != nil {
		log.Printf("[license] state=%s id=%s product=%s edition=%s nodes=%d/%d excluded=%d daysLeft=%d violations=%v",
			st.State, st.License.ID, st.License.Product, st.License.Edition,
			st.NodeCount, st.License.Restrictions.Nodes, st.NodeExcluded, st.DaysLeft(), st.Violations)
	} else {
		log.Printf("[license] state=%s nodes=%d excluded=%d", st.State, st.NodeCount, st.NodeExcluded)
	}
}

// ── events ────────────────────────────────────────────────────────────────

func newRecorder(k8sCli kubernetes.Interface) record.EventRecorder {
	b := record.NewBroadcaster()
	b.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: k8sCli.CoreV1().Events(namespace)})
	return b.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "supkube-license"})
}

// ensureStatusObject makes sure the ConfigMap we hang license Events off exists
// (Events need a real object with a UID). Best-effort: on failure we return a
// bare reference so recording still degrades gracefully.
func ensureStatusObject(ctx context.Context, k8sCli kubernetes.Interface) *corev1.ConfigMap {
	cm, err := k8sCli.CoreV1().ConfigMaps(namespace).Get(ctx, statusCM, metav1.GetOptions{})
	if err == nil {
		return cm
	}
	created, cerr := k8sCli.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statusCM,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "supkube", "supkube.io/component": "license"},
		},
	}, metav1.CreateOptions{})
	if cerr != nil {
		log.Printf("[license] ensure status ConfigMap failed: %v", cerr)
		return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: statusCM, Namespace: namespace}}
	}
	return created
}

func (c *controller) emitEvents(st Status) {
	if c.recorder == nil || c.target == nil {
		return
	}
	today := time.Now().Format("2006-01-02")

	// State-transition events (fire once when the state changes).
	if st.State != c.lastState {
		switch st.State {
		case StateMissing:
			c.recorder.Event(c.target, corev1.EventTypeWarning, "LicenseMissing",
				"No supkube-license Secret found; running in degraded mode (reads/restores stay available, new backups blocked)")
		case StateInvalid:
			c.recorder.Event(c.target, corev1.EventTypeWarning, "LicenseInvalid",
				"License signature is invalid (Secret may have been edited); running in degraded mode")
		case StateDegraded:
			c.recorder.Event(c.target, corev1.EventTypeWarning, "LicenseExpired",
				"License expired more than 7 days ago; running in degraded mode (new backups blocked)")
		}
		c.lastState = st.State
	}

	if st.License != nil {
		// Loaded once per license id (first load or a new license rev).
		if c.lastLoadedID != st.License.ID {
			c.recorder.Eventf(c.target, corev1.EventTypeNormal, "LicenseLoaded",
				"License loaded: id=%s product=%s edition=%s nodes=%d daysLeft=%d",
				st.License.ID, st.License.Product, st.License.Edition, st.License.Restrictions.Nodes, st.DaysLeft())
			c.lastLoadedID = st.License.ID
		}
		// Expiring soon: within 30 days, at most once per day.
		if d := st.DaysLeft(); d >= 0 && d <= 30 && c.lastExpSoonDate != today {
			c.recorder.Eventf(c.target, corev1.EventTypeWarning, "LicenseExpiringSoon",
				"License expires in %d day(s) (id=%s)", d, st.License.ID)
			c.lastExpSoonDate = today
		}
	}

	// Node quota exceeded: warn-only, fire on transition into violation.
	exceeded := contains(st.Violations, "NodeCountExceeded")
	if exceeded && !c.lastExceeded {
		max := 0
		if st.License != nil {
			max = st.License.Restrictions.Nodes
		}
		c.recorder.Eventf(c.target, corev1.EventTypeWarning, "LicenseNodeCountExceeded",
			"Billable node count %d exceeds licensed maximum %d (add nodes to your license; not blocking)", st.NodeCount, max)
	}
	c.lastExceeded = exceeded
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
