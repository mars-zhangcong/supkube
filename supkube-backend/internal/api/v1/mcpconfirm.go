package v1

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/mcpconfirm"
)

// MCP HitL confirmation persistence (ADR-057 R1). These are INTERNAL endpoints
// the supkube-mcp server calls so it stays a pure backend client (no K8s deps).
// Snapshots persist as cross-replica K8s Secrets via mcpconfirm.Store.

// ConfirmNamespace is where confirmation Secrets live (env MCP_CONFIRM_NAMESPACE,
// else the pod's SA namespace, else "default"). Exported for the reaper.
func ConfirmNamespace() string {
	if v := strings.TrimSpace(os.Getenv("MCP_CONFIRM_NAMESPACE")); v != "" {
		return v
	}
	if b, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(b)); ns != "" {
			return ns
		}
	}
	return "default"
}

func newConfirmStore(ttl time.Duration) (*mcpconfirm.Store, error) {
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		return nil, err
	}
	return mcpconfirm.New(cl, ConfirmNamespace(), ttl), nil
}

// CreateMCPConfirmation persists a confirmation snapshot and returns its confirm_id.
func CreateMCPConfirmation(c *gin.Context) {
	var req struct {
		Skill      string         `json:"skill"`
		Args       map[string]any `json:"args"`
		DryRun     any            `json:"dryRun"`
		TTLSeconds int            `json:"ttlSeconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Skill) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill is required"})
		return
	}
	ttl := mcpconfirm.DefaultTTL
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}
	st, err := newConfirmStore(ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, err := st.Put(c.Request.Context(), mcpconfirm.Snapshot{Skill: req.Skill, Args: req.Args, DryRun: req.DryRun})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"confirmId": id})
}

// GetMCPConfirmation returns a snapshot by id, or 404 if missing/expired.
func GetMCPConfirmation(c *gin.Context) {
	st, err := newConfirmStore(0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	snap, ok, err := st.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "confirmation not found or expired"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"skill": snap.Skill, "args": snap.Args, "dryRun": snap.DryRun})
}

// DeleteMCPConfirmation consumes a confirmation (one-shot).
func DeleteMCPConfirmation(c *gin.Context) {
	st, err := newConfirmStore(0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := st.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
