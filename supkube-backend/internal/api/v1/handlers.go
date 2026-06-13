package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/auth"
	"github.com/supkube/supkube-backend/internal/gc"
	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/velerons"
	"github.com/supkube/supkube-backend/internal/version"
)

// GetStatus returns system status. Public endpoint (bypasses auth) so
// scripts can confirm what backend version is actually running before
// they touch credentials. Use `curl -s http://supkube/api/v1/status` —
// the build stamp tells you whether a recent helm upgrade actually
// rolled out.
func GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"version":    version.Version,
		"buildStamp": version.BuildStamp,
	})
}

// CreateNamespace creates a new namespace. Used by the Restore drawer's
// "Create A New Namespace" flow so the user can restore into a fresh target
// without bouncing out to kubectl. Refuses to (re)create protected namespaces.
func CreateNamespace(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateK8sName(req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if isProtectedNamespace(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("namespace %q is protected and cannot be created via SupKube", req.Name)})
		return
	}
	cl, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: req.Name,
			Labels: map[string]string{
				"supkube.io/managed-by": "supkube",
				"supkube.io/created-by": "restore-drawer",
			},
		},
	}
	if _, err := cl.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"name": req.Name})
}

// isProtectedNamespace lists namespaces SupKube refuses to delete or
// reactively recreate. velero/kube-system/etc. are platform-critical and
// supkube hosts the controller itself.
func isProtectedNamespace(name string) bool {
	switch name {
	case "velero", "supkube", "kube-system", "kube-public", "kube-node-lease", "default":
		return true
	}
	return false
}

// ListNamespaces returns all namespaces in the cluster
func ListNamespaces(c *gin.Context) {
	cl, err := getRequestKubernetesClient(c) // v0.9.0.1 — header-routed
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	nsList, err := cl.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	names := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		names = append(names, ns.Name)
	}
	c.JSON(http.StatusOK, gin.H{"items": names, "total": len(names)})
}

