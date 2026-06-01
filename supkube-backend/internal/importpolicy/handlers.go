// handlers.go — gin REST handlers for ImportPolicy CR.
//
// API surface:
//
//	GET    /api/v1/import-policies            list
//	POST   /api/v1/import-policies            create (validate cron/duration)
//	GET    /api/v1/import-policies/:name      get one
//	PUT    /api/v1/import-policies/:name      update spec
//	DELETE /api/v1/import-policies/:name      delete (controller goroutine 关闭)
//	POST   /api/v1/import-policies/:name/run-once   立即触发一次 syncOnce
//	POST   /api/v1/import-policies/:name/pause      patch spec.paused=true
//	POST   /api/v1/import-policies/:name/resume     patch spec.paused=false
//
// 错误响应严格 ADR-035: {"error": "<人话>", "code": "ERR_IMPORTPOLICY_XXX"}
//
// handler 不持有 controller; 通过 module-level singleton (RegisterController)
// 拿 controller 引用. cmd/server startup 先 RunController (异步) 再
// RegisterController + RegisterRoutes — handler 调 syncOnce 直接转 controller.
package importpolicy

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// Error codes (ADR-035). 全部以 ERR_IMPORTPOLICY_ 前缀, 便于前端 i18n 表
// 集中收敛. ERR_FINGERPRINT_* 由 fingerprint 模块定义, handler 转发即可.
const (
	ErrBSLNotFound      = "ERR_IMPORTPOLICY_BSL_NOTFOUND"
	ErrCronInvalid      = "ERR_IMPORTPOLICY_CRON_INVALID"
	ErrIntervalTooShort = "ERR_IMPORTPOLICY_INTERVAL_TOO_SHORT"
	ErrInvalidName      = "ERR_IMPORTPOLICY_INVALID_NAME"
	ErrInvalidMode      = "ERR_IMPORTPOLICY_INVALID_MODE"
	ErrInvalidFpMode    = "ERR_IMPORTPOLICY_INVALID_FINGERPRINT_MODE"
	ErrNotFound         = "ERR_IMPORTPOLICY_NOT_FOUND"
	ErrAlreadyExists    = "ERR_IMPORTPOLICY_ALREADY_EXISTS"
	ErrBadRequest       = "ERR_IMPORTPOLICY_BAD_REQUEST"
	ErrInternal         = "ERR_IMPORTPOLICY_INTERNAL"
)

// nameRe — 与 cluster CR 同规则 (DNS-1123 label).
var nameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// ─────────────────────────────────────────────────────────────────────
// handler-side state — controller singleton 注入
// ─────────────────────────────────────────────────────────────────────

var (
	mu             sync.RWMutex
	registeredCtrl *Controller
	registeredDyn  dynamic.Interface
	bslExists      func(ctx context.Context, name string) (bool, error)
)

// RegisterController 让 handler 知道去哪儿调 SyncOnce. 在 cmd/server
// startup 调一次; 没注册时 run-once endpoint 返回 503.
func RegisterController(c *Controller, dynCli dynamic.Interface, bslCheck func(ctx context.Context, name string) (bool, error)) {
	mu.Lock()
	defer mu.Unlock()
	registeredCtrl = c
	registeredDyn = dynCli
	bslExists = bslCheck
}

func getCtrl() (*Controller, dynamic.Interface) {
	mu.RLock()
	defer mu.RUnlock()
	return registeredCtrl, registeredDyn
}

// ─────────────────────────────────────────────────────────────────────
// DTOs
// ─────────────────────────────────────────────────────────────────────

// ImportPolicyDTO 是返回给 SPA 的扁平形状. 把 spec/status 拍平, 但 status
// 嵌套保留 (status 字段都有意义).
type ImportPolicyDTO struct {
	Name         string             `json:"name"`
	CreationTime time.Time          `json:"creationTime,omitempty"`
	Spec         ImportPolicySpec   `json:"spec"`
	Status       ImportPolicyStatus `json:"status"`
}

// CreateRequest 是 POST body.
type CreateRequest struct {
	Name string           `json:"name" binding:"required"`
	Spec ImportPolicySpec `json:"spec" binding:"required"`
}

// UpdateRequest 是 PUT body (只能改 spec).
type UpdateRequest struct {
	Spec ImportPolicySpec `json:"spec" binding:"required"`
}

// ─────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────

