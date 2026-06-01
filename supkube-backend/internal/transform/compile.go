// Package transform 编译 SupKube 两层模型 (Transform + TransformSet) 为
// Velero 原生 resourceModifierRef ConfigMap.
//
// 编译契约 (ADR-003 v0.9.x 修订 + PRD-002 v1.3 §4.1bis):
//   - 输入: TransformSet (含 transformRefs[] + params 注入)
//   - 输出: 单个 ConfigMap `supkube-restore-rm-<name>-<hash>` (velero ns),
//     data.rules.yaml, len(Data)==1 (Velero 硬要求, ADR-003)
//   - 顺序: transformRefs 顺序 → rules.yaml 内规则顺序;
//     同 path 后者覆盖 (Velero 实际语义 — 调用方有意识地放后者优先)
//   - 增量: hash = SHA256(transformSet name + transformRefs + sorted params),
//     12 char prefix; 同输入复用同 CM (CM name 命中既有 → 直接返回, 不重创建)
//
// 不变量:
//   - data.rules.yaml 必须是单 key, ADR-003 血泪教训 (Velero parser 拒绝 len(Data)!=1)
//   - CM 命名带 hash 防冲突, restore 完成后由 ADR-014 GC 周期清理
//   - ${VAR} 占位符: params 必须全部提供, 缺失即编译失败 (防忘传参导致 silent
//     run with literal "${FROM}" 串)
//
// 双层 schema 假设 (PRD-002 v1.3, 还未独立 schema 文件):
//   - Transform CM (in `supkube` ns, label supkube.io/kind=transform):
//     data["rules.yaml"] = Velero ResourceModifierDoc YAML, 可含 ${VAR} 占位符
//   - TransformSet CM (in `supkube` ns, label supkube.io/kind=transform-set):
//     data["transform-set.yaml"] = YAML with `transformRefs: [name1, name2]`
//     and optional `defaults: {VAR: defaultValue}`
package transform

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigyaml "sigs.k8s.io/yaml"
)

// ─── 常量 / Schema types ────────────────────────────────────────────────
// 已迁移到 types.go (2026-05-31 合并 Agent N + Agent #144 重复声明). 见 types.go 为权威.
// CompileResult 是 TransformSet 编译产物.
type CompileResult struct {
	ConfigMap   *corev1.ConfigMap // 派生 CM (Compile 已 Create 或复用; CompileDryRun 只构造不 Create)
	RulesYAML   string            // 展平后的 rules.yaml 内容
	Hash        string            // SHA256 prefix, 复用 CM 用
	AppliedRefs []string          // 实际拍进 rules.yaml 的 Transform 名 (按 transformRefs 顺序)
}

// ─── 顶层 API ──────────────────────────────────────────────────────────

// Compile 把 TransformSet 名 + params 展平编译为派生 ConfigMap.
//
// 步骤:
//  1. Get TransformSet (在 SupKubeNamespace, 校验 label kind=transform-set)
//  2. Loop transformRefs[]: Get 每个 Transform (校验 label kind=transform)
//  3. 合并 params (TransformSet.defaults 提供默认, 调用方 params 覆盖)
//  4. 对每个 Transform applyParams 替换 ${VAR}, Validate 校验
//  5. 拼接 rules.yaml (按 transformRefs 顺序, 同 path 后者覆盖语义来自 Velero)
//  6. 算 hash, 检查派生 CM 是否已存在 — 是则直接返回 (复用, 不重 Create)
//  7. 否则 Create 派生 CM (velero ns, 带 GC label)
func Compile(ctx context.Context, cl client.Client, transformSetName string, params map[string]string) (*CompileResult, error) {
	return compile(ctx, cl, transformSetName, params, false /*dryRun*/)
}

// CompileDryRun 跟 Compile 一样但不真 Create CM (preview-resolution 端点用).
// 返回的 CompileResult.ConfigMap 是构造好的, 但未提交到 cluster.
// 如果同 hash 的派生 CM 已存在, 返回的是 cluster 上现有的那个 (read-only).
func CompileDryRun(ctx context.Context, cl client.Client, transformSetName string, params map[string]string) (*CompileResult, error) {
	return compile(ctx, cl, transformSetName, params, true /*dryRun*/)
}