// ListBackups returns all Velero backups, each enriched with a `supkube`
// sidecar containing dataPath / volume metadata / tarball size.
//
// v0.8.6: the enrichment is the "Backup Composition" answer to the
// recurring "what's actually in this backup?" question. Pre-v0.8.6 the
// list response was just the raw velerov1.Backup slice, which Velero
// schema doesn't expose any of these fields on. See backup_meta.go +
// backup_meta_bsl.go for the implementation.
//
// Volume metadata + tarball sizes are computed in BATCH (one cluster
// query, one S3 ListObjectsV2) and then joined in memory, so a page of
// 100 backups still costs only a few external calls.
func ListBackups(c *gin.Context) {
	// v0.9.0.1 fix #3 — route to remote cluster when X-Supkube-Cluster
	// header is set by the SPA's Mode Switcher. Same wire format; same
	// types; just a different rest.Config underneath.
	cl, err := getRequestRuntimeClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	backupList := &velerov1.BackupList{}
	namespace := c.DefaultQuery("namespace", velerons.Namespace())
	phase := c.Query("phase")
	if err := cl.List(context.Background(), backupList, client.InNamespace(namespace)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Batch-fetch volume metadata for the whole list (1 VSC list call,
	// 1 PVB list call). Avoid N round-trips per page row.
	volMeta := collectVolumeMetadataBatch(c.Request.Context(), cl)
	// v0.8.10.4: batch reserved-bytes (one PVC list per unique included
	// namespace) so Snapshot RPs whose VSCs got auto-deleted by Velero
	// v1.18 still show a meaningful Size cell.
	reservedMeta := reservedBytesBatch(c.Request.Context(), cl, backupList.Items)
	// v0.8.11 #24: stable Application Items count per backup. One pass
	// over the unique namespace set; same categorisation as the drawer's
	// artifact-breakdown.
	appItemsMeta := applicationItemsBatch(c.Request.Context(), backupList.Items)
	// v0.9.1.5: one Schedule list for the whole page → per-row imported
	// detection is a map lookup. Drives the "Imported" chip on Restore
	// Points + the whole-tarball restore UX in the Restore drawer (C-001).
	localSchedules := localScheduleSet(c.Request.Context(), cl)

	items := make([]map[string]interface{}, 0, len(backupList.Items))
	for i := range backupList.Items {
		b := &backupList.Items[i]
		if phase != "" && string(b.Status.Phase) != phase {
			continue
		}
		meta := SupKubeBackupMeta{
			DataPath: classifyDataPath(b),
		}
		if v, ok := volMeta[b.Name]; ok {
			meta.VolumeCount = v.Count
			meta.VolumeBytes = v.Bytes
		}
		meta.ReservedBytes = reservedMeta[b.Name]
		meta.ApplicationItems = appItemsMeta[b.Name]
		meta.Imported = detectImportedBackup(b, localSchedules)
		meta.Origin = detectBackupOrigin(b, localSchedules)
		// Tarball size from BSL is cached per-BSL with 60s TTL so a
		// page render is O(BSL count) external calls, not O(backup count).
		size, errStr := getBackupTarballSize(c.Request.Context(), cl, b)
		meta.TarballBytes = size
		meta.TarballError = errStr

		items = append(items, enrichBackupForResponse(b, meta))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// enrichBackupForResponse takes a velerov1.Backup and our SupKube
// sidecar, and produces a JSON-friendly map that embeds the original
// Backup fields verbatim + adds a top-level "supkube" key.
//
// We marshal-then-unmarshal once to flatten the Backup into a map.
// The cost is real (small JSON round-trip per row) but it keeps the
// existing client-side `row.metadata.name`, `row.status.phase`, etc.
// references working with zero changes — that's worth ~2 µs per row.
func enrichBackupForResponse(b *velerov1.Backup, meta SupKubeBackupMeta) map[string]interface{} {
	raw, err := json.Marshal(b)
	if err != nil {
		// Should never happen for a typed Velero struct; fall back to
		// a minimal shape so the row still renders.
		return map[string]interface{}{
			"metadata": map[string]interface{}{"name": b.Name},
			"supkube":  meta,
		}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]interface{}{
			"metadata": map[string]interface{}{"name": b.Name},
			"supkube":  meta,
		}
	}
	out["supkube"] = meta
	return out
}

// CreateBackup creates a new Velero backup.
// v0.6 dual-mode volume backup:
//   - SnapshotVolumes=true        → CSI snapshot path (Velero v1.14+ core)
//   - DefaultVolumesToFsBackup=true → Restic/Kopia filesystem backup path
//
// UI sends exactly one of these as true; either OR neither (skip volumes) is
// valid. Both true is rejected as ambiguous.
func CreateBackup(c *gin.Context) {
	var req struct {
		Name                     string            `json:"name" binding:"required"`
		IncludedNamespaces       []string          `json:"includedNamespaces"`
		ExcludedNamespaces       []string          `json:"excludedNamespaces"`
		TTL                      string            `json:"ttl"`
		LabelSelector            map[string]string `json:"labelSelector"`
		StorageLocation          string            `json:"storageLocation"`
		SnapshotVolumes          *bool             `json:"snapshotVolumes"`
		DefaultVolumesToFsBackup *bool             `json:"defaultVolumesToFsBackup"`
		// v0.8.7: Data Mover. Layered on top of CSI — when true, Velero
		// takes the CSI snapshot and Kopia-uploads its contents to the
		// BSL, making the backup portable across clusters. Requires
		// node-agent DaemonSet to be deployed in the velero namespace.
		SnapshotMoveData *bool `json:"snapshotMoveData"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.SnapshotVolumes != nil && *req.SnapshotVolumes &&
		req.DefaultVolumesToFsBackup != nil && *req.DefaultVolumesToFsBackup {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ambiguous volume mode: pick exactly one of snapshotVolumes (CSI) or defaultVolumesToFsBackup (filesystem)"})
		return
	}
	// Data Mover only makes sense layered on CSI — it's "take the CSI
	// snapshot then upload it". Filesystem backup already uploads,
	// so combining them is a config smell. Reject early with the user-
	// visible reason rather than letting Velero silently ignore one.
	if req.SnapshotMoveData != nil && *req.SnapshotMoveData {
		csiOn := req.SnapshotVolumes != nil && *req.SnapshotVolumes
		if !csiOn {
			c.JSON(http.StatusBadRequest, gin.H{"error": "snapshotMoveData requires snapshotVolumes=true (Data Mover is layered on CSI)"})
			return
		}
		if req.DefaultVolumesToFsBackup != nil && *req.DefaultVolumesToFsBackup {
			c.JSON(http.StatusBadRequest, gin.H{"error": "snapshotMoveData conflicts with defaultVolumesToFsBackup (both try to upload; pick one)"})
			return
		}
	}
	// v0.8.5 step 3: editor can only create Backups for their scoped ns.
	// Empty IncludedNamespaces = whole-cluster backup, which only admin
	// can do (editor must specify at least one ns within scope).
	if !auth.RequireNamespaceAccess(c, req.IncludedNamespaces...) {
		return
	}
	if len(req.IncludedNamespaces) == 0 {
		user := auth.FromContext(c)
		if user != nil && user.Role == auth.RoleEditor {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "editors must explicitly list includedNamespaces (whole-cluster backups require admin)",
			})
			return
		}
	}

	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	backup := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: velerons.Namespace(),
		},
		Spec: velerov1.BackupSpec{
			IncludedNamespaces:       req.IncludedNamespaces,
			ExcludedNamespaces:       req.ExcludedNamespaces,
			SnapshotVolumes:          req.SnapshotVolumes,
			DefaultVolumesToFsBackup: req.DefaultVolumesToFsBackup,
			SnapshotMoveData:         req.SnapshotMoveData,
		},
	}
	if req.TTL != "" {
		duration, parseErr := time.ParseDuration(req.TTL)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid TTL format: " + parseErr.Error()})
			return
		}
		backup.Spec.TTL = metav1.Duration{Duration: duration}
	}
	if len(req.LabelSelector) > 0 {
		backup.Spec.LabelSelector = &metav1.LabelSelector{MatchLabels: req.LabelSelector}
	}
	// v0.9.1.10 (#107): resolve the effective BackupStorageLocation.
	//
	// Demo failure (2026-05-28): the UI used to hardcode storageLocation:
	// "default", and an unspecified BSL also makes Velero fall back to a BSL
	// literally named "default". Our AKS dev clusters have only "azure-blob"
	// — no "default" — so the Backup failed validation with the opaque
	// "BackupStorageLocation \"default\" not found". We now:
	//   - empty            → auto-pick (default-flagged / sole / named-default)
	//   - specified        → validate it exists, else return an actionable 422
	//                         (with the list of real BSLs) instead of letting
	//                         Velero produce the cryptic failure later.
	if req.StorageLocation == "" {
		bslName, rerr := resolveBackupStorageLocation(context.Background(), cl)
		if rerr != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": rerr.Error()})
			return
		}
		backup.Spec.StorageLocation = bslName
	} else {
		if rerr := assertBSLExists(context.Background(), cl, req.StorageLocation); rerr != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": rerr.Error()})
			return
		}
		backup.Spec.StorageLocation = req.StorageLocation
	}
	if err := cl.Create(context.Background(), backup); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, backup)
}

// resolveBackupStorageLocation picks the effective BSL when the caller didn't
// name one. Velero's own implicit fallback is a BSL literally named "default"
// — but many clusters don't have one (our AKS dev clusters' sole BSL is
// "azure-blob"), so an unspecified-BSL backup fails validation. (#107.)
//
// Precedence:
//  1. a BSL with spec.default == true  (Velero's notion of "the default")
//  2. the sole BSL, when exactly one exists
//  3. a BSL literally named "default", if present
//  4. otherwise error — multiple BSLs, none default; the caller must choose.
func resolveBackupStorageLocation(ctx context.Context, cl client.Client) (string, error) {
	bslList := &velerov1.BackupStorageLocationList{}
	if err := cl.List(ctx, bslList, client.InNamespace(velerons.Namespace())); err != nil {
		return "", fmt.Errorf("listing BackupStorageLocations: %w", err)
	}
	return pickPreferredBSL(bslList.Items, "BackupStorageLocation")
}

// resolveExportStorageLocation picks a CLOUD (off-site) BSL for the export half
// of an L2 policy when the caller didn't name one. It mirrors
// resolveBackupStorageLocation but EXCLUDES the SupKube-managed in-cluster Local
// BSL (label supkube.io/bsl-role=local), so the export copy lands off-site —
// preserving the 3-2-1-1-0 "≥1 copy off-site" property. (#101 finding 1.)
func resolveExportStorageLocation(ctx context.Context, cl client.Client) (string, error) {
	bslList := &velerov1.BackupStorageLocationList{}
	if err := cl.List(ctx, bslList, client.InNamespace(velerons.Namespace())); err != nil {
		return "", fmt.Errorf("listing BackupStorageLocations: %w", err)
	}
	cloud := make([]velerov1.BackupStorageLocation, 0, len(bslList.Items))
	for i := range bslList.Items {
		if bslList.Items[i].Labels["supkube.io/bsl-role"] == "local" {
			continue // the in-cluster Local store is never an off-site target
		}
		cloud = append(cloud, bslList.Items[i])
	}
	return pickPreferredBSL(cloud, "cloud (off-site) BackupStorageLocation")
}

// pickPreferredBSL applies SupKube's "which BSL did the caller mean" precedence
// to a candidate set: (1) one flagged spec.default; (2) the sole candidate;
// (3) one literally named "default" (Velero's own implicit fallback); else an
// actionable error listing the candidates. `what` personalizes the messages.
func pickPreferredBSL(items []velerov1.BackupStorageLocation, what string) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no %s is configured — add one under Storage Locations first", what)
	}
	for i := range items {
		if items[i].Spec.Default {
			return items[i].Name, nil
		}
	}
	if len(items) == 1 {
		return items[0].Name, nil
	}
	for i := range items {
		if items[i].Name == "default" {
			return "default", nil
		}
	}
	names := make([]string, 0, len(items))
	for i := range items {
		names = append(names, items[i].Name)
	}
	return "", fmt.Errorf("multiple %ss exist and none is marked default; specify one explicitly (available: %s)", what, strings.Join(names, ", "))
}

// assertBSLExists turns a bad storageLocation name into an actionable 422 at
// create time, rather than the cryptic Velero validation failure minutes
// later. Lists once and matches by name. (#107.)
func assertBSLExists(ctx context.Context, cl client.Client, name string) error {
	bslList := &velerov1.BackupStorageLocationList{}
	if err := cl.List(ctx, bslList, client.InNamespace(velerons.Namespace())); err != nil {
		return fmt.Errorf("listing BackupStorageLocations: %w", err)
	}
	names := make([]string, 0, len(bslList.Items))
	for i := range bslList.Items {
		if bslList.Items[i].Name == name {
			return nil
		}
		names = append(names, bslList.Items[i].Name)
	}
	return fmt.Errorf("BackupStorageLocation %q not found (available: %s)", name, strings.Join(names, ", "))
}

// GetBackup returns a specific backup, enriched with the same `supkube`
// sidecar shape ListBackups produces.
//
// For single-backup queries we compute volume metadata directly (the
// batch helper is overkill for N=1) but we still go through the BSL
// cache for tarball size — repeated detail-page polls hit the same
// 60s-cached BSL listing.
func GetBackup(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	backup := &velerov1.Backup{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: velerons.Namespace()}, backup); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	volCount, volBytes := collectVolumeMetadata(c.Request.Context(), cl, name)
	tarballBytes, tarballErr := getBackupTarballSize(c.Request.Context(), cl, backup)
	reservedBytes := reservedBytesForBackup(c.Request.Context(), cl, backup)
	localSchedules := localScheduleSet(c.Request.Context(), cl)
	meta := SupKubeBackupMeta{
		DataPath:      classifyDataPath(backup),
		VolumeCount:   volCount,
		VolumeBytes:   volBytes,
		ReservedBytes: reservedBytes,
		TarballBytes:  tarballBytes,
		TarballError:  tarballErr,
		Imported:      detectImportedBackup(backup, localSchedules),
		Origin:        detectBackupOrigin(backup, localSchedules),
	}
	c.JSON(http.StatusOK, enrichBackupForResponse(backup, meta))
}

// DeleteBackup deletes a backup
// DeleteBackup performs a CASCADE delete: removes the backup tarball from
// object storage, deletes any CSI VolumeSnapshot/VolumeSnapshotContent the
// backup created, deletes PodVolumeBackups for the Restic/Kopia path, AND
// then removes the Backup CR itself.
//
// Previous SupKube versions (pre v0.7.9) just called cl.Delete(Backup) which
// only removed the K8s object — Velero's BackupSyncController would
// silently re-sync it back from the still-present BSL data 60s later, AND
// the underlying storage usage was never reclaimed. That was a real bug;
// users thought they were freeing space when they weren't.
//
// The right way is `DeleteBackupRequest` CRD. Velero's controller picks
// it up and performs the full cascade. We create one DBR per delete; the
// controller handles dedup/in-progress detection.
func DeleteBackup(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Confirm the Backup exists first so the API returns 404 (not 500)
	// when the user references something gone.
	backup := &velerov1.Backup{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: velerons.Namespace()}, backup); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// v0.8.5 step 3: editor can only delete Backups whose ns is in scope.
	// Whole-cluster backups (empty IncludedNamespaces) require admin.
	if len(backup.Spec.IncludedNamespaces) == 0 {
		user := auth.FromContext(c)
		if user != nil && user.Role == auth.RoleEditor {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "deleting a whole-cluster backup requires admin",
			})
			return
		}
	} else if !auth.RequireNamespaceAccess(c, backup.Spec.IncludedNamespaces...) {
		return
	}

	// DBR name: <backup>-<ts>. Velero auto-cleans DBRs after processing;
	// the unique suffix lets us re-trigger a delete if a prior one stalled.
	dbrName := fmt.Sprintf("%s-delete-%s", name, time.Now().UTC().Format("20060102150405"))
	dbr := &velerov1.DeleteBackupRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbrName,
			Namespace: velerons.Namespace(),
			Labels: map[string]string{
				"velero.io/backup-name": name,
				"supkube.io/managed-by": "supkube",
			},
		},
		Spec: velerov1.DeleteBackupRequestSpec{
			BackupName: name,
		},
	}
	if err := cl.Create(context.Background(), dbr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create DeleteBackupRequest: " + err.Error()})
		return
	}
	// v0.8.8: schedule a deferred orphan-GC scan ~60s from now. Velero's
	// DBR controller cleans most things but leaks Data Mover retained
	// VSC/VS. The debounced timer in gc.TriggerSoon coalesces rapid
	// deletes into a single scan, avoiding K8s API churn when a user
	// bulk-deletes 20 restore points in a row.
	k8sCli, kerr := k8s.GetClient()
	if kerr == nil {
		gc.TriggerSoon(context.Background(), cl, k8sCli)
	}
	c.JSON(http.StatusAccepted, gin.H{
		"message":             "Delete in progress (cascade); orphan cleanup scheduled ~60s",
		"deleteBackupRequest": dbrName,
		"backupName":          name,
	})
}

// ForceDeleteBackup is the emergency-escape hatch for a Backup CR that's
// stuck — either because Velero's controller is hung, the DBR cascade
// stalled, or a finalizer is wedged from a half-done previous delete.
//
// It does what `kubectl patch ... finalizers=null && kubectl delete` does
// manually, but: (a) keeps RBAC scoping, (b) records what was bypassed in
// audit log, (c) schedules orphan GC for the storage-snapshot side
// (otherwise force-delete leaks cloud-provider snapshots).
//
// USE WITH CARE: this does NOT trigger DBR cascade. The backup tarball on
// the BSL is left behind (Velero's BackupSyncController will re-sync it
// back from BSL in 60s unless the object was already removed). For a
// CLEAN cascading delete, use DELETE /backups/:name. Use this only when
// that has stalled. The response tells the user what was bypassed.
//
// v0.9.x: introduced after repeated demo pain — a Backup CR with a
// node-agent-related finalizer was un-deletable for hours; users had to
// kubectl-edit the CR by hand. (See task #68, ADR-026 background.)
func ForceDeleteBackup(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	backup := &velerov1.Backup{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: velerons.Namespace()}, backup); err != nil {
		if apierrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "backup not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Same authz scope as DeleteBackup — force-delete must not be a
	// privilege-escalation path. Whole-cluster backup still requires admin.
	if len(backup.Spec.IncludedNamespaces) == 0 {
		user := auth.FromContext(c)
		if user != nil && user.Role == auth.RoleEditor {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "force-deleting a whole-cluster backup requires admin",
			})
			return
		}
	} else if !auth.RequireNamespaceAccess(c, backup.Spec.IncludedNamespaces...) {
		return
	}

	hadFinalizers := append([]string{}, backup.Finalizers...)
	bypassed := []string{}

	// Step 1: strip finalizers if any. We patch with empty slice (not nil)
	// so the JSON-Patch is unambiguous.
	if len(backup.Finalizers) > 0 {
		patch := client.MergeFrom(backup.DeepCopy())
		backup.Finalizers = []string{}
		if err := cl.Patch(context.Background(), backup, patch); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":         "failed to strip finalizers: " + err.Error(),
				"hadFinalizers": hadFinalizers,
			})
			return
		}
		bypassed = append(bypassed, fmt.Sprintf("finalizers (%s)", strings.Join(hadFinalizers, ",")))
	}

	// Step 2: direct delete. NO DBR — that's the path we just abandoned.
	if err := cl.Delete(context.Background(), backup); err != nil && !apierrors.IsNotFound(err) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":              "failed to delete Backup after stripping finalizers: " + err.Error(),
			"strippedFinalizers": hadFinalizers,
		})
		return
	}
	bypassed = append(bypassed, "DeleteBackupRequest cascade (BSL tarball NOT removed)")

	// Step 3: schedule orphan GC so the cloud snapshot side doesn't leak.
	// Per D-11 the orphan-VSC/cloud-snapshot trap is the real billing risk.
	k8sCli, kerr := k8s.GetClient()
	if kerr == nil {
		gc.TriggerSoon(context.Background(), cl, k8sCli)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "Backup force-deleted. Review 'bypassed' — those side effects were skipped.",
		"backupName":        name,
		"bypassed":          bypassed,
		"orphanGcScheduled": kerr == nil,
		"warning":           "BSL tarball not removed by force-delete. If you want the storage reclaimed, also remove the BSL object directly or wait for retention policy.",
	})
}

// ListRestores returns all Velero restores
func ListRestores(c *gin.Context) {
	cl, err := getRequestRuntimeClient(c) // v0.9.0.1 — header-routed
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	restoreList := &velerov1.RestoreList{}
	namespace := c.DefaultQuery("namespace", velerons.Namespace())
	if err := cl.List(context.Background(), restoreList, client.InNamespace(namespace)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := restoreList.Items
	if rawLimit := c.Query("limit"); rawLimit != "" {
		if limit, err := strconv.Atoi(rawLimit); err == nil && limit > 0 && limit < len(items) {
			items = items[:limit]
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// CreateRestore creates a new Velero restore (v0.7.10 — Kasten-style flow).
//
// Why each field exists (problem this solves):
//
//   - NamespaceMapping  : target ns picker. Empty mapping → restore in-place to
//     original ns. Non-empty value redirects each source ns to its target.
//   - CleanupBeforeRestore : Velero's default `existingResourcePolicy: none`
//     skips resources that already exist, which makes the restore a no-op when
//     the target ns is unchanged. With this flag we delete the target ns first
//     and wait until it's fully gone, then create the Restore. This matches
//     Kasten's "the contents of the selected namespace will be overwritten"
//     promise.
//   - IncludedResources / ExcludedResources : per-artifact selection from the
//     drawer's Spec checkbox list. We pass them through to Velero, which
//     filters at restore time.
//   - ExistingResourcePolicy : explicit "" / "none" / "update" choice for
//     advanced users who do not want a full ns wipe but still want to overwrite
//     mutable resources like ConfigMaps/Secrets.
func CreateRestore(c *gin.Context) {
	var req struct {
		Name                   string            `json:"name" binding:"required"`
		BackupName             string            `json:"backupName" binding:"required"`
		IncludedNamespaces     []string          `json:"includedNamespaces"`
		NamespaceMapping       map[string]string `json:"namespaceMapping"`
		CleanupBeforeRestore   bool              `json:"cleanupBeforeRestore"`
		IncludedResources      []string          `json:"includedResources"`
		ExcludedResources      []string          `json:"excludedResources"`
		ExistingResourcePolicy string            `json:"existingResourcePolicy"`
		// v0.8.2 Transform Set integration. When non-empty, the Restore
		// references the named ConfigMap in velero ns so Velero applies
		// the JSONPath patches at restore time. We don't materialize
		// anything new here — just write the ref.
		TransformSetName string `json:"transformSetName,omitempty"`
		// v0.9.0 MC3 — cross-cluster restore. Empty or "this-cluster" means
		// "restore on the local cluster" (existing v0.8 behavior). Otherwise
		// the named cluster must be a registered Cluster CR; we use its
		// kubeconfig to apply the Restore CR on the remote cluster instead.
		// The remote cluster's Velero must already have the Backup metadata
		// synced (via BackupSyncController from a shared BSL) — MC4 will
		// automate the BSL sync; for MC3 we just apply the Restore CR and
		// let Velero fail-loudly if metadata isn't present yet.
		TargetCluster string `json:"targetCluster,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// v0.8.5 step 3: editor must have access to BOTH source ns (from
	// IncludedNamespaces) AND target ns (from NamespaceMapping values).
	// Otherwise an editor scoped to ns-A could restore an ns-B backup
	// into ns-A — sneakily exfiltrating data they shouldn't see.
	nsToCheck := append([]string{}, req.IncludedNamespaces...)
	nsToCheck = append(nsToCheck, collectRestoreTargetNamespaces(req.NamespaceMapping, req.IncludedNamespaces)...)
	if !auth.RequireNamespaceAccess(c, nsToCheck...) {
		return
	}

	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Cleanup-before-restore: delete the target namespace(s) first. We compute
	// the target set from NamespaceMapping values (when present) or from
	// IncludedNamespaces (when restoring in-place). Refuses to touch protected
	// namespaces — that's a guardrail, not a user choice.
	if req.CleanupBeforeRestore {
		targets := collectRestoreTargetNamespaces(req.NamespaceMapping, req.IncludedNamespaces)
		if err := deleteNamespacesAndWait(context.Background(), targets, 60*time.Second); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("cleanup failed: %v", err)})
			return
		}
	}

	spec := velerov1.RestoreSpec{
		BackupName:         req.BackupName,
		IncludedNamespaces: req.IncludedNamespaces,
		NamespaceMapping:   req.NamespaceMapping,
		IncludedResources:  req.IncludedResources,
		ExcludedResources:  req.ExcludedResources,
	}
	// Velero rejects unknown ExistingResourcePolicy values. Only forward when
	// the caller explicitly set "none" or "update"; empty string preserves
	// Velero's default behavior.
	switch req.ExistingResourcePolicy {
	case "none", "update":
		spec.ExistingResourcePolicy = velerov1.PolicyType(req.ExistingResourcePolicy)
	case "":
		// leave unset
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid existingResourcePolicy %q (allowed: none, update, empty)", req.ExistingResourcePolicy)})
		return
	}

	// v0.8.2: Transform Set ref. Velero expects ResourceModifier to live
	// as a ConfigMap in the same namespace (velero) — we just write the
	// LocalObjectReference. The ConfigMap itself was created by the
	// Transform Set CRUD endpoints earlier (or by the Pre-flight
	// "Apply Suggested Fix" flow).
	if req.TransformSetName != "" {
		spec.ResourceModifier = &corev1.TypedLocalObjectReference{
			APIGroup: nil, // core/v1 → APIGroup must be nil
			Kind:     "ConfigMap",
			Name:     req.TransformSetName,
		}
	}

	restore := &velerov1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: velerons.Namespace(),
			Labels: map[string]string{
				"supkube.io/managed-by": "supkube",
			},
			Annotations: map[string]string{},
		},
		Spec: spec,
	}

	// v0.9.0 MC3+MC4 — cross-cluster dispatch.
	//
	// If targetCluster is empty or "this-cluster", fall through to the
	// existing local client. Otherwise:
	//   1. (MC3) Resolve target cluster + its kubeconfig Secret, build a
	//      remote velero-aware client.
	//   2. (MC4) Auto-mirror the BSL + credential Secret onto the target
	//      and wait for the target's Velero to sync the Backup CR.
	//   3. (MC3) Apply the Restore CR on the remote cluster.
	if req.TargetCluster != "" && req.TargetCluster != "this-cluster" {
		// Stamp the source cluster identity for audit / cross-cluster UI
		// reconciliation. The target cluster's Velero ignores this; only
		// SupKube cares about it.
		restore.ObjectMeta.Annotations["supkube.io/restore-target-cluster"] = req.TargetCluster
		restore.ObjectMeta.Annotations["supkube.io/restore-initiated-by"] = "supkube-multicluster"

		remoteCli, rerr := buildRemoteVeleroClient(context.Background(), req.TargetCluster)
		if rerr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "target cluster: " + rerr.Error()})
			return
		}

		// MC4 — auto-sync BSL + secret + wait for Backup CR. Takes up to
		// backupSyncTimeout (90s default). The user sees a spinner; this
		// is acceptable UX because cross-cluster restore is intrinsically
		// an "explicit slow op" — they expect the wait.
		prepCtx, prepCancel := context.WithTimeout(context.Background(), backupSyncTimeout+15*time.Second)
		defer prepCancel()
		if err := prepareTargetForCrossClusterRestore(prepCtx, remoteCli, req.BackupName); err != nil {
			// Surface the precise message — the SPA shows it raw to the
			// admin; pre-flight failures are typically actionable
			// (wrong BSL, secret missing, network ACL, etc.).
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
			return
		}

		if err := remoteCli.Create(context.Background(), restore); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "remote restore create: " + err.Error()})
			return
		}
		// Echo the same CR back in the response with the target-cluster
		// annotation visible so the SPA can pivot the user to the target
		// cluster's Activity page.
		c.JSON(http.StatusCreated, restore)
		return
	}

	// Local-cluster path (unchanged from v0.8.x).
	if err := cl.Create(context.Background(), restore); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, restore)
}

