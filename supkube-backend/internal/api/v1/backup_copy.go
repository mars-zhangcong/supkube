// Package v1: POST /backup-copy/preflight — PRD-007 §4.3 Layer 4 Preflight.
// Phase 1 ship: Preflight 完整; Transfer 返回 501.
package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/supkube/supkube-backend/internal/backupcopy"
	"github.com/supkube/supkube-backend/internal/k8s"
)

// BackupCopyPreflight POST /backup-copy/preflight.
// 检查 source BSL 上的 backup 哪些可走 Layer 4, 哪些被拦截 (含具体错误码 + 修复指引).
// Editor role; whole-cluster bypass 同 backup delete 规则.
func BackupCopyPreflight(c *gin.Context) {
	var req backupcopy.PreflightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.SourceBSL == "" || req.TargetBSL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sourceBSL and targetBSL are required"})
		return
	}
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	res, err := backupcopy.Preflight(c.Request.Context(), cl, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// CreateBackupCopy POST /backup-copy — Phase 2 真 Transfer (rclone sync).
// Phase 1 stub: 返回 501 Not Implemented + 引用 PRD/ADR.
func CreateBackupCopy(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Layer 4 Transfer not implemented in Phase 1",
		"phase": "Phase 1 ship: Preflight only. Phase 2: rclone sync + Activity Timeline 联动.",
		"ref":   []string{"PRD-007 §4.3", "ADR-031 §8", "engineer-testing/fixtures/velero-real-2026-05-31-060756/"},
	})
}