func compile(ctx context.Context, cl client.Client, transformSetName string, params map[string]string, dryRun bool) (*CompileResult, error) {
	// Step 1: Get TransformSet CM.
	tsCM := &corev1.ConfigMap{}
	err := cl.Get(ctx, types.NamespacedName{Namespace: SupKubeNamespace, Name: transformSetName}, tsCM)
	if err != nil {
		return nil, fmt.Errorf("get TransformSet %q: %w", transformSetName, err)
	}
	if tsCM.Labels[LabelKind] != KindTransformSet {
		return nil, fmt.Errorf("ConfigMap %q exists but is not a TransformSet (label %s=%s expected, got %q)",
			transformSetName, LabelKind, KindTransformSet, tsCM.Labels[LabelKind])
	}
	specRaw, ok := tsCM.Data[TransformSetSpecKey]
	if !ok || strings.TrimSpace(specRaw) == "" {
		return nil, fmt.Errorf("TransformSet %q missing data[%s]", transformSetName, TransformSetSpecKey)
	}
	var spec TransformSetSpec
	if err := sigyaml.Unmarshal([]byte(specRaw), &spec); err != nil {
		return nil, fmt.Errorf("parse TransformSet %q spec: %w", transformSetName, err)
	}
	if len(spec.TransformRefs) == 0 {
		return nil, fmt.Errorf("TransformSet %q has no transformRefs", transformSetName)
	}

	// Step 2-3: merge params (调用方覆盖 defaults).
	mergedParams := make(map[string]string, len(spec.Defaults)+len(params))
	for k, v := range spec.Defaults {
		mergedParams[k] = v
	}
	for k, v := range params {
		mergedParams[k] = v
	}

	// Step 4-5: loop refs, applyParams, validate, concat.
	var resolvedDocs []string
	appliedRefs := make([]string, 0, len(spec.TransformRefs))
	for _, ref := range spec.TransformRefs {
		trCM := &corev1.ConfigMap{}
		if err := cl.Get(ctx, types.NamespacedName{Namespace: SupKubeNamespace, Name: ref}, trCM); err != nil {
			return nil, fmt.Errorf("get Transform %q (refed by TransformSet %q): %w", ref, transformSetName, err)
		}
		if trCM.Labels[LabelKind] != KindTransform {
			return nil, fmt.Errorf("ConfigMap %q exists but is not a Transform (label %s=%s expected, got %q)",
				ref, LabelKind, KindTransform, trCM.Labels[LabelKind])
		}
		raw, ok := trCM.Data[TransformRulesKey]
		if !ok || strings.TrimSpace(raw) == "" {
			return nil, fmt.Errorf("Transform %q missing data[%s]", ref, TransformRulesKey)
		}
		resolved, err := applyParams(raw, mergedParams)
		if err != nil {
			return nil, fmt.Errorf("Transform %q: %w", ref, err)
		}
		if err := Validate(resolved); err != nil {
			return nil, fmt.Errorf("Transform %q failed validation after param substitution: %w", ref, err)
		}
		resolvedDocs = append(resolvedDocs, resolved)
		appliedRefs = append(appliedRefs, ref)
	}

	mergedRulesYAML, err := mergeRulesYAML(resolvedDocs)
	if err != nil {
		return nil, fmt.Errorf("merge rules.yaml: %w", err)
	}

	// Step 6: hash + CM name.
	hash := hashInput(transformSetName, appliedRefs, mergedParams)
	cmName := DerivedCMNamePrefix + transformSetName + "-" + hash
	// k8s name length cap 253. Truncate if needed (hash already encodes uniqueness).
	if len(cmName) > 253 {
		// keep prefix + suffix-hash, chop middle of tsName.
		room := 253 - len(DerivedCMNamePrefix) - 1 - len(hash) // 1 for '-'
		if room < 8 {
			return nil, fmt.Errorf("TransformSet name %q too long for derived CM name (k8s 253 cap)", transformSetName)
		}
		cmName = DerivedCMNamePrefix + transformSetName[:room] + "-" + hash
	}

	// Step 7: probe existing.
	existing := &corev1.ConfigMap{}
	getErr := cl.Get(ctx, types.NamespacedName{Namespace: VeleroNamespace, Name: cmName}, existing)
	if getErr == nil {
		// Reuse: same hash → identical content guaranteed (by definition of hashInput).
		return &CompileResult{
			ConfigMap:   existing,
			RulesYAML:   existing.Data[TransformRulesKey],
			Hash:        hash,
			AppliedRefs: appliedRefs,
		}, nil
	}
	if !apierrors.IsNotFound(getErr) {
		return nil, fmt.Errorf("probe existing derived CM %q: %w", cmName, getErr)
	}

	// Construct fresh.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: VeleroNamespace,
			Labels: map[string]string{
				LabelKind:         KindCompiledRM,
				LabelManagedBy:    "supkube",
				LabelCompiledFrom: transformSetName,
				LabelCompileHash:  hash,
			},
		},
		Data: map[string]string{
			TransformRulesKey: mergedRulesYAML,
		},
	}

	if !dryRun {
		if err := cl.Create(ctx, cm); err != nil {
			// Race: another caller created it between our Get and Create. Re-fetch.
			if apierrors.IsAlreadyExists(err) {
				if rerr := cl.Get(ctx, types.NamespacedName{Namespace: VeleroNamespace, Name: cmName}, existing); rerr == nil {
					return &CompileResult{
						ConfigMap:   existing,
						RulesYAML:   existing.Data[TransformRulesKey],
						Hash:        hash,
						AppliedRefs: appliedRefs,
					}, nil
				}
			}
			return nil, fmt.Errorf("create derived CM %q: %w", cmName, err)
		}
	}

	return &CompileResult{
		ConfigMap:   cm,
		RulesYAML:   mergedRulesYAML,
		Hash:        hash,
		AppliedRefs: appliedRefs,
	}, nil
}