// collectRestoreTargetNamespaces returns the set of namespaces that the
// restore will populate. NamespaceMapping values (target side) take precedence;
// when no mapping is given we fall back to IncludedNamespaces (in-place).
func collectRestoreTargetNamespaces(mapping map[string]string, included []string) []string {
	seen := map[string]struct{}{}
	add := func(n string) {
		if n == "" {
			return
		}
		seen[n] = struct{}{}
	}
	if len(mapping) > 0 {
		for _, v := range mapping {
			add(v)
		}
	} else {
		for _, n := range included {
			add(n)
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	return out
}

// deleteNamespacesAndWait deletes each namespace and blocks until each one is
// actually gone (or the per-namespace timeout elapses). Returns the first
// error encountered. Protected namespaces are rejected up-front so the user
// cannot blow up velero/kube-system by accident.
func deleteNamespacesAndWait(ctx context.Context, names []string, timeout time.Duration) error {
	if len(names) == 0 {
		return nil
	}
	for _, n := range names {
		if isProtectedNamespace(n) {
			return fmt.Errorf("refusing to delete protected namespace %q", n)
		}
	}
	cl, err := k8s.GetClient()
	if err != nil {
		return err
	}
	for _, name := range names {
		// Issue delete; ignore NotFound (idempotent).
		if err := cl.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete ns %q: %w", name, err)
		}
		// Poll until the namespace is fully removed. Velero's restore would
		// otherwise race a still-Terminating ns and report "operation cannot
		// be fulfilled".
		deadline := time.Now().Add(timeout)
		for {
			_, err := cl.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("namespace %q still terminating after %s", name, timeout)
			}
			time.Sleep(1 * time.Second)
		}
	}
	return nil
}

