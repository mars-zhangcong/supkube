package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supkube/supkube-backend/internal/auth"
)

// TestBackupRepoToDTO covers the pure CR→DTO mapper (the real logic). Table-driven,
// no cluster needed — this is the unit-testable seam the handler delegates to.
func TestBackupRepoToDTO(t *testing.T) {
	maint := metav1.NewTime(time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC))

	tests := []struct {
		name string
		in   velerov1.BackupRepository
		want BackupRepoDTO
	}{
		{
			name: "kopia Ready with maintenance time",
			in: velerov1.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{Name: "app-kopia-abc123"},
				Spec: velerov1.BackupRepositorySpec{
					RepositoryType:        velerov1.BackupRepositoryTypeKopia,
					VolumeNamespace:       "app",
					BackupStorageLocation: "default",
				},
				Status: velerov1.BackupRepositoryStatus{
					Phase:               velerov1.BackupRepositoryPhaseReady,
					LastMaintenanceTime: &maint,
				},
			},
			want: BackupRepoDTO{
				Name:                  "app-kopia-abc123",
				RepositoryType:        "kopia",
				VolumeNamespace:       "app",
				BackupStorageLocation: "default",
				Phase:                 "Ready",
				LastMaintenanceTime:   "2026-06-25T10:00:00Z",
			},
		},
		{
			name: "never maintained → empty string, not Go zero time",
			in: velerov1.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{Name: "new-repo"},
				Spec:       velerov1.BackupRepositorySpec{RepositoryType: velerov1.BackupRepositoryTypeRestic},
				Status:     velerov1.BackupRepositoryStatus{Phase: velerov1.BackupRepositoryPhaseNew},
			},
			want: BackupRepoDTO{
				Name:           "new-repo",
				RepositoryType: "restic",
				Phase:          "New",
				// LastMaintenanceTime intentionally "" (nil pointer guard)
			},
		},
		{
			name: "NotReady carries status message",
			in: velerov1.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{Name: "broken-repo"},
				Status: velerov1.BackupRepositoryStatus{
					Phase:   velerov1.BackupRepositoryPhaseNotReady,
					Message: "unable to connect to storage",
				},
			},
			want: BackupRepoDTO{
				Name:    "broken-repo",
				Phase:   "NotReady",
				Message: "unable to connect to storage",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backupRepoToDTO(tt.in)
			if got != tt.want {
				t.Errorf("backupRepoToDTO()\n got = %+v\nwant = %+v", got, tt.want)
			}
		})
	}
}

// TestListBackupRepositories_NoCluster mirrors the codebase's *_NoCluster style:
// without a reachable cluster the handler must fail gracefully (500, no panic).
// If a cluster IS available (local dev) a 200 with {items,total} is also fine.
func TestListBackupRepositories_NoCluster(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &auth.User{Subject: "test", Username: "test", Role: auth.RoleAdmin})
		c.Next()
	})
	r.GET("/api/v1/backup-repositories", ListBackupRepositories)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/backup-repositories", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Errorf("expected 500 (no cluster) or 200 (cluster present), got %d: %s", w.Code, w.Body.String())
	}
}
