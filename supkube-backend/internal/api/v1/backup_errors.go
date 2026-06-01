// Package v1: GET /api/v1/backups/:name/errors (v0.8.8.1).
//
// Why this exists
// ───────────────
// A Velero Backup CR carries an `errors: N` count in status, but the
// actual error messages are scattered across:
//
//  1. status.message            — Velero's own top-level error (rare)
//  2. DataUpload CRs            — Data Mover failures (most common)
//  3. PodVolumeBackup CRs       — Filesystem backup failures
//  4. <backup>-results.gz in BSL — Velero-emitted per-namespace errors
//     (requires DownloadRequest dance)
//
// Until v0.8.8 the SupKube UI's Activity Action Details panel showed
// "Health: 2 errors" with NO way to see what those errors actually were
// for Backups. The Restore path was fixed in v0.7.11 (DownloadRequest
// for RestoreResults); the Backup path was a known TODO that got
// forgotten. This silent-error pattern is exactly what 工程座右铭 §11.3
// warns against — see ADR-024 + 架构设计.md §11.3.
//
// This endpoint MVP covers sources (2) and (3) because Data Mover
// errors are by far the most common (and what triggered the v0.8.8.1
// bug fix). Source (4) — Velero `<backup>-results.gz` in BSL — needs
// the DownloadRequest dance similar to GetRestoreResults; deferred to
// v0.9 because in practice DataUpload + PVB messages cover 95% of cases.
package v1

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// BackupErrorEntry is one error / failure message attributable to a
// specific resource (DataUpload or PodVolumeBackup). Grouped by namespace
// on the frontend the same way Restore errors are.
type BackupErrorEntry struct {
	// Source is the CR kind ("DataUpload" or "PodVolumeBackup") so the
	// UI can mark which subsystem produced the error.
	Source string `json:"source"`
	// SourcePVC is the protected PVC (when known). Lets the user see
	// "which volume failed" without cross-referencing.
	SourcePVC string `json:"sourcePvc,omitempty"`
	// Namespace of the source PVC. Used for UI grouping.
	Namespace string `json:"namespace,omitempty"`
	// Name of the failing CR (e.g. azure-mover-abc123-fc4b9) — useful
	// for the user to grep velero logs / kubectl describe directly.
	Name string `json:"name"`
	// Message is the actual failure text. May be long; the UI uses
	// a <pre> block with overflow so multi-line errors render readably.
	Message string `json:"message"`
}

// BackupErrorsResponse is what GET /backups/:name/errors returns.
type BackupErrorsResponse struct {
	BackupName string `json:"backupName"`
	// Errors lists every failed DataUpload + PodVolumeBackup tied to
	// this backup. Empty array (not null) when nothing failed.
	Errors []BackupErrorEntry `json:"errors"`
	// Warnings reserved for v0.9 BSL <backup>-results.gz integration.
	Warnings []BackupErrorEntry `json:"warnings"`
	// FetchError surfaces a single top-level error if the cluster query
	// itself failed. Frontend shows this above the empty list so users
	// don't see "no errors" when really we couldn't check.
	FetchError string `json:"fetchError,omitempty"`
}

// GetBackupErrors returns aggregated error messages for a Backup.
// Public route — RBAC table treats it as viewer-readable (same as
// GET /backups/:name).
func GetBackupErrors(c *gin.Context) {
	name := c.Param("name")
	resp := BackupErrorsResponse{
		BackupName: name,
		Errors:     []BackupErrorEntry{},
		Warnings:   []BackupErrorEntry{},
	}

	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		resp.FetchError = "runtime client: " + err.Error()
		c.JSON(http.StatusOK, resp)
		return
	}

	// Confirm the Backup exists — otherwise return 404 so the UI shows
	// "Backup not found" instead of "no errors".
	bk := &velerov1.Backup{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, bk); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// ── DataUpload errors (Data Mover path) ───────────────────────
	// status.message is set by node-agent on failure; phase=Failed
	// alone is enough to consider it an error, but we surface the
	// message verbatim for the user. Empty message + Failed phase
	// is rare but possible (node-agent crash) — we still emit so the
	// user sees "this DataUpload failed" with no detail rather than
	// nothing.
	duList := &velerov2alpha1.DataUploadList{}
	if err := cl.List(context.Background(), duList, client.InNamespace("velero"), client.MatchingLabels{"velero.io/backup-name": name}); err == nil {
		for _, du := range duList.Items {
			if du.Status.Phase != velerov2alpha1.DataUploadPhaseFailed && du.Status.Phase != velerov2alpha1.DataUploadPhaseCanceled {
				continue
			}
			msg := du.Status.Message
			if msg == "" {
				msg = "(empty message; phase=" + string(du.Status.Phase) + ")"
			}
			resp.Errors = append(resp.Errors, BackupErrorEntry{
				Source:    "DataUpload",
				SourcePVC: du.Spec.SourcePVC,       // schema: plain string, not *.Name
				Namespace: du.Spec.SourceNamespace, // both are plain strings
				Name:      du.Name,
				Message:   msg,
			})
		}
	} else {
		// Don't fail the whole response — partial data is better than
		// no data. Append to FetchError so UI can show "couldn't read
		// DataUploads" beside whatever PVB errors we DID get.
		appendFetchErr(&resp, "list DataUploads: "+err.Error())
	}

	// ── PodVolumeBackup errors (Filesystem backup path) ───────────
	pvbList := &velerov1.PodVolumeBackupList{}
	if err := cl.List(context.Background(), pvbList, client.InNamespace("velero"), client.MatchingLabels{"velero.io/backup-name": name}); err == nil {
		for _, pvb := range pvbList.Items {
			if pvb.Status.Phase != velerov1.PodVolumeBackupPhaseFailed {
				continue
			}
			msg := pvb.Status.Message
			if msg == "" {
				msg = "(empty message; phase=" + string(pvb.Status.Phase) + ")"
			}
			resp.Errors = append(resp.Errors, BackupErrorEntry{
				Source:    "PodVolumeBackup",
				SourcePVC: pvb.Spec.Volume,
				Namespace: pvb.Namespace,
				Name:      pvb.Name,
				Message:   msg,
			})
		}
	} else {
		appendFetchErr(&resp, "list PodVolumeBackups: "+err.Error())
	}

	// Hint when the count from Backup.status disagrees with what we
	// found. If Velero reports errors=2 but we found 0 messages, the
	// errors are likely in <backup>-results.gz (BSL) — surface a
	// breadcrumb pointing v0.9 work, NOT silent zero.
	if bk.Status.Errors > 0 && len(resp.Errors) == 0 {
		appendFetchErr(&resp, "Backup.status reports errors but no DataUpload/PVB failed; the messages likely live in BSL <backup>-results.gz (v0.9 will fetch these via DownloadRequest, similar to RestoreResults).")
	}

	c.JSON(http.StatusOK, resp)
}

// appendFetchErr concatenates errors so we never lose a message but
// also don't crash if multiple sources fail in one request.
func appendFetchErr(r *BackupErrorsResponse, s string) {
	if r.FetchError != "" {
		r.FetchError += " | "
	}
	r.FetchError += s
}