// GetRestore returns a specific restore
func GetRestore(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	restore := &velerov1.Restore{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: velerons.Namespace()}, restore); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, restore)
}

// DeleteRestore deletes a restore
func DeleteRestore(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	restore := &velerov1.Restore{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: velerons.Namespace()}, restore); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// v0.8.5 step 3: editor must have access to the Restore's target ns.
	// Target derived from NamespaceMapping (cross-ns) or IncludedNamespaces (in-place).
	targets := collectRestoreTargetNamespaces(restore.Spec.NamespaceMapping, restore.Spec.IncludedNamespaces)
	if !auth.RequireNamespaceAccess(c, targets...) {
		return
	}
	if err := cl.Delete(context.Background(), restore); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "restore deleted"})
}

// GetRestoreResults returns the full structured errors/warnings for a Restore.
//
// Why this used to be a stub: Velero only stores error/warning *counts* in
// `Restore.status.errors` and `.warnings`. The actual messages live in object
// storage (BSL) as a gzipped JSON blob keyed by namespace. To retrieve them
// you must create a `DownloadRequest` CRD with target kind RestoreResults,
// wait for the Velero controller to populate `.status.downloadURL` (a
// short-lived presigned BSL URL), then HTTP GET + gunzip the body.
//
// v0.7.11: we now do exactly that, so the Restores page can actually show
// the user *why* a restore reported `FinalizingPartiallyFailed` instead of
// just an opaque "Errors (1)" badge.
//
// The detailed fetch is best-effort: if anything in the DownloadRequest dance
// fails (controller slow, BSL temporarily unreachable, URL expired), we still
// return the summary so the drawer renders. The error message is exposed as
// `detailedError` for diagnostics.
func GetRestoreResults(c *gin.Context) {
	name := c.Param("name")
	// v0.9.1.10 (#103): header-routed, matching ListRestores. Previously this
	// used the always-local k8s.GetRuntimeClient(), so when the user viewed a
	// restore under a non-local cluster selector the lookup 404'd — they saw
	// the "N errors" count (ListActions/GetAction) but got nothing when they
	// drilled in ("3 errors 看不到", 2026-05-28 demo). Routing both reads
	// through the same cluster keeps the count and the detail consistent.
	cl, err := getRequestRuntimeClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	restore := &velerov1.Restore{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: velerons.Namespace()}, restore); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	status := restore.Status

	payload := gin.H{
		"name":                restore.Name,
		"phase":               string(status.Phase),
		"failureReason":       status.FailureReason,
		"validationErrors":    status.ValidationErrors,
		"errors":              status.Errors,
		"warnings":            status.Warnings,
		"progress":            status.Progress,
		"startTimestamp":      status.StartTimestamp,
		"completionTimestamp": status.CompletionTimestamp,
	}

	// Only fetch detailed results when the restore is in a terminal state AND
	// has something interesting to report. Skipping the call for green
	// restores avoids the ~3s round-trip cost on the happy path. Velero phases
	// we treat as "ready to download":
	//   Completed, PartiallyFailed, FinalizingPartiallyFailed, Failed
	phase := string(status.Phase)
	terminal := phase == "Completed" || phase == "PartiallyFailed" ||
		phase == "Failed" || phase == "FinalizingPartiallyFailed" ||
		phase == "Finalizing"
	if terminal && (status.Errors > 0 || status.Warnings > 0) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		detailed, err := fetchRestoreDetailedResults(ctx, name)
		if err != nil {
			payload["detailedError"] = err.Error()
		} else {
			payload["detailed"] = detailed
		}
	}

	c.JSON(http.StatusOK, payload)
}

