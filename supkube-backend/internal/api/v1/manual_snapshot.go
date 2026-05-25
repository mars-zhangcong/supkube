// Package v1: one-click manual Application snapshot (v0.8.9.2).
//
// Why this exists as a separate endpoint vs. reusing CreateBackup
// ──────────────────────────────────────────────────────────────
// CreateBackup is the generic, parameterized "create any Backup" verb —
// it takes 10+ optional fields, validates ns scope, supports all 4 data
// paths, etc. Power tool.
//
// The Applications page "Snapshot" button is a totally different
// product surface: ONE click, ZERO config, the user just wants
// "right now, this namespace, take a cluster-local Restore Point I can
// use to revert if something goes wrong". Reusing CreateBackup means
// the frontend has to encode policy + safety defaults, which then
// diverges from the backend's intent and we get the v0.7-style "Custom
// silently means hourly" footguns over time.
//
// So we make this a first-class verb with hard-coded sensible defaults:
//
//   - snapshotVolumes=true, snapshotMoveData=false, fsBackup=false
//     → pure cluster-local CSI snapshot (matches the Applications page's
//       UX promise: "instant rollback point", not "DR backup")
//   - TTL=24h (configurable via body, but caller usually doesn't set it)
//   - StorageLocation=default (any BSL — only metadata tarball goes
//     there; PV data is in CSI snapshot)
//   - Labels distinguish these from policy-driven backups so the UI's
//     Restore Points page can show "(manual)" in the Policy column AND
//     filter them out if the user wants only scheduled RPs
//
// Audit log produces "user X took manual snapshot of ns Y" which is the
// clear, narrow audit story we want for compliance — not "user X called
// generic CreateBackup with arg matrix Z".
package v1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supkube/supkube-backend/internal/auth"
	"github.com/supkube/supkube-backend/internal/k8s"
)

// Label keys used for manual snapshots. Kept here next to the endpoint
// so the UI's filtering code (which reads the same labels) has a single
// source of truth to match.
const (
	// LabelManualSnapshot=true on a Backup means it was created by the
	// Applications-page "Snapshot" button, NOT by a Policy / Schedule.
	// Used by the Restore Points page to show "(manual)" in the Policy
	// column and as a filter chip.
	LabelManualSnapshot = "supkube.io/manual-snapshot"

	// LabelSourceNamespace records the protected namespace. Velero's
	// own labels don't include this directly (it's in spec); having
	// it as a label lets us do fast list-by-label queries from the UI
	// (e.g. "all manual snapshots of ns=test-app").
	LabelSourceNamespace = "supkube.io/source-namespace"

	// AnnotationTriggeredBy is human-readable: shows up in audit log
	// + Activity detail. Different value for manual vs run-once vs
	// scheduled so the audit reader can tell the trigger apart.
	AnnotationTriggeredBy = "supkube.io/triggered-by"

	// AnnotationCreatedByUser is the username (or email) of whoever
	// pushed the Snapshot button. Populated from the auth context.
	// Compliance: "who took this snapshot at this time" trail.
	AnnotationCreatedByUser = "supkube.io/created-by-user"
)

// CreateManualSnapshot is the one-click endpoint behind the
// Applications-page Snapshot button.
//
// Request body (all optional):
//
//	{"ttl":"24h", "storageLocation":"default", "comment":"pre-deploy snapshot"}
//
// The namespace comes from the URL path, NOT the body — this keeps the
// audit narrative crisp ("user took snapshot of <ns from URL>") and
// avoids body-vs-URL mismatch bugs.
func CreateManualSnapshot(c *gin.Context) {
	namespace := c.Param("namespace")
	if namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace is required in URL path"})
		return
	}
	// Protect well-known infra namespaces from accidental snapshot
	// (the Restore drawer already rejects RESTORING into these; here
	// we reject TAKING a snapshot of them since it's almost always a
	// user error — they don't actually want to snapshot kube-system).
	if isProtectedNamespace(namespace) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("refusing to snapshot protected namespace %q (kube-system, velero, supkube etc. are excluded)", namespace),
		})
		return
	}

	// RBAC: editor needs scope-coverage on this ns; admin always OK.
	if !auth.RequireNamespaceAccess(c, namespace) {
		return
	}

	var req struct {
		// Optional retention override. Default 24h matches the
		// "ephemeral local snapshot" product positioning.
		TTL string `json:"ttl,omitempty"`
		// Optional BSL override; defaults to cluster default.
		StorageLocation string `json:"storageLocation,omitempty"`
		// Optional free-text note carried into annotation so the
		// user can later remember WHY they took this snapshot
		// (e.g. "before risky deployment").
		Comment string `json:"comment,omitempty"`
	}
	// ShouldBindJSON tolerates empty body — caller can POST with no
	// body at all and get all defaults.
	_ = c.ShouldBindJSON(&req)

	// Defaults
	ttl := req.TTL
	if ttl == "" {
		ttl = "24h"
	}
	ttlDur, err := time.ParseDuration(ttl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ttl: " + err.Error()})
		return
	}
	bsl := req.StorageLocation
	if bsl == "" {
		bsl = "default"
	}

	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Backup name: <ns>-manual-<YYYYMMDDhhmmss>. Distinct from policy
	// names (<policy>-<ts>) so users + audit can tell at a glance.
	backupName := fmt.Sprintf("%s-manual-%s", namespace, time.Now().UTC().Format("20060102150405"))

	// Capture username for the audit annotation. FromContext is safe
	// to call even when auth is off (returns anonymous user).
	username := "anonymous"
	if u := auth.FromContext(c); u != nil && u.Username != "" {
		username = u.Username
	}

	annotations := map[string]string{
		AnnotationTriggeredBy:   "app-manual-snapshot",
		AnnotationCreatedByUser: username,
	}
	if req.Comment != "" {
		annotations["supkube.io/comment"] = req.Comment
	}

	// Hard-coded snapshot semantics: cluster-local CSI snapshot.
	// We DO NOT expose a Data Mover toggle on this endpoint — that's
	// what policies are for. Keeping the verb narrow is half the
	// reason this endpoint exists (see file header).
	csiOn := true
	fsOff := false
	moveOff := false

	backup := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: "velero",
			Labels: map[string]string{
				LabelManualSnapshot:  "true",
				LabelSourceNamespace: namespace,
			},
			Annotations: annotations,
		},
		Spec: velerov1.BackupSpec{
			IncludedNamespaces:       []string{namespace},
			SnapshotVolumes:          &csiOn,
			DefaultVolumesToFsBackup: &fsOff,
			SnapshotMoveData:         &moveOff,
			TTL:                      metav1.Duration{Duration: ttlDur},
			StorageLocation:          bsl,
		},
	}
	if err := cl.Create(context.Background(), backup); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create snapshot: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"backupName":  backupName,
		"namespace":   namespace,
		"triggeredBy": "app-manual-snapshot",
		"user":        username,
		"ttl":         ttlDur.String(),
		// Hint to the SPA: where the user can watch progress.
		"watchUrl":    "/activity?type=Backup&filter=" + backupName,
	})
}
