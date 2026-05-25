package v1

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supkube/supkube-backend/internal/auth"
	"github.com/supkube/supkube-backend/internal/k8s"
)

// GetSchedule (v0.8.9) returns a Policy (one or two Schedules aggregated).
//
// v0.8.9 change: returns PolicyAggregate, not raw Schedule. Old clients
// reading `data.metadata.name` / `data.spec.*` should switch to
// `data.snapshotSchedule.metadata.name` etc. The legacy shape can be
// retrieved by client.snapshotSchedule (always present).
func GetSchedule(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	policy, err := findPolicy(c.Request.Context(), cl, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// PatchSchedule updates an existing Velero Schedule.
//
// v0.8.5 step 5: this used to accept only `paused` (the toggle from the
// row menu). It now accepts the full editable subset so the Policy "Edit"
// form can write back. We intentionally DO NOT support every Velero spec
// field — the SupKube editor is a high-level Kasten-like form, not a YAML
// fork. The omitted fields keep their original values from the live CR.
//
// Editable fields (all optional; omit to leave unchanged):
//   paused                     bool              — quick pause/resume
//   schedule                   string (cron)     — new cron expression
//   ttl                        string (24h fmt)  — Backup retention
//   includedNamespaces         []string          — change ns scope (RBAC-checked against new + old)
//   storageLocation            string            — BSL name
//   snapshotVolumes            *bool             — CSI snapshot toggle
//   defaultVolumesToFsBackup   *bool             — filesystem backup toggle
//   annotations                map[string]string — supkube.io/* intent overlay; merged in
//
// We deliberately reject name changes; renaming a Schedule in Velero
// requires creating a new one (the Backup history is keyed by ref name).
func PatchSchedule(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Paused                   *bool             `json:"paused,omitempty"`
		Schedule                 *string           `json:"schedule,omitempty"`
		IncludedNamespaces       *[]string         `json:"includedNamespaces,omitempty"`
		SnapshotVolumes          *bool             `json:"snapshotVolumes,omitempty"`
		DefaultVolumesToFsBackup *bool             `json:"defaultVolumesToFsBackup,omitempty"`
		Annotations              map[string]string `json:"annotations,omitempty"`

		// Legacy single-schedule patch fields (still accepted for L1
		// and legacy policies; applied to the snapshot half).
		TTL              *string `json:"ttl,omitempty"`
		StorageLocation  *string `json:"storageLocation,omitempty"`
		SnapshotMoveData *bool   `json:"snapshotMoveData,omitempty"`

		// v0.8.9: per-role TTL / BSL for dual-mode policies. Patched
		// into the matching half if it exists; ignored otherwise.
		SnapshotTTL             *string `json:"snapshotTtl,omitempty"`
		SnapshotStorageLocation *string `json:"snapshotStorageLocation,omitempty"`
		ExportTTL               *string `json:"exportTtl,omitempty"`
		ExportStorageLocation   *string `json:"exportStorageLocation,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	policy, err := findPolicy(c.Request.Context(), cl, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// v0.8.5 step 3 RBAC: editor must scope-cover BOTH the policy's
	// CURRENT ns set AND any new ns set they're moving to.
	if !auth.RequireNamespaceAccess(c, policy.SnapshotSchedule.Spec.Template.IncludedNamespaces...) {
		return
	}
	if req.IncludedNamespaces != nil && !auth.RequireNamespaceAccess(c, (*req.IncludedNamespaces)...) {
		return
	}

	// Apply common fields + role-specific TTL/BSL to BOTH halves
	// (when present). The helper centralizes the merge so dual + single
	// modes go through the same code path.
	apply := func(s *velerov1.Schedule, role string) error {
		if req.Paused != nil {
			s.Spec.Paused = *req.Paused
		}
		if req.Schedule != nil {
			s.Spec.Schedule = *req.Schedule
		}
		tmpl := &s.Spec.Template
		if req.IncludedNamespaces != nil {
			tmpl.IncludedNamespaces = *req.IncludedNamespaces
		}
		if req.SnapshotVolumes != nil {
			tmpl.SnapshotVolumes = req.SnapshotVolumes
		}
		if req.DefaultVolumesToFsBackup != nil {
			tmpl.DefaultVolumesToFsBackup = req.DefaultVolumesToFsBackup
		}
		// Role-specific TTL / BSL win over legacy single-field input
		// when both are present (a user editing in the new UI will
		// only send the role-specific fields).
		ttl := req.TTL
		bsl := req.StorageLocation
		if role == roleSnapshot && req.SnapshotTTL != nil {
			ttl = req.SnapshotTTL
		}
		if role == roleSnapshot && req.SnapshotStorageLocation != nil {
			bsl = req.SnapshotStorageLocation
		}
		if role == roleExport && req.ExportTTL != nil {
			ttl = req.ExportTTL
		}
		if role == roleExport && req.ExportStorageLocation != nil {
			bsl = req.ExportStorageLocation
		}
		if ttl != nil {
			d, err := time.ParseDuration(*ttl)
			if err != nil {
				return fmt.Errorf("invalid ttl %q: %w", *ttl, err)
			}
			tmpl.TTL = metav1.Duration{Duration: d}
		}
		if bsl != nil {
			tmpl.StorageLocation = *bsl
		}
		// snapshotMoveData on each half is determined by role; allow
		// override only on legacy single-mode policies where role is
		// effectively snapshot but the user might be flipping flags.
		if req.SnapshotMoveData != nil && policy.Mode == "single" && policy.ExportSchedule == nil {
			tmpl.SnapshotMoveData = req.SnapshotMoveData
		} else if role == roleExport {
			truth := true
			tmpl.SnapshotMoveData = &truth
		} else if role == roleSnapshot {
			lie := false
			tmpl.SnapshotMoveData = &lie
		}
		// Merge annotations
		if len(req.Annotations) > 0 {
			if s.Annotations == nil {
				s.Annotations = map[string]string{}
			}
			for k, v := range req.Annotations {
				s.Annotations[k] = v
			}
		}
		return nil
	}

	// Snapshot half (always present)
	role := roleSnapshot
	if policy.SnapshotSchedule.Labels[labelPolicyRole] != "" {
		role = policy.SnapshotSchedule.Labels[labelPolicyRole]
	}
	if err := apply(policy.SnapshotSchedule, role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := cl.Update(c.Request.Context(), policy.SnapshotSchedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "snapshot half: " + err.Error()})
		return
	}
	// Export half (only in dual mode)
	if policy.ExportSchedule != nil {
		if err := apply(policy.ExportSchedule, roleExport); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := cl.Update(c.Request.Context(), policy.ExportSchedule); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "export half: " + err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, policy)
}

// RunScheduleOnce triggers an ad-hoc backup.
//
// v0.8.10 Plan-B: for a DUAL policy, we ONLY fire the snapshot half.
// The internal/policypair/ controller watches snapshot-half completion
// and fires the export half automatically. This shrinks the data gap
// between the two halves from minutes (the v0.8.9 "fire both at once
// and hope Velero schedules them close" approach) to ~30 seconds, and
// guarantees the export half doesn't kick off if the snapshot half
// fails (which previously left orphan partial exports on BSL).
//
// For L1 snapshot-only policies: same as before — fire the snapshot
// half, controller is a no-op (no peer export Schedule exists).
//
// For L1 export-only policies (no snapshot half, just export Schedule):
// we fire the export half directly. There's no snapshot to wait on, so
// Plan-B's serialization wouldn't help anyway. This is a legacy code
// path; new L1 policies should default to snapshot-only.
//
// Backup naming: <schedule-half-name>-<timestamp>, identical scheme to
// Velero's own scheduled runs. The controller-fired export half reuses
// the snapshot half's 14-digit timestamp suffix (see policypair package
// `suffixFromSnapshotName`) so paired Backups stay alphabetically and
// chronologically adjacent in any UI sort.
func RunScheduleOnce(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	policy, err := findPolicy(c.Request.Context(), cl, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// Editor RBAC: scope-cover the policy's ns set.
	if !auth.RequireNamespaceAccess(c, policy.SnapshotSchedule.Spec.Template.IncludedNamespaces...) {
		return
	}

	// Shared timestamp so paired Backups group together in UI.
	ts := time.Now().UTC().Format("20060102150405")

	// Helper to materialize one Backup CR from one half.
	createBackup := func(s *velerov1.Schedule) (string, error) {
		backupName := fmt.Sprintf("%s-%s", s.Name, ts)
		labels := map[string]string{
			"velero.io/schedule-name": s.Name,
			"supkube.io/run-trigger":  "manual",
		}
		// Inherit policy-name + role labels from the parent Schedule
		// so the Backup carries the same grouping info. The Velero
		// schedule-controller automatically copies these for scheduled
		// runs (v1.15+); we mirror that for manual runs.
		for k, v := range s.Labels {
			labels[k] = v
		}
		backup := &velerov1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backupName,
				Namespace: "velero",
				Labels:    labels,
				Annotations: map[string]string{
					"supkube.io/triggered-by": "policy-run-once",
				},
			},
			Spec: *s.Spec.Template.DeepCopy(),
		}
		if err := cl.Create(c.Request.Context(), backup); err != nil {
			return "", err
		}
		return backupName, nil
	}

	created := []string{}
	// Plan-B branch: for a dual policy, fire ONLY the snapshot half here.
	// The policypair controller picks up the snapshot's Completed event
	// and triggers the export half within ~10 seconds. The user gets a
	// single response back fast, and the export RP appears in the UI
	// once it materializes (the Activity / Restore Points pages already
	// poll on their own).
	if policy.SnapshotSchedule != nil {
		snapName, err := createBackup(policy.SnapshotSchedule)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "snapshot half: " + err.Error()})
			return
		}
		created = append(created, snapName)
	} else if policy.ExportSchedule != nil {
		// L1 export-only policy (no snapshot half). Fire export directly.
		// No Plan-B serialization applies because there's no snapshot to
		// wait on.
		expName, err := createBackup(policy.ExportSchedule)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "export-only policy: " + err.Error()})
			return
		}
		created = append(created, expName)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "policy has neither snapshot nor export half"})
		return
	}

	resp := gin.H{
		"policyName": policy.PolicyName,
		"backups":    created,
	}
	// Tell the SPA whether to expect a paired export RP showing up
	// soon. Helps the Activity page show a "waiting for export half"
	// placeholder card during the ~30s gap.
	if policy.SnapshotSchedule != nil && policy.ExportSchedule != nil {
		resp["pendingExportHalf"] = true
		resp["pairNote"] = "export half will be triggered automatically by the SupKube pair controller after the snapshot half completes (~30s)"
	}
	c.JSON(http.StatusCreated, resp)
}
