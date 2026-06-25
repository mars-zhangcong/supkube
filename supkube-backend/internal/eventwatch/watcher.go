// Package eventwatch polls Velero Backup/Restore CRs and publishes terminal
// events (*.completed / *.failed) on phase transition — the piece SupInsight's
// review flagged as missing: the thin-adapter "trigger → wait for completion"
// has nothing to wait on without these.
//
// Poll+diff (not an informer) to match the existing background-runner pattern
// (gc / policypair / clusterhealth). First pass SEEDS current phases without
// emitting, so existing-completed backups don't replay a storm on startup.
// The bus is lossy by design, so consumers also poll a status skill as the
// authoritative fallback (handoff §5.3).
package eventwatch

import (
	"context"
	"time"

	"github.com/supkube/supkube-backend/internal/events"
	"github.com/supkube/supkube-backend/internal/velerons"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultInterval = 10 * time.Second

// phaseItem is one CR's current state, normalized for diffing.
type phaseItem struct {
	key          string // kind/namespace/name (stable identity)
	subject      string // CR name (event subject)
	phase        string
	terminalType string // events.Type* if phase is terminal, else ""
}

// diffPhases is the pure core: for changed/new items whose phase is terminal,
// return events to publish; always update `last`. No emits until `seeded`.
func diffPhases(last map[string]string, seeded bool, items []phaseItem) []events.Event {
	var out []events.Event
	for _, it := range items {
		prev, ok := last[it.key]
		if seeded && it.terminalType != "" && (!ok || prev != it.phase) {
			out = append(out, events.Event{
				Type: it.terminalType, Subject: it.subject,
				Data: map[string]any{"phase": it.phase},
			})
		}
		last[it.key] = it.phase
	}
	return out
}

func terminalBackup(p velerov1.BackupPhase) string {
	switch p {
	case velerov1.BackupPhaseCompleted:
		return events.TypeBackupCompleted
	case velerov1.BackupPhaseFailed, velerov1.BackupPhasePartiallyFailed:
		return events.TypeBackupFailed
	}
	return ""
}

func terminalRestore(p velerov1.RestorePhase) string {
	switch p {
	case velerov1.RestorePhaseCompleted:
		return events.TypeRestoreCompleted
	case velerov1.RestorePhaseFailed, velerov1.RestorePhasePartiallyFailed:
		return events.TypeRestoreFailed
	}
	return ""
}

// Run starts the watcher loop (blocks until ctx done). Wire from server.go in a
// goroutine like the other background runners.
func Run(ctx context.Context, cl client.Client) {
	RunInterval(ctx, cl, defaultInterval)
}

func RunInterval(ctx context.Context, cl client.Client, interval time.Duration) {
	last := map[string]string{}
	seeded := false
	step := func() {
		items := collect(ctx, cl)
		for _, e := range diffPhases(last, seeded, items) {
			events.Publish(e)
		}
		seeded = true
	}
	step() // seed (no emits)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			step()
		}
	}
}

// collect lists backups+restores and normalizes to phaseItems.
func collect(ctx context.Context, cl client.Client) []phaseItem {
	ns := velerons.Namespace()
	var items []phaseItem
	bl := &velerov1.BackupList{}
	if err := cl.List(ctx, bl, client.InNamespace(ns)); err == nil {
		for i := range bl.Items {
			b := &bl.Items[i]
			items = append(items, phaseItem{
				key: "backup/" + b.Namespace + "/" + b.Name, subject: b.Name,
				phase: string(b.Status.Phase), terminalType: terminalBackup(b.Status.Phase),
			})
		}
	}
	rl := &velerov1.RestoreList{}
	if err := cl.List(ctx, rl, client.InNamespace(ns)); err == nil {
		for i := range rl.Items {
			r := &rl.Items[i]
			items = append(items, phaseItem{
				key: "restore/" + r.Namespace + "/" + r.Name, subject: r.Name,
				phase: string(r.Status.Phase), terminalType: terminalRestore(r.Status.Phase),
			})
		}
	}
	return items
}
