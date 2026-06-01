// Package v1: GET /storage/csi-autoconfig — status of the CSI auto-
// configuration controller (see internal/csi/autoconfig.go for the
// underlying logic + design rationale).
//
// The handler is a thin read of the controller's in-memory status; no
// K8s calls happen in the request path. UI (Settings → Storage) polls
// this every few seconds while the customer is on that tab.
package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/supkube/supkube-backend/internal/csi"
)

// GetCSIAutoConfigStatus returns the last reconcile result of the
// CSI auto-config controller — which drivers are installed, which VSCs
// were created (vs. left alone), and which drivers SupKube doesn't yet
// have a recipe for. If the controller hasn't run yet (boot race), the
// `lastReconcileAt` is zero and the lists are empty.
func GetCSIAutoConfigStatus(c *gin.Context) {
	c.JSON(http.StatusOK, csi.GetStatus())
}
