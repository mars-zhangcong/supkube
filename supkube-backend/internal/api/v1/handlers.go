package v1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// GetStatus returns system status
func GetStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "0.7.9-alpha",
	})
}

// ListNamespaces returns all namespaces in the cluster
func ListNamespaces(c *gin.Context) {
	cl, err := k8s.GetClient()
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

// ListBackups returns all Velero backups
func ListBackups(c *gin.Context) {
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	backupList := &velerov1.BackupList{}
	namespace := c.DefaultQuery("namespace", "velero")
	if err := cl.List(context.Background(), backupList, client.InNamespace(namespace)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": backupList.Items, "total": len(backupList.Items)})
}

// CreateBackup creates a new Velero backup.
// v0.6 dual-mode volume backup:
//   - SnapshotVolumes=true        → CSI snapshot path (Velero v1.14+ core)
//   - DefaultVolumesToFsBackup=true → Restic/Kopia filesystem backup path
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

	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	backup := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: "velero",
		},
		Spec: velerov1.BackupSpec{
			IncludedNamespaces:       req.IncludedNamespaces,
			ExcludedNamespaces:       req.ExcludedNamespaces,
			SnapshotVolumes:          req.SnapshotVolumes,
			DefaultVolumesToFsBackup: req.DefaultVolumesToFsBackup,
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
	if req.StorageLocation != "" {
		backup.Spec.StorageLocation = req.StorageLocation
	}
	if err := cl.Create(context.Background(), backup); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, backup)
}

// GetBackup returns a specific backup
func GetBackup(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	backup := &velerov1.Backup{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, backup); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, backup)
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
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, backup); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// DBR name: <backup>-<ts>. Velero auto-cleans DBRs after processing;
	// the unique suffix lets us re-trigger a delete if a prior one stalled.
	dbrName := fmt.Sprintf("%s-delete-%s", name, time.Now().UTC().Format("20060102150405"))
	dbr := &velerov1.DeleteBackupRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbrName,
			Namespace: "velero",
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
	c.JSON(http.StatusAccepted, gin.H{
		"message":              "Delete in progress (cascade)",
		"deleteBackupRequest":  dbrName,
		"backupName":           name,
	})
}

// ListRestores returns all Velero restores
func ListRestores(c *gin.Context) {
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	restoreList := &velerov1.RestoreList{}
	namespace := c.DefaultQuery("namespace", "velero")
	if err := cl.List(context.Background(), restoreList, client.InNamespace(namespace)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": restoreList.Items, "total": len(restoreList.Items)})
}

// CreateRestore creates a new Velero restore
func CreateRestore(c *gin.Context) {
	var req struct {
		Name                string   `json:"name" binding:"required"`
		BackupName          string   `json:"backupName" binding:"required"`
		IncludedNamespaces  []string `json:"includedNamespaces"`
		NamespaceMapping    map[string]string `json:"namespaceMapping"`
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
	restore := &velerov1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: "velero",
		},
		Spec: velerov1.RestoreSpec{
			BackupName:         req.BackupName,
			IncludedNamespaces: req.IncludedNamespaces,
			NamespaceMapping:   req.NamespaceMapping,
		},
	}
	if err := cl.Create(context.Background(), restore); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, restore)
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
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, restore); err != nil {
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
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, restore); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err := cl.Delete(context.Background(), restore); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "restore deleted"})
}

// GetRestoreResults returns structured errors/warnings from the restore status.
// Velero stores the full log/results files in object storage via DownloadRequest;
// that flow will be wired in v0.6. For now this endpoint exposes everything that's
// already in the Restore CR's .status — which covers validation failures, per-
// resource errors, and warnings and is enough to debug most Failed restores.
func GetRestoreResults(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	restore := &velerov1.Restore{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, restore); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	status := restore.Status
	c.JSON(http.StatusOK, gin.H{
		"name":              restore.Name,
		"phase":             string(status.Phase),
		"failureReason":     status.FailureReason,
		"validationErrors":  status.ValidationErrors,
		"errors":            status.Errors,
		"warnings":          status.Warnings,
		"progress":          status.Progress,
		"startTimestamp":    status.StartTimestamp,
		"completionTimestamp": status.CompletionTimestamp,
	})
}

// ListSchedules returns all Velero schedules
func ListSchedules(c *gin.Context) {
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	scheduleList := &velerov1.ScheduleList{}
	namespace := c.DefaultQuery("namespace", "velero")
	if err := cl.List(context.Background(), scheduleList, client.InNamespace(namespace)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": scheduleList.Items, "total": len(scheduleList.Items)})
}

// CreateSchedule creates a new Velero schedule.
// v0.7 Actions model: the UI collapses Snapshot+Export intent into a single
// Velero Schedule, but carries the original intent in annotations so the
// v0.9 self-managed scheduler can split it back out. Annotations on the
// Schedule itself (not on the template) — they describe policy intent, not
// per-backup metadata.
func CreateSchedule(c *gin.Context) {
	var req struct {
		Name                     string            `json:"name" binding:"required"`
		Schedule                 string            `json:"schedule" binding:"required"`
		IncludedNamespaces       []string          `json:"includedNamespaces"`
		TTL                      string            `json:"ttl"`
		StorageLocation          string            `json:"storageLocation"`
		SnapshotVolumes          *bool             `json:"snapshotVolumes"`
		DefaultVolumesToFsBackup *bool             `json:"defaultVolumesToFsBackup"`
		Annotations              map[string]string `json:"annotations"`
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

	template := velerov1.BackupSpec{
		IncludedNamespaces:       req.IncludedNamespaces,
		SnapshotVolumes:          req.SnapshotVolumes,
		DefaultVolumesToFsBackup: req.DefaultVolumesToFsBackup,
	}
	if req.TTL != "" {
		duration, parseErr := time.ParseDuration(req.TTL)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid TTL format: " + parseErr.Error()})
			return
		}
		template.TTL = metav1.Duration{Duration: duration}
	}
	if req.StorageLocation != "" {
		template.StorageLocation = req.StorageLocation
	}

	schedule := &velerov1.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.Name,
			Namespace:   "velero",
			Annotations: req.Annotations,
		},
		Spec: velerov1.ScheduleSpec{
			Schedule: req.Schedule,
			Template: template,
		},
	}
	if err := cl.Create(context.Background(), schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, schedule)
}

// DeleteSchedule deletes a schedule
func DeleteSchedule(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	schedule := &velerov1.Schedule{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, schedule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err := cl.Delete(context.Background(), schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "schedule deleted"})
}

// ListStorageLocations returns all backup storage locations
func ListStorageLocations(c *gin.Context) {
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	bslList := &velerov1.BackupStorageLocationList{}
	namespace := c.DefaultQuery("namespace", "velero")
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
			Namespace: "velero",
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