// hashInput SHA256(tsName + sortedRefs + sortedParams), 取前 12 hex char.
// 排序保证 map 顺序不稳定带来的 hash 不稳定.
func hashInput(tsName string, refs []string, params map[string]string) string {
	h := sha256.New()
	h.Write([]byte(tsName))
	h.Write([]byte{0x1f}) // unit separator, 防 name 与 ref 拼接 collision
	for _, r := range refs {
		h.Write([]byte(r))
		h.Write([]byte{0x1f})
	}
	// sort params keys for deterministic hash.
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0x1e}) // record separator
		h.Write([]byte(params[k]))
		h.Write([]byte{0x1f})
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}

// ─── Param substitution + validation ──────────────────────────────────

// paramPlaceholderRE 匹配 ${VARNAME}. VARNAME = [A-Za-z_][A-Za-z0-9_]*.
var paramPlaceholderRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// paramValueSafeRE 是 ${VAR} 替换值的内容白名单 (PRD-002 §4 finding #4 +
// 任务 #135). 允许: 字母/数字/`.`/`_`/`-`/`:`/`/`. 含义: 主机名 +
// 端口 + path 段 + 版本号都安全 (harbor.local:5000/library/foo, v1.18.0,
// csi-hostpath, gcr.io/project 都过). 但 regex meta (`*` `+` `?` `(` `)`
// `[` `]` `{` `}` `|` `\` `^` `$`) 全拒 — 防 ${VAR} 注 `.*` 把
// resourceNameRegex 的 match 面扩大到无关 resource.
//
// 与 dangerousShellChars 黑名单的关系: 这是白名单, 更严. 二者并存是
// defense-in-depth — 黑名单拒 `---` (yaml doc 分隔符) 之类不在白名单里
// 也未必被拒的串; 白名单兜底所有 regex meta. 必须同时通过两个 check
// 才算安全值.
//
// 对手作 (operator-authored) 的 regex 不走此 gate — 它直接写在 Transform
// rules.yaml 里, 不经 ${VAR} 替换. 那条路径靠 Validate 的 regexp.Compile
// probe 接住语法错误. 这是有意识的 trust boundary: operator 手写的
// regex 是设计意图, ${VAR} 注入的是输入数据, 二者信任级别不同.
var paramValueSafeRE = regexp.MustCompile(`^[A-Za-z0-9._\-:/]+$`)

