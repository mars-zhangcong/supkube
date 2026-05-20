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

// GetSchedule returns one Velero Schedule (the Kasten-style View drawer
// shows the raw CR; ListSchedules also has the data, but a dedicated GET
// keeps the URL stable for sharing / browser refresh).
func GetSchedule(c *gin.Context) {
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
	c.JSON(http.StatusOK, schedule)
}

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

// RunScheduleOnce triggers an ad-hoc backup using the schedule's template,
// bypassing the cron. Naming convention: <schedule>-<ts>, matching Velero's
// own naming from scheduled runs so the resulting Backup is recognizable.
// Velero's controller does NOT have a built-in "run now" verb; the standard
// workaround is to create a Backup CR directly from the schedule template,
// with the same velero.io/schedule-name label so it's attributed to the
// policy in the UI and inherits the retention model.
func RunScheduleOnce(c *gin.Context) {
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

	// Backup name: <schedule>-<YYYYMMDDhhmmss>, identical scheme to Velero's
	// scheduled runs so users get a coherent listing.
	backupName := fmt.Sprintf("%s-%s", schedule.Name, time.Now().UTC().Format("20060102150405"))

	labels := map[string]string{
		"velero.io/schedule-name": schedule.Name,
		"supkube.io/run-trigger":  "manual",
	}
	for k, v := range schedule.Labels {
		labels[k] = v
	}

	backup := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: "velero",
			Labels:    labels,
			Annotations: map[string]string{
				"supkube.io/triggered-by": "policy-run-once",
			},
		},
		Spec: *schedule.Spec.Template.DeepCopy(),
	}
	if err := cl.Create(context.Background(), backup); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"backupName": backupName, "scheduleName": schedule.Name})
}
