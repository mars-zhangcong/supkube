package server

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/supkube/supkube-backend/internal/api/v1"
)

func Run() error {
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// API v1 routes
	api := r.Group("/api/v1")
	{
		api.GET("/status", v1.GetStatus)
		api.GET("/namespaces", v1.ListNamespaces)

		// Backups
		api.GET("/backups", v1.ListBackups)
		api.POST("/backups", v1.CreateBackup)
		api.GET("/backups/:name", v1.GetBackup)
		api.DELETE("/backups/:name", v1.DeleteBackup)

		// Restores
		api.GET("/restores", v1.ListRestores)
		api.POST("/restores", v1.CreateRestore)
		api.GET("/restores/:name", v1.GetRestore)

		// Schedules
		api.GET("/schedules", v1.ListSchedules)
		api.POST("/schedules", v1.CreateSchedule)
		api.DELETE("/schedules/:name", v1.DeleteSchedule)

		// Storage locations
		api.GET("/storage-locations", v1.ListStorageLocations)
		api.POST("/storage-locations", v1.CreateStorageLocation)
	}

	return r.Run(":8080")
}
