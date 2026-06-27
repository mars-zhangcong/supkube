package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/supkube/supkube-backend/internal/drflow"
	"github.com/supkube/supkube-backend/internal/velerons"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetDRPosture exports SupKube's Velero + DRFlowRunner state in the SupInsight
// canonical backup/DR posture shape (engine-agnostic). SupInsight's SupKubeAdapter
// pulls this and ingests it — we own both ends, so SupKube speaks SupInsight's
// language directly (no translation layer). JSON field names MUST match
// supinsight-backend/internal/ingest/model.go.
func GetDRPosture(c *gin.Context) {
	cl, err := getRequestRuntimeClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	k8sCli, err := getRequestKubernetesClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx := context.Background()
	ns := velerons.Namespace()

	// Best-effort lists (partial posture beats a hard fail — same as topology.go).
	backups := &velerov1.BackupList{}
	_ = cl.List(ctx, backups, client.InNamespace(ns))
	bsls := &velerov1.BackupStorageLocationList{}
	_ = cl.List(ctx, bsls, client.InNamespace(ns))
	schedules := &velerov1.ScheduleList{}
	_ = cl.List(ctx, schedules, client.InNamespace(ns))
	restores := &velerov1.RestoreList{}
	_ = cl.List(ctx, restores, client.InNamespace(ns))
	runs, _ := drflow.ListRuns(ctx, k8sCli)

	cluster := c.GetHeader("X-Supkube-Cluster")
	if cluster == "" {
		cluster = "this-cluster"
	}

	out := drPosture{
		Engine:      drpEngine{Type: "supkube", Instance: cluster},
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
	}
	resourceSet := map[string]bool{}
	addRes := func(nsName string) {
		if nsName != "" && !resourceSet[nsName] {
			resourceSet[nsName] = true
			out.Resources = append(out.Resources, drpResource{ID: nsName, Name: nsName, Kind: "namespace", App: nsName})
		}
	}

	// Schedules → jobs + policies
	for _, s := range schedules.Items {
		res := firstNS(s.Spec.Template.IncludedNamespaces)
		addRes(res)
		out.Jobs = append(out.Jobs, drpJob{
			ID: s.Name, Name: s.Name, ResourceID: res, PolicyID: s.Name,
			Enabled: !s.Spec.Paused, Schedule: s.Spec.Schedule,
		})
		out.Policies = append(out.Policies, drpPolicy{
			ID: s.Name, Name: s.Name,
			RetentionDays: ttlDays(s.Spec.Template.TTL),
			Immutable:     s.Annotations["supkube.io/object-lock"] == "true",
		})
	}

	// Backups → sessions + recovery points (+ failed-alerts)
	for _, b := range backups.Items {
		res := firstNS(b.Spec.IncludedNamespaces)
		addRes(res)
		sched := b.Labels["velero.io/schedule-name"]
		immutable := b.Annotations["supkube.io/object-lock"] == "true"
		out.Sessions = append(out.Sessions, drpSession{
			ID: b.Name, JobID: sched,
			StartedAt: fmtTime(b.Status.StartTimestamp), FinishedAt: fmtTime(b.Status.CompletionTimestamp),
			Status:          backupStatus(b.Status.Phase),
			DurationSeconds: durSecs(b.Status.StartTimestamp, b.Status.CompletionTimestamp),
		})
		if b.Status.CompletionTimestamp != nil {
			out.RecoveryPoints = append(out.RecoveryPoints, drpRecoveryPoint{
				ID: b.Name, JobID: sched, ResourceID: res,
				CreatedAt: fmtTime(b.Status.CompletionTimestamp), Immutable: immutable,
				RepositoryID: b.Spec.StorageLocation,
			})
		}
		if b.Status.Phase == velerov1.BackupPhaseFailed || b.Status.Phase == velerov1.BackupPhasePartiallyFailed {
			out.Alerts = append(out.Alerts, drpAlert{
				ID: b.Name, ResourceID: res, JobID: sched, Severity: "P1", Type: "BACKUP_JOB_FAILED",
				Message: b.Status.FailureReason, OccurredAt: fmtTime(b.Status.CompletionTimestamp),
			})
		}
	}

	// BSLs → repositories (+ unavailable-alerts)
	for _, l := range bsls.Items {
		bucket := ""
		if os := l.Spec.StorageType.ObjectStorage; os != nil {
			bucket = os.Bucket
		}
		out.Repositories = append(out.Repositories, drpRepo{
			ID: l.Name, Name: l.Spec.Provider + ":" + bucket, Type: "object-store",
			Immutable: l.Annotations["supkube.io/object-lock"] == "true",
		})
		if l.Status.Phase == velerov1.BackupStorageLocationPhaseUnavailable {
			out.Alerts = append(out.Alerts, drpAlert{
				ID: l.Name, Severity: "P1", Type: "REPOSITORY_UNAVAILABLE", Message: l.Status.Message,
			})
		}
	}

	// Restores
	for _, r := range restores.Items {
		out.Restores = append(out.Restores, drpRestore{
			ID: r.Name, RecoveryPointID: r.Spec.BackupName,
			StartedAt: fmtTime(r.Status.StartTimestamp), FinishedAt: fmtTime(r.Status.CompletionTimestamp),
			Status:          restoreStatus(r.Status.Phase),
			DurationSeconds: durSecs(r.Status.StartTimestamp, r.Status.CompletionTimestamp),
		})
	}

	// DRFlowRunner drills → ② 演练证据回流
	for _, run := range runs {
		st := "partial"
		switch run.Phase {
		case drflow.PhaseSucceeded:
			st = "passed"
		case drflow.PhaseFailed:
			st = "failed"
		}
		rto := 0
		if !run.UpdatedAt.IsZero() && !run.StartedAt.IsZero() {
			rto = int(run.UpdatedAt.Sub(run.StartedAt).Seconds())
		}
		out.Drills = append(out.Drills, drpDrill{
			ID: run.ID, Name: run.TargetApp, ResourceID: run.DrillNS,
			ExecutedAt: run.StartedAt.UTC().Format(time.RFC3339), Status: st,
			RTOSeconds: rto, Phases: []string{string(run.Phase)}, Evidence: run.KBCluster,
		})
		if run.Phase == drflow.PhaseFailed {
			out.Alerts = append(out.Alerts, drpAlert{
				ID: run.ID, ResourceID: run.DrillNS, Severity: "P1", Type: "DRILL_FAILED",
				Message: run.FailReason, OccurredAt: run.UpdatedAt.UTC().Format(time.RFC3339),
			})
		}
	}

	c.JSON(http.StatusOK, out)
}

