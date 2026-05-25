// Package v1: HTTP handlers for the v0.8.8 Cluster Hygiene feature.
//
// Endpoints mounted under /api/v1:
//
//   GET  /settings/cleanup            — read current GC settings + last run
//   PUT  /settings/cleanup            — admin: update settings
//   POST /admin/cleanup/orphans       — admin: trigger immediate scan
//
// All three endpoints are admin-only; RBAC enforcement lives in
// internal/auth/rbac.go (added in the same release).
package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/supkube/supkube-backend/internal/gc"
	"github.com/supkube/supkube-backend/internal/k8s"
)

// settingsResponse is the JSON shape returned by GET /settings/cleanup.
// We include both the persisted settings AND a "lastRun" sub-object so
// the Settings UI can render a "Last run: X minutes ago, deleted Y" line
// without a second request.
type settingsResponse struct {
	Enabled       bool             `json:"enabled"`
	IntervalHours int              `json:"intervalHours"`
	LastRun       *lastRunPayload  `json:"lastRun,omitempty"`
}

type lastRunPayload struct {
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`
	Summary     string    `json:"summary"`
	VSCDeleted  int       `json:"vscDeleted"`
	VSDeleted   int       `json:"vsDeleted"`
	PVBDeleted  int       `json:"pvbDeleted"`
	DUDeleted   int       `json:"dataUploadDeleted"`
	Err         string    `json:"error,omitempty"`
}

// scanResultToPayload converts the gc package's internal result type
// into the JSON-friendly response. Returns nil when there's no
// recorded run yet (zero time means "never ran").
func scanResultToPayload(r gc.ScanResult) *lastRunPayload {
	if r.FinishedAt.IsZero() {
		return nil
	}
	p := &lastRunPayload{
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		Summary:    r.Summary(),
		VSCDeleted: r.VSCDeleted,
		VSDeleted:  r.VSDeleted,
		PVBDeleted: r.PVBDeleted,
		DUDeleted:  r.DUDeleted,
	}
	if r.Err != nil {
		p.Err = r.Err.Error()
	}
	return p
}

// GetCleanupSettings returns the current GC config + last run summary.
// Used by the Settings page on every open + after a manual cleanup
// click to show updated last-run state.
func GetCleanupSettings(c *gin.Context) {
	k8sCli, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s := gc.LoadSettings(c.Request.Context(), k8sCli)
	c.JSON(http.StatusOK, settingsResponse{
		Enabled:       s.Enabled,
		IntervalHours: s.IntervalHours,
		LastRun:       scanResultToPayload(gc.LastRun()),
	})
}

// UpdateCleanupSettings persists user-edited settings to the ConfigMap.
// Admin-only (gated by the RBAC table).
//
// Validation: intervalHours must be in [1, 168]. Lower bound prevents
// effectively-continuous scans (which would hammer the K8s API);
// upper bound (1 week) prevents footgun "I set it to 9999 by accident
// and orphans piled up for a year".
func UpdateCleanupSettings(c *gin.Context) {
	var req struct {
		Enabled       *bool `json:"enabled"`
		IntervalHours *int  `json:"intervalHours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	k8sCli, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Load existing + overlay incoming fields. Lets the UI PATCH only
	// the field that changed without remembering the other.
	s := gc.LoadSettings(c.Request.Context(), k8sCli)
	if req.Enabled != nil {
		s.Enabled = *req.Enabled
	}
	if req.IntervalHours != nil {
		if *req.IntervalHours < 1 || *req.IntervalHours > 168 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "intervalHours must be between 1 and 168 (1 week)"})
			return
		}
		s.IntervalHours = *req.IntervalHours
	}
	if err := gc.SaveSettings(c.Request.Context(), k8sCli, s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settingsResponse{
		Enabled:       s.Enabled,
		IntervalHours: s.IntervalHours,
		LastRun:       scanResultToPayload(gc.LastRun()),
	})
}

// RunOrphanCleanup triggers an immediate scan + delete pass.
// Admin-only. Emits an Activity event (visible in /activity).
//
// Synchronous: the response carries the ScanResult so the UI can show
// "deleted X VSCs" right after the button click without polling. Scans
// over a clean cluster take <500ms; even with thousands of orphans we
// stay well under HTTP timeout.
func RunOrphanCleanup(c *gin.Context) {
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	k8sCli, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Use a 60s context cap — protective against a stuck K8s API.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	r := gc.RunOnce(ctx, cl, k8sCli, "manual")
	c.JSON(http.StatusOK, gin.H{
		"summary": r.Summary(),
		"result":  scanResultToPayload(r),
	})
}