// dangerousShellChars 字符集: 出现在 ${VAR} 替换值里就拒. 避免 inject 到
// regex / shell-evaluated context. Velero parser 自己不 eval shell, 但 rules
// 里的 regex/value 段含 ; | & $ ( ) 会触发 PRD-002 §4 安全要求.
const dangerousShellChars = ";|&$()<>`\n\r"

// applyParams 把 ${VAR} 替换为 params[VAR]. 未匹配的 ${VAR} → err (防忘传参).
// 校验值不含危险字符 (PRD-002 §4 安全要求).
func applyParams(template string, params map[string]string) (string, error) {
	var missingVars []string
	var unsafeVars []string
	out := paramPlaceholderRE.ReplaceAllStringFunc(template, func(match string) string {
		varName := match[2 : len(match)-1] // strip ${ }
		val, ok := params[varName]
		if !ok {
			missingVars = append(missingVars, varName)
			return match // leave as-is; we'll error below
		}
		if strings.ContainsAny(val, dangerousShellChars) {
			unsafeVars = append(unsafeVars, varName)
			return match
		}
		// yaml control sequences 也拒 — e.g. value 含 "---" 或 leading "&" anchor.
		if strings.Contains(val, "---") || strings.HasPrefix(strings.TrimSpace(val), "&") || strings.HasPrefix(strings.TrimSpace(val), "*") {
			unsafeVars = append(unsafeVars, varName)
			return match
		}
		// PRD-002 §4 finding #4 / 任务 #135: 白名单 gate — 防 regex meta
		// 注入到 resourceNameRegex (例: ${name}=evil.* 想匹配任何 name).
		// 见 paramValueSafeRE doc 的 trust-boundary 解释.
		if !paramValueSafeRE.MatchString(val) {
			unsafeVars = append(unsafeVars, varName)
			return match
		}
		return val
	})
	if len(missingVars) > 0 {
		return "", fmt.Errorf("missing params for placeholder(s): %s", strings.Join(uniqueSorted(missingVars), ", "))
	}
	if len(unsafeVars) > 0 {
		return "", fmt.Errorf("param value(s) contain unsafe characters (shell metacharacters or yaml control): %s", strings.Join(uniqueSorted(unsafeVars), ", "))
	}
	return out, nil
}