// ---- helpers ----

func firstNS(ns []string) string {
	if len(ns) > 0 {
		return ns[0]
	}
	return ""
}
func fmtTime(t *metav1.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
func durSecs(start, end *metav1.Time) int {
	if start == nil || end == nil {
		return 0
	}
	return int(end.Sub(start.Time).Seconds())
}
func ttlDays(ttl metav1.Duration) int {
	return int(ttl.Duration.Hours() / 24)
}
func backupStatus(p velerov1.BackupPhase) string {
	switch p {
	case velerov1.BackupPhaseCompleted:
		return "success"
	case velerov1.BackupPhaseFailed, velerov1.BackupPhaseFailedValidation:
		return "failed"
	case velerov1.BackupPhasePartiallyFailed:
		return "warning"
	case velerov1.BackupPhaseInProgress:
		return "running"
	default:
		return string(p)
	}
}
func restoreStatus(p velerov1.RestorePhase) string {
	switch p {
	case velerov1.RestorePhaseCompleted:
		return "success"
	case velerov1.RestorePhaseFailed, velerov1.RestorePhaseFailedValidation:
		return "failed"
	case velerov1.RestorePhasePartiallyFailed:
		return "warning"
	default:
		return string(p)
	}
}

// ---- canonical output shape (JSON must match supinsight ingest.PostureSnapshot) ----

type drpEngine struct {
	Type     string `json:"type"`
	Version  string `json:"version"`
	Instance string `json:"instance"`
}
type drpResource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	App  string `json:"app"`
	Tier string `json:"tier"`
}
type drpPolicy struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RetentionDays int    `json:"retentionDays"`
	GFS           string `json:"gfs"`
	Immutable     bool   `json:"immutable"`
}
type drpJob struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ResourceID string `json:"resourceId"`
	PolicyID   string `json:"policyId"`
	Enabled    bool   `json:"enabled"`
	Schedule   string `json:"schedule"`
}
type drpSession struct {
	ID              string `json:"id"`
	JobID           string `json:"jobId"`
	StartedAt       string `json:"startedAt"`
	FinishedAt      string `json:"finishedAt"`
	Status          string `json:"status"`
	DurationSeconds int    `json:"durationSeconds"`
}
type drpRecoveryPoint struct {
	ID           string `json:"id"`
	JobID        string `json:"jobId"`
	ResourceID   string `json:"resourceId"`
	CreatedAt    string `json:"createdAt"`
	Immutable    bool   `json:"immutable"`
	RepositoryID string `json:"repositoryId"`
}
type drpRepo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Immutable bool   `json:"immutable"`
}
type drpRestore struct {
	ID              string `json:"id"`
	RecoveryPointID string `json:"recoveryPointId"`
	StartedAt       string `json:"startedAt"`
	FinishedAt      string `json:"finishedAt"`
	Status          string `json:"status"`
	DurationSeconds int    `json:"durationSeconds"`
}
type drpDrill struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	ResourceID string   `json:"resourceId"`
	ExecutedAt string   `json:"executedAt"`
	Status     string   `json:"status"`
	RTOSeconds int      `json:"rtoSeconds"`
	Phases     []string `json:"phases"`
	Evidence   string   `json:"evidence"`
}
type drpAlert struct {
	ID         string `json:"id"`
	ResourceID string `json:"resourceId"`
	JobID      string `json:"jobId"`
	Severity   string `json:"severity"`
	Type       string `json:"type"`
	Message    string `json:"message"`
	OccurredAt string `json:"occurredAt"`
}
type drPosture struct {
	Engine         drpEngine          `json:"engine"`
	Resources      []drpResource      `json:"resources"`
	Policies       []drpPolicy        `json:"policies"`
	Jobs           []drpJob           `json:"jobs"`
	Sessions       []drpSession       `json:"sessions"`
	RecoveryPoints []drpRecoveryPoint `json:"recoveryPoints"`
	Repositories   []drpRepo          `json:"repositories"`
	Restores       []drpRestore       `json:"restores"`
	Drills         []drpDrill         `json:"drills"`
	Alerts         []drpAlert         `json:"alerts"`
	CollectedAt    string             `json:"collectedAt"`
}
