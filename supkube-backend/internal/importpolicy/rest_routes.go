// rest_routes.go — gin route 注册 + RBAC table 扩展.
//
// cmd/server/server.go startup 调一次 RegisterRoutes; 不会重复挂载.
// RBAC entries 通过 auth.RegisterPermissions 在 init() 注入 — 这样
// auth/rbac.go 的 permissionTable 不用直接 import importpolicy 包 (避免
// 循环依赖).
//
// 全部 endpoints 设为 RoleEditor (操作员级别) — 与 Schedules / Backups
// 同档; 读则 RoleViewer. Admin 也满足 (IsAtLeast).
package importpolicy

import (
	"github.com/gin-gonic/gin"

	"github.com/supkube/supkube-backend/internal/auth"
)

// RegisterRoutes 把 ImportPolicy 的 HTTP 路由挂到给定 api group, 并同步
// 把 RBAC 条目注册进 auth.permissionTable.
//
// Caller (cmd/server/server.go):
//
//	importpolicy.RegisterRoutes(api)
//
// 必须在 cmd/server 已经 RunController + RegisterController 之后调.
func RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/import-policies", ListImportPolicies)
	api.POST("/import-policies", CreateImportPolicy)
	api.GET("/import-policies/:name", GetImportPolicy)
	api.PUT("/import-policies/:name", UpdateImportPolicy)
	api.DELETE("/import-policies/:name", DeleteImportPolicy)
	api.POST("/import-policies/:name/run-once", RunOnce)
	api.POST("/import-policies/:name/pause", PauseImportPolicy)
	api.POST("/import-policies/:name/resume", ResumeImportPolicy)

	auth.RegisterPermissions(
		auth.PermEntry{Method: "GET", Pattern: "/api/v1/import-policies", MinRole: auth.RoleViewer},
		auth.PermEntry{Method: "GET", Pattern: "/api/v1/import-policies/:name", MinRole: auth.RoleViewer},
		auth.PermEntry{Method: "POST", Pattern: "/api/v1/import-policies", MinRole: auth.RoleEditor},
		auth.PermEntry{Method: "PUT", Pattern: "/api/v1/import-policies/:name", MinRole: auth.RoleEditor},
		auth.PermEntry{Method: "DELETE", Pattern: "/api/v1/import-policies/:name", MinRole: auth.RoleEditor},
		auth.PermEntry{Method: "POST", Pattern: "/api/v1/import-policies/:name/run-once", MinRole: auth.RoleEditor},
		auth.PermEntry{Method: "POST", Pattern: "/api/v1/import-policies/:name/pause", MinRole: auth.RoleEditor},
		auth.PermEntry{Method: "POST", Pattern: "/api/v1/import-policies/:name/resume", MinRole: auth.RoleEditor},
	)
}
