package v1

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// PatchSchedule updates a schedule (e.g., pause/resume)
func PatchSchedule(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Paused *bool `json:"paused"`
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
	schedule := &velerov1.Schedule{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, schedule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if req.Paused != nil {
		schedule.Spec.Paused = *req.Paused
	}
	if err := cl.Update(context.Background(), schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, schedule)
}
