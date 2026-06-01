// Package v1: transform_sets_preview.go — POST /transform-sets/:name/preview-resolution
//
// PRD-002 v1.3 §4.3 (T1 编译契约抓手).
//
// 为什么单独一个 endpoint:
//   - Trigger Restore 前客户想看清楚 "我这一组 TransformSet+params 到底会被
//     Velero 应用成什么样的 patches". 直接打开 Restore 看 effective rules
//     需要 Restore 已经跑了, 晚了 (失败也回不去看).
//   - dry-run 模式: 不真 Create 派生 CM, 只算 effective YAML + 命名, 返回给
//     UI 渲染 review 面板. 安全, viewer 角色可用.
//
// 走的是 transform.CompileDryRun (Compile 的 sibling, 不真 Create CM).
package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/transform"
)

// PreviewResolutionRequest 是 POST body.
type PreviewResolutionRequest struct {
	// Params 是 ${VAR} 注入值. nil 或空 map 都 OK (TransformSet.defaults 兜底).
	Params map[string]string `json:"params"`
}

// PreviewResolutionResponse 是 200 OK body.
type PreviewResolutionResponse struct {
	// EffectiveRulesYAML 是展平 + 替换后的 rules.yaml, 客户看的就是这串.
	EffectiveRulesYAML string `json:"effectiveRulesYaml"`
	// DerivedCMName 是若真 Compile 会产生 (或复用) 的 CM 命名.
	DerivedCMName string `json:"derivedCMName"`
	// Hash 是命名里的 12 char hash, 客户对比两次 preview 是否同输入用.
	Hash string `json:"hash"`
	// AppliedRefs 是按 transformRefs 顺序排好的 Transform 名.
	AppliedRefs []string `json:"appliedRefs"`
}

// PreviewTransformSetResolution — 不创建 CM, 只算 + 返回, 让客户审 effective patches.
// Viewer 角色可用 (安全, 仅返回计算结果).
//
// Errors:
//   - 400: TransformSet 不存在 / Transform 不存在 / 缺 params / 不安全 param 值
//   - 500: K8s API 不可达
func PreviewTransformSetResolution(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "TransformSet name required in URL path"})
		return
	}
	var req PreviewResolutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Empty body is OK (params optional) — only fail on malformed JSON.
		// gin reports io.EOF on empty body as err; we tolerate that.
		if err.Error() != "EOF" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	res, err := transform.CompileDryRun(c.Request.Context(), cl, name, req.Params)
	if err != nil {
		// 编译错误 (缺 params / Transform 不存在 / yaml broken) 都归 400 — 客户输入问题.
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, PreviewResolutionResponse{
		EffectiveRulesYAML: res.RulesYAML,
		DerivedCMName:      res.ConfigMap.Name,
		Hash:               res.Hash,
		AppliedRefs:        res.AppliedRefs,
	})
}
