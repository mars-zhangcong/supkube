// Package gc: periodic orphan-GC runner + Activity event emitter.
//
// The runner is one goroutine per pod (single replica today, sharded
// election later if we go HA). It reads settings from a ConfigMap on
// every tick — so flipping the toggle in the Settings UI takes effect
// at the next tick without a pod restart.
//
// Settings ConfigMap shape (cm: supkube-settings in supkube namespace):
//
//	data:
//	  cleanup.enabled:        "true"  | "false"   (default true)
//	  cleanup.intervalHours:  "6"                  (default 6)
//
// Activity events: every scan that finds OR deletes anything writes a
// K8s Event in the supkube ns with reportingController=supkube.io/gc.
// SupKube's existing Activity page reads K8s Events labeled with
// supkube.io/activity=true, so adding the label is enough — no schema
// changes to the Activity API.
package gc

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	settingsConfigMap  = "supkube-settings"
	settingsNamespace  = "supkube"
	defaultIntervalHrs = 6
)

// Settings is the parsed view of the ConfigMap. Always returns sensible
// defaults when the CM is missing or fields are unset — the GC must be
// usable on a fresh install before anyone touches Settings.
type Settings struct {
	Enabled       bool
	IntervalHours int
}

// LoadSettings reads the ConfigMap and parses the relevant keys. Missing
// CM → defaults (enabled=true, interval=6h). We deliberately default to
// ON because the orphan problem causes silent storage cost — surfacing it
// behind an opt-in toggle would leave customers paying for orphans they
// never notice.
func LoadSettings(ctx context.Context, k8sCli kubernetes.Interface) Settings {
	s := Settings{Enabled: true, IntervalHours: defaultIntervalHrs}
	cm, err := k8sCli.CoreV1().ConfigMaps(settingsNamespace).Get(ctx, settingsConfigMap, metav1.GetOptions{})
	if err != nil {
		// CM missing on first run is normal; return defaults silently.
		return s
	}
	if v, ok := cm.Data["cleanup.enabled"]; ok {
		s.Enabled = strings.EqualFold(strings.TrimSpace(v), "true")
	}
	if v, ok := cm.Data["cleanup.intervalHours"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 1 && n <= 168 {
			s.IntervalHours = n
		}
	}
	return s
}

// SaveSettings persists user-edited settings into the ConfigMap. Creates
// the CM if it doesn't exist yet. Caller is the PUT endpoint handler.
func SaveSettings(ctx context.Context, k8sCli kubernetes.Interface, s Settings) error {
	data := map[string]string{
		"cleanup.enabled":       strconv.FormatBool(s.Enabled),
		"cleanup.intervalHours": strconv.Itoa(s.IntervalHours),
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      settingsConfigMap,
			Namespace: settingsNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "supkube",
				"supkube.io/component":         "settings",
			},
		},
		Data: data,
	}
	// Upsert: try Update first (common case after first save), fall back
	// to Create on NotFound.
	_, err := k8sCli.CoreV1().ConfigMaps(settingsNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	if apierrors.IsNotFound(err) {
		_, err = k8sCli.CoreV1().ConfigMaps(settingsNamespace).Create(ctx, cm, metav1.CreateOptions{})
	}
	return err
}

// lastRunCache holds the most recent ScanResult for the Settings UI's
// "last run" display. In-memory is fine: a pod restart wipes it, but the
// UI just shows "no recent run" until the next scan completes.
var lastRunCache ScanResult

// LastRun returns the most recent scan result (or zero value if none yet).
func LastRun() ScanResult { return lastRunCache }

// RunOnce is the manual-trigger entry point used by the
// POST /admin/cleanup/orphans handler. Wraps Scan with event emission +
// last-run cache update.
func RunOnce(ctx context.Context, cl client.Client, k8sCli kubernetes.Interface, trigger string) ScanResult {
	r := Scan(ctx, cl)
	lastRunCache = r
	emitEvent(ctx, k8sCli, r, trigger)
	return r
}

