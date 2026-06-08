package drflow

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// RegisterRoutes wires the DRFlow HTTP endpoints into a Gin router group.
//
//	POST   /api/v1/drflow           — trigger a new drill run
//	GET    /api/v1/drflow           — list all runs (newest-first)
//	GET    /api/v1/drflow/:id       — single run detail
func RegisterRoutes(rg *gin.RouterGroup, k8sCli kubernetes.Interface, dynCli dynamic.Interface) {
	rg.POST("/drflow", handleTrigger(k8sCli, dynCli))
	rg.GET("/drflow", handleList(k8sCli))
	rg.GET("/drflow/:id", handleGet(k8sCli))
}

func handleTrigger(k8sCli kubernetes.Interface, dynCli dynamic.Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		var t Trigger
		if err := c.ShouldBindJSON(&t); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		run, err := StartRun(c.Request.Context(), k8sCli, dynCli, t)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, run)
	}
}

func handleList(k8sCli kubernetes.Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		runs, err := ListRuns(c.Request.Context(), k8sCli)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, runs)
	}
}

func handleGet(k8sCli kubernetes.Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		run, err := GetRun(c.Request.Context(), k8sCli, c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if run == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
			return
		}
		c.JSON(http.StatusOK, run)
	}
}