// ListSchedules returns Policies (aggregated view, v0.8.9).
//
// Pre-v0.8.9 this returned a flat array of velerov1.Schedule. v0.8.9
// adds the dual-schedule pair model — two K8s Schedules forming one
// logical Policy. To preserve the 1-row-per-policy UI experience we
// aggregate here.
//
// Response shape change: items[] now contains PolicyAggregate entries,
// not raw Schedules. Frontend reads:
//
//	item.policyName       → display name + URL identifier
//	item.mode             → "single" | "dual"
//	item.snapshotSchedule → the L1 / snapshot half (always present)
//	item.exportSchedule   → only set when mode=dual
//
// Backwards compat: legacy pre-v0.8.9 single Schedules (no labels) come
// through as mode=single with the Schedule in snapshotSchedule — old
// frontends that read `item.snapshotSchedule.metadata.name` keep working
// for these. New frontends should read item.policyName + .mode.
func ListSchedules(c *gin.Context) {
	cl, err := getRequestRuntimeClient(c) // v0.9.0.1 — header-routed
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	scheduleList := &velerov1.ScheduleList{}
	namespace := c.DefaultQuery("namespace", velerons.Namespace())
	if err := cl.List(context.Background(), scheduleList, client.InNamespace(namespace)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	policies := aggregateSchedulesIntoPolicies(scheduleList.Items)
	// v0.8.10.1: enrich each policy with restorePointCount so the
	// Policies page can show a clickable RP count column. Single bulk
	// Backup list inside — non-fatal on error (column shows 0).
	policies = enrichRestorePointCounts(context.Background(), cl, policies)
	c.JSON(http.StatusOK, gin.H{
		"items": policies,
		"total": len(policies),
	})
}

// CreateSchedule creates a new Velero schedule.
// CreateSchedule (v0.8.9) — creates either ONE or TWO Velero Schedules
// representing a SupKube Policy.
//
// Single-schedule (L1, legacy): produces 1 Schedule with `policy-role:
// snapshot` label. Backups it fires keep the CSI snapshot cluster-local.
//
// Dual-schedule (L2, new): produces 2 Schedules — `<name>` (snapshot
// half, snapshotMoveData=false, short TTL, local BSL) and
// `<name>-export` (export half, snapshotMoveData=true, long TTL, cloud
// BSL). Each fires independently per its own cron; the resulting two
// Backups are grouped in the UI by the shared `policy-name` label.
//
// See policy.go for the data model + naming convention.
func CreateSchedule(c *gin.Context) {
	// Backwards-compat: accept the legacy single-schedule fields
	// alongside the new Dual + role-specific fields. Frontend can
	// migrate at its own pace.
	var req struct {
		// Common fields (apply to either / both halves)
		Name                     string            `json:"name" binding:"required"`
		Schedule                 string            `json:"schedule" binding:"required"`
		IncludedNamespaces       []string          `json:"includedNamespaces"`
		SnapshotVolumes          *bool             `json:"snapshotVolumes"`
		DefaultVolumesToFsBackup *bool             `json:"defaultVolumesToFsBackup"`
		Paused                   bool              `json:"paused"`
		Annotations              map[string]string `json:"annotations"`

		// Legacy single-schedule fields (used when Dual=false)
		TTL              string `json:"ttl"`
		StorageLocation  string `json:"storageLocation"`
		SnapshotMoveData *bool  `json:"snapshotMoveData"` // legacy; new code derives from role

		// v0.8.9 Dual mode (L2) fields
		Dual                    bool   `json:"dual"`
		SnapshotTTL             string `json:"snapshotTtl,omitempty"`
		SnapshotStorageLocation string `json:"snapshotStorageLocation,omitempty"`
		ExportTTL               string `json:"exportTtl,omitempty"`
		ExportStorageLocation   string `json:"exportStorageLocation,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// v0.8.5 step 3: editor must have access to all included namespaces.
	if !auth.RequireNamespaceAccess(c, req.IncludedNamespaces...) {
		return
	}
	if len(req.IncludedNamespaces) == 0 {
		user := auth.FromContext(c)
		if user != nil && user.Role == auth.RoleEditor {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "editors must explicitly list includedNamespaces (whole-cluster schedules require admin)",
			})
			return
		}
	}

	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Adapt the request into the cross-cutting PolicyRequest used by
	// policy.go's builders. The Dual flag drives whether we materialize
	// one or two schedules.
	pr := &PolicyRequest{
		Name:                     req.Name,
		Schedule:                 req.Schedule,
		IncludedNamespaces:       req.IncludedNamespaces,
		Paused:                   req.Paused,
		Annotations:              req.Annotations,
		SnapshotVolumes:          req.SnapshotVolumes,
		DefaultVolumesToFsBackup: req.DefaultVolumesToFsBackup,
		Dual:                     req.Dual,
	}
	if req.Dual {
		// L2: caller MUST supply per-role TTLs + BSLs. We don't fall
		// back to single-mode TTL to avoid silent UX surprise.
		pr.SnapshotTTL = req.SnapshotTTL
		pr.SnapshotStorageLocation = req.SnapshotStorageLocation
		pr.ExportTTL = req.ExportTTL
		pr.ExportStorageLocation = req.ExportStorageLocation
	} else {
		// L1 (or legacy): the single TTL + BSL apply to the snapshot half.
		pr.SnapshotTTL = req.TTL
		pr.SnapshotStorageLocation = req.StorageLocation
	}

	// v0.9.1.10 (#101 finding 1 / #107 sibling): resolve empty BSLs + validate
	// explicit ones, so a policy never silently inherits Velero's implicit
	// "default" BSL. On clusters whose BSL isn't named "default" (our AKS dev
	// has only "azure-blob"), a blank half produced Backups that failed
	// validation with "BackupStorageLocation \"default\" not found".
	//   - snapshot half: general resolver (the local copy may legitimately be
	//     the in-cluster MinIO, the sole BSL, or a flagged default).
	//   - export half:  CLOUD-preferring resolver so the off-site copy never
	//     collapses onto the Local store (keeps the 3-2-1-1-0 promise).
	// Resolve fires ONLY when the caller left the field empty; an explicit
	// name is validated and otherwise passed through untouched.
	if pr.SnapshotStorageLocation == "" {
		name, rerr := resolveBackupStorageLocation(context.Background(), cl)
		if rerr != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "snapshot storage location: " + rerr.Error()})
			return
		}
		pr.SnapshotStorageLocation = name
	} else if rerr := assertBSLExists(context.Background(), cl, pr.SnapshotStorageLocation); rerr != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "snapshot storage location: " + rerr.Error()})
		return
	}
	if req.Dual {
		if pr.ExportStorageLocation == "" {
			name, rerr := resolveExportStorageLocation(context.Background(), cl)
			if rerr != nil {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "export storage location: " + rerr.Error()})
				return
			}
			pr.ExportStorageLocation = name
		} else if rerr := assertBSLExists(context.Background(), cl, pr.ExportStorageLocation); rerr != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "export storage location: " + rerr.Error()})
			return
		}
	}

	// Build + create the snapshot half. Always present.
	snapSched, err := pr.buildSchedule(roleSnapshot)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := cl.Create(context.Background(), snapSched); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Optionally build + create the export half. If this fails after
	// the snapshot half was already created, we surface the error AND
	// leave the snapshot half in place (a partial L2 = a usable L1,
	// rather than aborting both and leaving the user with nothing).
	if req.Dual {
		expSched, err := pr.buildSchedule(roleExport)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "snapshot half created but export half failed to build: " + err.Error(),
				"partialPolicy": snapSched.Name,
			})
			return
		}
		if err := cl.Create(context.Background(), expSched); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":         "snapshot half created but export half failed: " + err.Error(),
				"partialPolicy": snapSched.Name,
			})
			return
		}
		c.JSON(http.StatusCreated, PolicyAggregate{
			PolicyName:       req.Name,
			Mode:             "dual",
			SnapshotSchedule: snapSched,
			ExportSchedule:   expSched,
		})
		return
	}

	// Single-schedule response (L1).
	c.JSON(http.StatusCreated, PolicyAggregate{
		PolicyName:       req.Name,
		Mode:             "single",
		SnapshotSchedule: snapSched,
	})
}

// DeleteSchedule deletes a Policy — pair-aware in v0.8.9.
//
// If the policy is dual-mode (snapshot + export), BOTH schedules are
// deleted. RBAC check runs on the snapshot half's ns scope (export
// half always has the same ns since they're built from one PolicyRequest).
//
// Legacy single-schedule policies (no `policy-name` label) are deleted
// by their exact name, same as pre-v0.8.9.
func DeleteSchedule(c *gin.Context) {
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
	// Editor RBAC: scope-cover the snapshot half's ns. Both halves
	// always share IncludedNamespaces by construction.
	if !auth.RequireNamespaceAccess(c, policy.SnapshotSchedule.Spec.Template.IncludedNamespaces...) {
		return
	}
	// Delete in order: export first (so a partial failure leaves the
	// snapshot half — a usable L1 — rather than orphaning the export).
	deleted := []string{}
	if policy.ExportSchedule != nil {
		if err := cl.Delete(c.Request.Context(), policy.ExportSchedule); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to delete export half: " + err.Error(),
				"deleted": deleted,
			})
			return
		}
		deleted = append(deleted, policy.ExportSchedule.Name)
	}
	if err := cl.Delete(c.Request.Context(), policy.SnapshotSchedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to delete snapshot half: " + err.Error(),
			"deleted": deleted,
		})
		return
	}
	deleted = append(deleted, policy.SnapshotSchedule.Name)
	c.JSON(http.StatusOK, gin.H{
		"message": "policy deleted",
		"deleted": deleted,
	})
}

// ListStorageLocations returns all backup storage locations
func ListStorageLocations(c *gin.Context) {
	cl, err := getRequestRuntimeClient(c) // v0.9.0.1 — header-routed
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	bslList := &velerov1.BackupStorageLocationList{}
	namespace := c.DefaultQuery("namespace", velerons.Namespace())
	if err := cl.List(context.Background(), bslList, client.InNamespace(namespace)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": bslList.Items, "total": len(bslList.Items)})
}

// CreateStorageLocation creates a new backup storage location
func CreateStorageLocation(c *gin.Context) {
	var req struct {
		Name     string            `json:"name" binding:"required"`
		Provider string            `json:"provider" binding:"required"`
		Bucket   string            `json:"bucket" binding:"required"`
		Config   map[string]string `json:"config"`
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
	bsl := &velerov1.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: velerons.Namespace(),
		},
		Spec: velerov1.BackupStorageLocationSpec{
			Provider: req.Provider,
			StorageType: velerov1.StorageType{
				ObjectStorage: &velerov1.ObjectStorageLocation{
					Bucket: req.Bucket,
				},
			},
			Config: req.Config,
		},
	}
	if err := cl.Create(context.Background(), bsl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, bsl)
}