// Run starts the periodic ticker. Returns immediately; goroutine runs
// for the lifetime of the process. Re-reads settings on every tick so
// UI-driven config changes take effect by the next interval boundary.
//
// Tick cadence: we use a 5-minute granularity TICKER + check elapsed
// time against current intervalHours. Picking a finer ticker lets us
// react quickly to settings changes (toggling enabled OFF stops new
// scans within 5 min) while keeping the actual scan rate at the
// user-chosen interval.
func Run(ctx context.Context, cl client.Client, k8sCli kubernetes.Interface) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	var lastScan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s := LoadSettings(ctx, k8sCli)
			if !s.Enabled {
				continue
			}
			if time.Since(lastScan) < time.Duration(s.IntervalHours)*time.Hour {
				continue
			}
			log.Printf("[gc] periodic scan starting (interval=%dh)", s.IntervalHours)
			r := Scan(ctx, cl)
			lastRunCache = r
			emitEvent(ctx, k8sCli, r, "periodic")
			lastScan = time.Now()
		}
	}
}

// emitEvent writes a K8s Event for this scan run. Used by both periodic
// and manual triggers. The event surfaces on the Activity page because
// SupKube's audit / activity reader includes events with the
// supkube.io/activity=true label.
//
// We only emit if something was found OR if it was a manual trigger.
// A periodic scan that found zero orphans would create noise in
// Activity — better to be quiet on no-op periodics.
func emitEvent(ctx context.Context, k8sCli kubernetes.Interface, r ScanResult, trigger string) {
	if trigger == "periodic" && r.VSCDeleted+r.VSDeleted+r.PVBDeleted+r.DUDeleted == 0 && r.Err == nil {
		return
	}
	t := metav1.NewMicroTime(r.FinishedAt)
	evtType := corev1.EventTypeNormal
	reason := "OrphanCleanup"
	if r.Err != nil {
		evtType = corev1.EventTypeWarning
		reason = "OrphanCleanupFailed"
	}
	evt := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "orphan-gc-",
			Namespace:    settingsNamespace,
			Labels: map[string]string{
				"supkube.io/activity":  "true", // ← Activity page picks this up
				"supkube.io/component": "gc",
				"supkube.io/trigger":   trigger, // periodic | manual | post-delete
			},
		},
		EventTime: t,
		Type:      evtType,
		Reason:    reason,
		Note:      r.Summary(),
		Regarding: corev1.ObjectReference{
			Kind:      "ConfigMap",
			Namespace: settingsNamespace,
			Name:      settingsConfigMap,
		},
		ReportingController: "supkube.io/gc",
		ReportingInstance:   "supkube-backend",
		Action:              "Cleanup",
	}
	if _, err := k8sCli.EventsV1().Events(settingsNamespace).Create(ctx, evt, metav1.CreateOptions{}); err != nil {
		log.Printf("[gc] failed to emit event: %v", err)
	}
}

// triggerSoonAt holds the in-flight one-shot scan request. When the
// user deletes a backup, we don't scan immediately (Velero's DBR
// controller hasn't finished yet) — instead we schedule a scan ~60s
// later. Multiple rapid deletes collapse onto the same scheduled scan
// (debounce), avoiding O(deletes) K8s API churn.
var (
	pendingTimer *time.Timer
)

// TriggerSoon schedules a one-shot scan ~60 seconds from now. If called
// again before that fires, the timer resets (debounce). Safe to call
// from any handler that mutates Velero state in a way that might leave
// orphans (Backup delete, BSL delete, ad-hoc kubectl interventions, …).
//
// Why 60s: Velero's DeleteBackupRequest controller typically processes
// within 30s. 60s gives margin without making the user wait. The
// follow-up scan finds orphans whose parent is now gone and sweeps them.
func TriggerSoon(ctx context.Context, cl client.Client, k8sCli kubernetes.Interface) {
	if pendingTimer != nil {
		pendingTimer.Stop()
	}
	pendingTimer = time.AfterFunc(60*time.Second, func() {
		// Detach from the request context — the delete handler that
		// triggered us is long gone. Use a fresh background context so
		// cancellation of the request doesn't kill our scan.
		bg := context.Background()
		r := Scan(bg, cl)
		lastRunCache = r
		emitEvent(bg, k8sCli, r, "post-delete")
	})
}
