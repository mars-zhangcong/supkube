// Package velerons is the single source of truth for the namespace that
// Velero (server, node-agent, BackupStorageLocation, VolumeSnapshotLocation,
// cloud-credentials Secret, Backup/Restore CRs) is deployed into.
//
// History: the namespace used to be hardcoded as the literal "velero" in ~80
// call sites across the backend, while the Helm chart bundled Velero as a
// subchart that lands in the *release* namespace (supkube) — the velero
// subchart (chart 12.0.1) ignores `namespaceOverride`. The result was a split
// where the Velero controller ran in `supkube` but the backend hunted BSLs in
// `velero`, so every BSL the UI created became an orphan and stayed Unknown.
//
// This package resolves the namespace ONCE at process start from the
// VELERO_NAMESPACE env var (injected by the Helm chart: config.veleroNamespace
// → ConfigMap → backend Deployment), falling back to "velero" for dev /
// standalone runs and tests. Every call site reads velerons.Namespace()
// instead of a literal, so SupKube and Velero are guaranteed to agree.
package velerons

import (
	"os"
	"strings"
)

// Default is the fallback namespace when VELERO_NAMESPACE is unset (dev,
// standalone binaries, unit tests).
const Default = "velero"

// namespace is resolved once at package load. The Velero namespace cannot
// change at runtime, so reading the env exactly once is both correct and
// cheaper than os.Getenv on every Kubernetes call.
var namespace = resolve()

func resolve() string {
	if v := strings.TrimSpace(os.Getenv("VELERO_NAMESPACE")); v != "" {
		return v
	}
	return Default
}

// Namespace returns the configured Velero namespace.
func Namespace() string { return namespace }
