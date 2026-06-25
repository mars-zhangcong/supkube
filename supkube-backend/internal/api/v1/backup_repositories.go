package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/velerons"
)

// BackupRepoDTO is the trimmed, UI-facing view of a Velero BackupRepository CR.
// We deliberately don't return the raw CR (managedFields / full spec) — the UI
// only needs identity + storage location + maintenance/phase. The dedup-stats
// fields (PRD-029/N2 "去重可见") are declared but not populated yet: the Velero
// BackupRepository CR doesn't carry dedup numbers; those come from the Kopia
// repository service (PRD-029, a later human-led WI). Keeping the field here now
// stabilises the UI contract so adding the real source later is non-breaking.
type BackupRepoDTO struct {
	Name                  string `json:"name"`
	RepositoryType        string `json:"repositoryType"` // kopia | restic
	VolumeNamespace       string `json:"volumeNamespace"`
	BackupStorageLocation string `json:"backupStorageLocation"`
	Phase                 string `json:"phase"` // New | Ready | NotReady
	Message               string `json:"message,omitempty"`
	LastMaintenanceTime   string `json:"lastMaintenanceTime,omitempty"` // RFC3339; "" if never run
	DedupRatio            string `json:"dedupRatio,omitempty"`          // TODO(PRD-029): 去重比, source = Kopia repo service
}

// backupRepoToDTO trims one BackupRepository CR into the UI DTO. Pure function
// (no client/context) so it's unit-testable without a cluster.
func backupRepoToDTO(repo velerov1.BackupRepository) BackupRepoDTO {
	dto := BackupRepoDTO{
		Name:                  repo.Name,
		RepositoryType:        repo.Spec.RepositoryType,
		VolumeNamespace:       repo.Spec.VolumeNamespace,
		BackupStorageLocation: repo.Spec.BackupStorageLocation,
		Phase:                 string(repo.Status.Phase),
		Message:               repo.Status.Message,
	}
	// Guard the nil pointer: a never-maintained repo has no LastMaintenanceTime,
	// and we want "" in the JSON, not Go's zero time ("0001-01-01T00:00:00Z").
	if t := repo.Status.LastMaintenanceTime; t != nil {
		dto.LastMaintenanceTime = t.Format(time.RFC3339)
	}
	return dto
}

// ListBackupRepositories returns the cluster's Velero BackupRepository CRs as
// trimmed DTOs. Read-only; the client is resolved via getRequestRuntimeClient
// so the X-Supkube-Cluster header routes the read to the right cluster.
// PRD-029/N2 (Kopia 仓库服务) — the first UI window into "有哪些备份仓库".
func ListBackupRepositories(c *gin.Context) {
	runtimeClient, err := getRequestRuntimeClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	list := &velerov1.BackupRepositoryList{}
	if err := runtimeClient.List(context.Background(), list, client.InNamespace(velerons.Namespace())); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]BackupRepoDTO, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, backupRepoToDTO(list.Items[i]))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}