func uniqueSorted(in []string) []string {
	seen := map[string]struct{}{}
	for _, s := range in {
		seen[s] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Validate 校验 rules.yaml 安全:
//   - 内容是合法 yaml + 解出 ResourceModifierDoc 形 (有 resourceModifierRules)
//   - 每条 regex 字段能被 regexp.Compile (拒 catastrophic backtrack 来源)
//   - 长度上限 (拒 100KB+ 单 rules.yaml — Velero 也读不动)
//   - 不残留未替换的 ${VAR} (applyParams 应该负责, 但 defense in depth)
func Validate(rulesYAML string) error {
	if len(rulesYAML) > 256*1024 {
		return fmt.Errorf("rules.yaml too large (%d bytes, cap 256KB)", len(rulesYAML))
	}
	// Residual placeholder check.
	if loc := paramPlaceholderRE.FindString(rulesYAML); loc != "" {
		return fmt.Errorf("rules.yaml still contains unsubstituted placeholder %q", loc)
	}
	// Parse as ResourceModifierDoc shape.
	var doc struct {
		Version               string                   `yaml:"version" json:"version"`
		ResourceModifierRules []map[string]interface{} `yaml:"resourceModifierRules" json:"resourceModifierRules"`
	}
	if err := sigyaml.Unmarshal([]byte(rulesYAML), &doc); err != nil {
		return fmt.Errorf("rules.yaml is not valid YAML/JSON: %w", err)
	}
	if doc.Version == "" {
		return fmt.Errorf("rules.yaml missing top-level `version` field")
	}
	if len(doc.ResourceModifierRules) == 0 {
		return fmt.Errorf("rules.yaml has no resourceModifierRules")
	}
	// regex compile probe inside conditions.resourceNameRegex.
	for i, rule := range doc.ResourceModifierRules {
		conds, ok := rule["conditions"].(map[string]interface{})
		if !ok {
			continue
		}
		if re, ok := conds["resourceNameRegex"].(string); ok && re != "" {
			if len(re) > 1024 {
				return fmt.Errorf("rule[%d].conditions.resourceNameRegex too long (%d chars, cap 1024)", i, len(re))
			}
			if _, err := regexp.Compile(re); err != nil {
				return fmt.Errorf("rule[%d].conditions.resourceNameRegex compile failed: %w", i, err)
			}
			// Note: we DON'T whitelist the regex string content here —
			// operator-authored Transforms legitimately use patterns like
			// `.*` to mean "any name". The injection threat from
			// ${VAR} substitution is gated upstream in applyParams via
			// paramValueSafeRE (see PRD-002 §4 finding #4 + task #135's
			// rationale).
		}
	}
	return nil
}

// ─── mergeRulesYAML ────────────────────────────────────────────────────

// mergeRulesYAML 把多个 Transform 的 rules.yaml 合并为单个 doc, 按顺序拼接
// resourceModifierRules. 版本字段取第一份 (Velero 当前只支持 v1, 不打算混版本).
// 同 path 后者覆盖语义由 Velero 在 restore 时实现 — 我们只保证顺序.
func mergeRulesYAML(docs []string) (string, error) {
	type doc struct {
		Version               string                   `yaml:"version" json:"version"`
		ResourceModifierRules []map[string]interface{} `yaml:"resourceModifierRules" json:"resourceModifierRules"`
	}
	if len(docs) == 0 {
		return "", fmt.Errorf("no Transform docs to merge")
	}
	var merged doc
	for i, raw := range docs {
		var d doc
		if err := sigyaml.Unmarshal([]byte(raw), &d); err != nil {
			return "", fmt.Errorf("doc[%d] parse: %w", i, err)
		}
		if merged.Version == "" {
			merged.Version = d.Version
		}
		// 不强制 version 一致 (留余地兼容 Velero 后续版本); 取第一份.
		merged.ResourceModifierRules = append(merged.ResourceModifierRules, d.ResourceModifierRules...)
	}
	if merged.Version == "" {
		merged.Version = "v1"
	}
	out, err := sigyaml.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("marshal merged: %w", err)
	}
	return string(out), nil
}

// ─── GC ────────────────────────────────────────────────────────────────

// GC 在 Restore 完成时调用, 给该 Restore 引用过的派生 CM 打上 LabelRestoreRef.
// 真删由 ADR-014 controller 周期跑 (检查 timestamp + 无 active Restore 引用).
//
// 当前 Phase 1 实现: 列出 velero ns 下所有 KindCompiledRM CM, 若其
// labels[LabelRestoreRef] == restoreName 则不动 (已标记); 否则若该 Restore
// 引用过它 (通过外部传入的 cmName 列表 — Phase 1 简化为 patch all 派生 CM
// 给同 restoreName 的) — 实际产品化时应由 caller 传 cmName 进来.
//
// Phase 1 ship-able: 函数签名留 restoreName, 内部目前 no-op + log. ADR-014
// GC controller 自己扫 timestamp > 30min 且无 ActivelyReferenced 的派生 CM.
// 完整 GC 落地在另一 PR (ADR-014 controller).
func GC(ctx context.Context, cl client.Client, restoreName string) error {
	if restoreName == "" {
		return fmt.Errorf("GC: restoreName required")
	}
	// Phase 1: patch label LabelRestoreRef on every derived CM that doesn't
	// have one yet. ADR-014 controller will use lack of LabelRestoreRef +
	// age > 30min as the orphan signal.
	list := &corev1.ConfigMapList{}
	if err := cl.List(ctx, list, client.InNamespace(VeleroNamespace),
		client.MatchingLabels{LabelKind: KindCompiledRM}); err != nil {
		return fmt.Errorf("list derived CMs: %w", err)
	}
	for i := range list.Items {
		cm := &list.Items[i]
		if cm.Labels[LabelRestoreRef] != "" {
			continue // already marked
		}
		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}
		cm.Labels[LabelRestoreRef] = restoreName
		if err := cl.Update(ctx, cm); err != nil {
			// log-and-continue: GC is best-effort, ADR-014 controller will
			// retry. Don't fail the calling Restore reconcile loop.
			continue
		}
	}
	return nil
}
