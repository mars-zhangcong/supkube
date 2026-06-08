// Package drflow implements ADR-053 DRFlowRunner — a K8s-native 5-stage
// linear DR drill orchestrator.
//
// # Phases (linear, no skip)
//
//	Pending → RestoringDB → RestoringApp → Realigning → Validating → Succeeded
//	         \________________________________________/
//	                    any phase → Failed{phase, reason}
//
// # State persistence
//
// DRFlow state is stored in a K8s ConfigMap named "drflow-state-<runID>"
// in the supkube namespace. ConfigMaps are the canonical K8s-native store
// for lightweight non-secret state (vs SQLite which the rest of SupKube
// doesn't use). A new run creates the CM; completion leaves it for history.
//
// # Timeline / Activity events
//
// Phase transitions emit K8s Events with label supkube.io/activity=true.
// SupKube's existing Activity page reads this label automatically — no
// schema or API changes required (per ADR-053 D5).
//
// # Gate conditions (ADR-053 D3)
//
//	RestoringDB   : KB Cluster CR phase=Running + svc DNS resolves + KB Secret ready
//	RestoringApp  : App Pod phase=Running + all containers Ready
//	Realigning    : App→DB TCP probe (SELECT 1 via psql or TCP dial)
//	Validating    : SELECT 1 + row count/checksum + ≥1 negative deviation injection
//	Succeeded     : all validations pass
package drflow

import (
	"fmt"
	"time"
)

const (
	Namespace      = "supkube"
	cmPrefix       = "drflow-state-"
	labelActivity  = "supkube.io/activity"
	labelComponent = "supkube.io/component"
	labelRunID     = "supkube.io/drflow-run-id"
	labelPhase     = "supkube.io/drflow-phase"
	reportingCtrl  = "supkube.io/drflow"
	reportingInst  = "supkube-backend"
)

// Phase represents the current stage of a DRFlow run.
type Phase string

const (
	PhasePending      Phase = "Pending"
	PhaseRestoringDB  Phase = "RestoringDB"
	PhaseRestoringApp Phase = "RestoringApp"
	PhaseRealigning   Phase = "Realigning"
	PhaseValidating   Phase = "Validating"
	PhaseSucceeded    Phase = "Succeeded"
	PhaseFailed       Phase = "Failed"
)

// Run holds the complete state of one DRFlow drill run.
type Run struct {
	ID          string    `json:"id"`
	Phase       Phase     `json:"phase"`
	FailedPhase string    `json:"failed_phase,omitempty"` // set when Phase=Failed
	FailReason  string    `json:"fail_reason,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// DrillNS is the namespace where KB Cluster + App are restored.
	DrillNS string `json:"drill_ns"`
	// TargetApp is the label selector for the app pod.
	TargetApp string `json:"target_app"`
	// KBCluster is the KubeBlocks Cluster CR name in DrillNS.
	KBCluster string `json:"kb_cluster"`
	// DBSecret is the K8s Secret name with DB credentials in DrillNS.
	DBSecret string `json:"db_secret"`
	// BackupName is the KB Backup CR name used by the RestoringDB trigger.
	BackupName string `json:"backup_name,omitempty"`
	// BackupNamespace is the namespace of the Backup CR (defaults to DrillNS).
	BackupNamespace string `json:"backup_namespace,omitempty"`
	// DryRun disables trigger side-effects (KB Restore, Adminer deploy) for gate testing.
	DryRun bool `json:"dry_run,omitempty"`
}

// Trigger is the input to POST /api/drflow.
type Trigger struct {
	DrillNS         string `json:"drill_ns"`
	TargetApp       string `json:"target_app"`
	KBCluster       string `json:"kb_cluster"`
	DBSecret        string `json:"db_secret"`
	BackupName      string `json:"backup_name"`
	BackupNamespace string `json:"backup_namespace,omitempty"` // defaults to DrillNS
	// DryRun skips trigger functions (KB Restore, Adminer deploy) while still
	// polling gates. Allows verifying gate structure without real K8s mutations.
	DryRun bool `json:"dry_run,omitempty"`
}

// Validate returns an error if required fields are missing.
func (t Trigger) Validate() error {
	if t.DrillNS == "" {
		return fmt.Errorf("drill_ns required")
	}
	if t.KBCluster == "" {
		return fmt.Errorf("kb_cluster required")
	}
	if t.TargetApp == "" {
		return fmt.Errorf("target_app required")
	}
	if t.DBSecret == "" {
		return fmt.Errorf("db_secret required")
	}
	if !t.DryRun && t.BackupName == "" {
		return fmt.Errorf("backup_name required (or set dry_run=true to skip restore trigger)")
	}
	return nil
}

// cmName returns the ConfigMap name for a given run ID.
func cmName(runID string) string { return cmPrefix + runID }