// ListImportPolicies — GET /api/v1/import-policies
func ListImportPolicies(c *gin.Context) {
	_, dyn := getCtrl()
	if dyn == nil {
		writeErr(c, http.StatusServiceUnavailable, ErrInternal, "controller not initialized")
		return
	}
	list, err := dyn.Resource(GVR).Namespace(Namespace).List(c.Request.Context(), metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			c.JSON(http.StatusOK, gin.H{"items": []ImportPolicyDTO{}})
			return
		}
		writeErr(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	items := make([]ImportPolicyDTO, 0, len(list.Items))
	for i := range list.Items {
		p, err := fromUnstructured(&list.Items[i])
		if err != nil {
			continue // 跳过坏 CR, 不让一个污染整个 list
		}
		items = append(items, toDTO(p))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetImportPolicy — GET /api/v1/import-policies/:name
func GetImportPolicy(c *gin.Context) {
	_, dyn := getCtrl()
	if dyn == nil {
		writeErr(c, http.StatusServiceUnavailable, ErrInternal, "controller not initialized")
		return
	}
	name := c.Param("name")
	cr, err := dyn.Resource(GVR).Namespace(Namespace).Get(c.Request.Context(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeErr(c, http.StatusNotFound, ErrNotFound, "import policy not found")
			return
		}
		writeErr(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	p, err := fromUnstructured(cr)
	if err != nil {
		writeErr(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	c.JSON(http.StatusOK, toDTO(p))
}

// CreateImportPolicy — POST /api/v1/import-policies
func CreateImportPolicy(c *gin.Context) {
	_, dyn := getCtrl()
	if dyn == nil {
		writeErr(c, http.StatusServiceUnavailable, ErrInternal, "controller not initialized")
		return
	}
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, http.StatusBadRequest, ErrBadRequest, err.Error())
		return
	}
	if !nameRe.MatchString(req.Name) {
		writeErr(c, http.StatusBadRequest, ErrInvalidName, "name must be DNS-1123 label")
		return
	}
	if code, msg, ok := validateSpec(c.Request.Context(), &req.Spec); !ok {
		writeErr(c, http.StatusBadRequest, code, msg)
		return
	}
	defaultsForSpec(&req.Spec)
	cr := toUnstructured(req.Name, req.Spec)
	created, err := dyn.Resource(GVR).Namespace(Namespace).Create(c.Request.Context(), cr, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			writeErr(c, http.StatusConflict, ErrAlreadyExists, "import policy '"+req.Name+"' already exists")
			return
		}
		writeErr(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	p, _ := fromUnstructured(created)
	c.JSON(http.StatusCreated, toDTO(p))
}

// UpdateImportPolicy — PUT /api/v1/import-policies/:name
func UpdateImportPolicy(c *gin.Context) {
	_, dyn := getCtrl()
	if dyn == nil {
		writeErr(c, http.StatusServiceUnavailable, ErrInternal, "controller not initialized")
		return
	}
	name := c.Param("name")
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErr(c, http.StatusBadRequest, ErrBadRequest, err.Error())
		return
	}
	if code, msg, ok := validateSpec(c.Request.Context(), &req.Spec); !ok {
		writeErr(c, http.StatusBadRequest, code, msg)
		return
	}
	defaultsForSpec(&req.Spec)
	// Patch spec only (保留 status, metadata.labels, owner annotations 不被
	// 覆盖). merge patch on .spec 是替换整个 object — 我们要的就是这个语
	// 义 (UI Edit 表单提交完整 spec).
	patch := map[string]interface{}{"spec": specToMap(req.Spec)}
	body, _ := json.Marshal(patch)
	updated, err := dyn.Resource(GVR).Namespace(Namespace).Patch(
		c.Request.Context(), name, types.MergePatchType, body, metav1.PatchOptions{},
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeErr(c, http.StatusNotFound, ErrNotFound, "import policy not found")
			return
		}
		writeErr(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	p, _ := fromUnstructured(updated)
	c.JSON(http.StatusOK, toDTO(p))
}

// DeleteImportPolicy — DELETE /api/v1/import-policies/:name
//
// controller manager loop 下次 tick (≤30s) 会发现 CR 不在 list 里, 自动
// stopRunner. 这里不直接调 stopRunner 因为 handler 不应该感知 manager
// 的内部状态.
func DeleteImportPolicy(c *gin.Context) {
	_, dyn := getCtrl()
	if dyn == nil {
		writeErr(c, http.StatusServiceUnavailable, ErrInternal, "controller not initialized")
		return
	}
	name := c.Param("name")
	if err := dyn.Resource(GVR).Namespace(Namespace).Delete(c.Request.Context(), name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			writeErr(c, http.StatusNotFound, ErrNotFound, "import policy not found")
			return
		}
		writeErr(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// RunOnce — POST /api/v1/import-policies/:name/run-once
func RunOnce(c *gin.Context) {
	ctrl, _ := getCtrl()
	if ctrl == nil {
		writeErr(c, http.StatusServiceUnavailable, ErrInternal, "controller not initialized")
		return
	}
	name := c.Param("name")
	res, err := ctrl.SyncOnce(c.Request.Context(), name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeErr(c, http.StatusNotFound, ErrNotFound, "import policy not found")
			return
		}
		writeErr(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"importedCount": res.ImportedCount,
		"rejectedCount": res.RejectedCount,
		"newBackups":    res.NewBackups,
		"errors":        res.Errors,
	})
}

// PauseImportPolicy — POST /api/v1/import-policies/:name/pause
func PauseImportPolicy(c *gin.Context) {
	patchPaused(c, true)
}

// ResumeImportPolicy — POST /api/v1/import-policies/:name/resume
func ResumeImportPolicy(c *gin.Context) {
	patchPaused(c, false)
}

func patchPaused(c *gin.Context, paused bool) {
	_, dyn := getCtrl()
	if dyn == nil {
		writeErr(c, http.StatusServiceUnavailable, ErrInternal, "controller not initialized")
		return
	}
	name := c.Param("name")
	patch := map[string]interface{}{"spec": map[string]interface{}{"paused": paused}}
	body, _ := json.Marshal(patch)
	updated, err := dyn.Resource(GVR).Namespace(Namespace).Patch(
		c.Request.Context(), name, types.MergePatchType, body, metav1.PatchOptions{},
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeErr(c, http.StatusNotFound, ErrNotFound, "import policy not found")
			return
		}
		writeErr(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	p, _ := fromUnstructured(updated)
	c.JSON(http.StatusOK, toDTO(p))
}

// ─────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────

// validateSpec 检查所有跨字段约束. 返回 (errorCode, humanMsg, ok).
//
// 顺序: mode 合法 → mode 对应字段合法 → fingerprintMode 合法 → BSL 存在.
// BSL 校验放最后是因为最贵 (要查 K8s API).
func validateSpec(ctx context.Context, s *ImportPolicySpec) (string, string, bool) {
	switch s.Mode {
	case ImportModeContinuous:
		// 解析 interval; 默认 60s.
		raw := s.ContinuousInterval
		if raw == "" {
			raw = "60s"
		}
		d, err := time.ParseDuration(raw)
		if err != nil {
			return ErrBadRequest, "continuousInterval not a valid Go duration: " + err.Error(), false
		}
		if d < minContinuousInterval {
			return ErrIntervalTooShort, "continuousInterval must be >= 30s", false
		}
	case ImportModeScheduled:
		if s.Schedule == "" {
			return ErrCronInvalid, "schedule is required when mode=Scheduled", false
		}
		if _, err := ParseCron(s.Schedule); err != nil {
			return ErrCronInvalid, err.Error(), false
		}
	case "":
		return ErrInvalidMode, "mode is required (Continuous|Scheduled)", false
	default:
		return ErrInvalidMode, "mode must be Continuous or Scheduled, got: " + string(s.Mode), false
	}
	switch s.FingerprintMode {
	case "", FingerprintModeEnforce, FingerprintModeWarn, FingerprintModeDisabled:
		// ok
	default:
		return ErrInvalidFpMode, "fingerprintMode must be enforce|warn|disabled", false
	}
	if s.SourceBSL == "" {
		return ErrBadRequest, "sourceBSL is required", false
	}
	// BSL 存在性校验 — 通过注入的 bslExists 回调走 (生产是查 Velero BSL
	// CR; 单测可以注入 always-true). 没注册 bslExists 则跳过 (向后兼容).
	mu.RLock()
	checker := bslExists
	mu.RUnlock()
	if checker != nil {
		ok, err := checker(ctx, s.SourceBSL)
		if err != nil {
			// 查不到不一定是不存在; 网络错误也算. 偏严 — 返回 not found.
			return ErrBSLNotFound, "verify BSL: " + err.Error(), false
		}
		if !ok {
			return ErrBSLNotFound, "BSL '" + s.SourceBSL + "' not found in velero namespace", false
		}
	}
	return "", "", true
}

// defaultsForSpec 把缺省值填到 spec (写 CR 之前调). 让 CR 写到 API
// server 时字段已经"具象", admin kubectl get -o yaml 看到的就是实际生效
// 的值, 不是 "(empty=default)".
func defaultsForSpec(s *ImportPolicySpec) {
	if s.ContinuousInterval == "" && s.Mode == ImportModeContinuous {
		s.ContinuousInterval = "60s"
	}
	if s.FingerprintMode == "" {
		s.FingerprintMode = FingerprintModeEnforce
	}
	if s.TargetVeleroNamespace == "" {
		s.TargetVeleroNamespace = defaultTargetVeleroNamespace
	}
}

// toDTO 是 ImportPolicy -> DTO (扁平化).
func toDTO(p *ImportPolicy) ImportPolicyDTO {
	return ImportPolicyDTO{
		Name:         p.Name,
		CreationTime: p.CreationTimestamp.Time,
		Spec:         p.Spec,
		Status:       p.Status,
	}
}

// toUnstructured 是 (name, spec) -> *unstructured.Unstructured (create 用).
func toUnstructured(name string, spec ImportPolicySpec) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": GroupName + "/" + Version,
			"kind":       Kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": Namespace,
			},
			"spec": specToMap(spec),
		},
	}
}

// specToMap 把 ImportPolicySpec 序列化成 map[string]interface{} (k8s
// unstructured 要求). 走 json round-trip 偷懒 — 字段都是 JSON-tagged.
func specToMap(s ImportPolicySpec) map[string]interface{} {
	b, _ := json.Marshal(s)
	m := map[string]interface{}{}
	_ = json.Unmarshal(b, &m)
	return m
}

// writeErr 是 ADR-035 统一错误响应辅助.
func writeErr(c *gin.Context, status int, code, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg, "code": code})
}
