// controller.go — ImportPolicy reconciler.
//
// 不走 controller-runtime Manager + Reconcile 的原因:
//   - ImportPolicy 的核心循环是 "时间触发 + S3 list", 不是 "K8s 事件触
//     发". controller-runtime 的 informer 缓存帮不上忙 (BSL 内容在 K8s
//     之外).
//   - 我们要 per-CR 独立 ticker / cron entry, 这跟 Reconcile 的 work-queue
//     模型反向. 自己起 goroutine 更直观.
//
// 结构:
//
//	Run(ctx)              — 顶层入口, 起 manager loop. 30s 扫一次 CR 列表,
//	                        对每个 CR 调 ensureRunner. CR 删除时关掉对应
//	                        goroutine.
//	runnerFor(policy)     — 一个 CR 一个 goroutine. 内部根据 mode 起
//	                        ticker 或 cron entry, 每次触发调 syncOnce.
//	syncOnce(ctx, policy) — 实际同步: 列 BSL → diff lastSeenBackupName →
//	                        validate fingerprint → import → 更新 status.
//	stopRunner(name)      — cancel + 从 sync.Map 删. CR delete / pause /
//	                        mode change 都走这里然后重启.
package importpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

const (
	// managerInterval 是顶层 manager loop 多久扫一次 CR 列表. 30s 是经
	// 验值: 比 ImportPolicy 最短允许的 Continuous interval (30s) 一样,
	// CR 创建后最坏 30s 就会被 manager 接管. 不影响已 running 的 CR.
	managerInterval = 30 * time.Second

	// defaultContinuousInterval 是 spec.continuousInterval 缺省值.
	defaultContinuousInterval = 60 * time.Second
	// minContinuousInterval 是允许的最小间隔. 低于此值, handler 拒绝 +
	// controller 兜底 clamp (防御性, 万一 CRD validation 漏过).
	minContinuousInterval = 30 * time.Second

	// defaultTargetVeleroNamespace 是 spec.targetVeleroNamespace 缺省值.
	defaultTargetVeleroNamespace = "velero"

	// Labels 写到导入的 Velero Backup CR 上.
	LabelImported          = "supkube.io/imported"
	LabelSourceCluster     = "supkube.io/source-cluster"
	LabelFingerprintStatus = "supkube.io/fingerprint-status"
)

// runner 包装一个 per-CR goroutine 的生命周期. cancel 关 ctx,
// done channel 用于等 goroutine 真退出 (避免 stop 后立刻 ensure 出现
// 两个 goroutine).
type runner struct {
	cancel context.CancelFunc
	done   chan struct{}
	// generation 用于 mode/interval/schedule 变更检测; 变了就 restart.
	generation int64
}

// Controller 是 manager 的具体实现. 持有 dependencies + 在 sync.Map 里
// 跟踪 per-CR runner.
type Controller struct {
	DynCli    dynamic.Interface
	Lister    BackupLister
	Importer  BackupImporter
	Validator Validator

	runners sync.Map // map[string]*runner; key=policy name
}

// RunController 是 cmd/server 调的入口. 阻塞到 ctx 取消.
//
// 签名: 用 plain dependencies (不引 manager interface), 测试时直接
// 构造 Controller{} 调 syncOnce/ensureRunner 即可, 不必走 Run.
func RunController(ctx context.Context, dynCli dynamic.Interface, lister BackupLister, importer BackupImporter, validator Validator) {
	c := &Controller{
		DynCli:    dynCli,
		Lister:    lister,
		Importer:  importer,
		Validator: validator,
	}
	c.Run(ctx)
}

// Run 是 controller 主循环. 30s 扫 CR 一次, ensure runner 同步.
func (c *Controller) Run(ctx context.Context) {
	log.Printf("[importpolicy] controller started (manager interval %s)", managerInterval)
	tick := time.NewTicker(managerInterval)
	defer tick.Stop()
	// 立即扫一次, 不等首个 tick.
	c.reconcileAll(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[importpolicy] controller stopping: %v", ctx.Err())
			c.stopAll()
			return
		case <-tick.C:
			c.reconcileAll(ctx)
		}
	}
}

// reconcileAll 列出所有 CR, ensure/stop runner 与之对齐.
func (c *Controller) reconcileAll(ctx context.Context) {
	list, err := c.DynCli.Resource(GVR).Namespace(Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		// CRD 没装时安静跳过 (chart 安装前的 backend 启动场景).
		if apierrors.IsNotFound(err) || strings.Contains(err.Error(), "no matches for kind") {
			return
		}
		log.Printf("[importpolicy] list CRs: %v (skip this tick)", err)
		return
	}
	seen := map[string]struct{}{}
	for i := range list.Items {
		p, err := fromUnstructured(&list.Items[i])
		if err != nil {
			log.Printf("[importpolicy] %s: decode CR: %v", list.Items[i].GetName(), err)
			continue
		}
		seen[p.Name] = struct{}{}
		c.ensureRunner(ctx, p)
	}
	// 删掉已不存在的 CR 对应 runner.
	c.runners.Range(func(k, v interface{}) bool {
		name := k.(string)
		if _, ok := seen[name]; !ok {
			c.stopRunner(name)
		}
		return true
	})
}

// ensureRunner 保证有一个对应 policy 的 goroutine 在跑. mode/interval/
// schedule/paused 变更时会 stop+start.
func (c *Controller) ensureRunner(parent context.Context, p *ImportPolicy) {
	gen := generationOf(p)
	if v, ok := c.runners.Load(p.Name); ok {
		r := v.(*runner)
		if r.generation == gen {
			return // 状态未变, 复用
		}
		c.stopRunner(p.Name)
	}
	// Paused 时, 不起 runner — 只写 status.phase=Paused 一次.
	if p.Spec.Paused {
		c.patchStatus(parent, p.Name, statusPatch{Phase: PhasePaused})
		// 仍然记录一个空的 runner, 避免每个 tick 都重写 Paused status.
		// generation 不同时下个 ensureRunner 会重起.
		c.runners.Store(p.Name, &runner{cancel: func() {}, done: closedChan(), generation: gen})
		return
	}
	ctx, cancel := context.WithCancel(parent)
	r := &runner{cancel: cancel, done: make(chan struct{}), generation: gen}
	c.runners.Store(p.Name, r)
	go c.runnerFor(ctx, p, r)
}

// stopRunner cancel + 等 goroutine 退出, 然后从 map 删.
func (c *Controller) stopRunner(name string) {
	v, ok := c.runners.LoadAndDelete(name)
	if !ok {
		return
	}
	r := v.(*runner)
	r.cancel()
	// Best-effort wait, 别死等 — 单测里 done 可能已经 close.
	select {
	case <-r.done:
	case <-time.After(2 * time.Second):
		log.Printf("[importpolicy] %s: runner stop timeout", name)
	}
}

func (c *Controller) stopAll() {
	c.runners.Range(func(k, _ interface{}) bool {
		c.stopRunner(k.(string))
		return true
	})
}

// runnerFor 是 per-CR 的主循环. defer close done 让 stopRunner 能 join.
func (c *Controller) runnerFor(ctx context.Context, p *ImportPolicy, r *runner) {
	defer close(r.done)
	// 入循环就标 Active, 让 UI 不要还显示 Pending.
	c.patchStatus(ctx, p.Name, statusPatch{Phase: PhaseActive})

	switch p.Spec.Mode {
	case ImportModeContinuous:
		interval := parseDurationOr(p.Spec.ContinuousInterval, defaultContinuousInterval)
		if interval < minContinuousInterval {
			interval = minContinuousInterval
		}
		tk := time.NewTicker(interval)
		defer tk.Stop()
		// 立即 sync 一次, 不等首个 tick (与 clusterhealth 同模式).
		c.safeSync(ctx, p)
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				c.safeSync(ctx, p)
			}
		}
	case ImportModeScheduled:
		sched, err := ParseCron(p.Spec.Schedule)
		if err != nil {
			c.patchStatus(ctx, p.Name, statusPatch{
				Phase:     PhaseFailed,
				LastError: "invalid cron: " + err.Error(),
			})
			return
		}
		// Scheduled 模式启动也立即跑一次, 用户的预期通常是"创建即触发首次同
		// 步, 之后按 cron". 不然首个 cron 到达前 UI 一直零计数, 像没工作.
		c.safeSync(ctx, p)
		for {
			next := sched.Next(time.Now())
			if next.IsZero() {
				log.Printf("[importpolicy] %s: cron has no next trigger; runner exiting", p.Name)
				return
			}
			wait := time.Until(next)
			tm := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				tm.Stop()
				return
			case <-tm.C:
				c.safeSync(ctx, p)
			}
		}
	default:
		c.patchStatus(ctx, p.Name, statusPatch{Phase: PhaseFailed, LastError: "unknown mode: " + string(p.Spec.Mode)})
	}
}

// safeSync 包 syncOnce, 把 panic 兜住, 写 lastError. controller 主循环
// 不能 panic 退出.
func (c *Controller) safeSync(ctx context.Context, p *ImportPolicy) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[importpolicy] %s: PANIC in syncOnce: %v", p.Name, r)
			c.patchStatus(ctx, p.Name, statusPatch{
				Phase:     PhaseFailed,
				LastError: fmt.Sprintf("panic: %v", r),
			})
		}
	}()
	if err := c.syncOnce(ctx, p); err != nil {
		log.Printf("[importpolicy] %s: sync error: %v", p.Name, err)
	}
}

// SyncResult 是 run-once handler 用的返回 (controller 内部也用一份, 给单
// 测断言更方便).
type SyncResult struct {
	ImportedCount int      `json:"importedCount"`
	RejectedCount int      `json:"rejectedCount"`
	Errors        []string `json:"errors,omitempty"`
	NewBackups    []string `json:"newBackups,omitempty"`
}

// SyncOnce 是 handler 调的 public 入口 (run-once endpoint). 内部调
// syncOnceWithResult 拿 SyncResult.
func (c *Controller) SyncOnce(ctx context.Context, name string) (*SyncResult, error) {
	cr, err := c.DynCli.Resource(GVR).Namespace(Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	p, err := fromUnstructured(cr)
	if err != nil {
		return nil, err
	}
	return c.syncOnceWithResult(ctx, p)
}

// syncOnce 是 runner 内部用的轻包装 — 只关心 error (status patch 已经
// 在 syncOnceWithResult 里写好了).
func (c *Controller) syncOnce(ctx context.Context, p *ImportPolicy) error {
	_, err := c.syncOnceWithResult(ctx, p)
	return err
}

// syncOnceWithResult 是核心同步逻辑.
//
// 流程:
//  1. 列 BSL 上的 backup 名 (BackupLister)
//  2. 排序 + 筛出 > lastSeenBackupName 的"新"backup
//  3. 对每个新 backup: validate fingerprint → (按 mode 决定是否) import
//  4. 累加计数 → patch status
//
// 单条 backup 处理失败不影响后续 backup; 整体网络/list 失败才返回 error
// 给上层 (会写 lastError + Phase=Failed).
func (c *Controller) syncOnceWithResult(ctx context.Context, p *ImportPolicy) (*SyncResult, error) {
	res := &SyncResult{}
	if p.Spec.Paused {
		// safety net — 正常路径 ensureRunner 已经拦掉 paused CR; 这里再
		// 兜一遍是给 run-once handler 防御.
		c.patchStatus(ctx, p.Name, statusPatch{Phase: PhasePaused})
		return res, nil
	}
	names, err := c.Lister.ListBackups(ctx, p.Spec.SourceBSL)
	if err != nil {
		c.patchStatus(ctx, p.Name, statusPatch{
			Phase:     PhaseFailed,
			LastError: fmt.Sprintf("list BSL %q: %v", p.Spec.SourceBSL, err),
		})
		return nil, fmt.Errorf("list BSL: %w", err)
	}
	sort.Strings(names) // Lister 应已排序, 这里二次保险

	targetNs := p.Spec.TargetVeleroNamespace
	if targetNs == "" {
		targetNs = defaultTargetVeleroNamespace
	}
	fpMode := p.Spec.FingerprintMode
	if fpMode == "" {
		fpMode = FingerprintModeEnforce
	}

	lastSeen := p.Status.LastSeenBackupName
	highestSeen := lastSeen

	for _, name := range names {
		if name <= lastSeen {
			continue // 已经处理过
		}
		// fingerprint validate
		fp, fpErr := c.Validator.ValidateBackup(ctx, p.Spec.SourceBSL, name, fpMode)
		if fpErr != nil {
			res.RejectedCount++
			res.Errors = append(res.Errors, fmt.Sprintf("%s: fingerprint error: %v", name, fpErr))
			// 不更新 highestSeen — 下个 tick 会重试.
			continue
		}
		// sourceClusterID 过滤 (spec.sourceClusterID 非空时, 必须匹配
		// fingerprint result 里的 sourceClusterID; 不匹配 = 静默跳过, 不
		// 计入 rejected, 这是 "不属于我这个 policy 关心的 backup", 不是错).
		if p.Spec.SourceClusterID != "" && fp != nil && fp.SourceClusterID != "" {
			if fp.SourceClusterID != p.Spec.SourceClusterID {
				// 仍然推进 highestSeen — 否则会无限重 validate.
				if name > highestSeen {
					highestSeen = name
				}
				continue
			}
		}
		// enforce 模式且校验失败 → reject
		if fpMode == FingerprintModeEnforce && !fp.IsValid() {
			res.RejectedCount++
			reason := "missing"
			if fp != nil && fp.Reason != "" {
				reason = fp.Reason
			} else if fp != nil {
				reason = fp.Status
			}
			res.Errors = append(res.Errors, fmt.Sprintf("%s: fingerprint %s (enforce mode)", name, reason))
			// 推进 watermark, 防止反复 reject 同一个 backup (admin 看到
			// rejectedCount += 1 后会去查; 我们不需要每 tick 都加 1).
			if name > highestSeen {
				highestSeen = name
			}
			continue
		}
		// import
		labels := map[string]string{
			LabelImported: "true",
		}
		if fp != nil && fp.SourceClusterID != "" {
			labels[LabelSourceCluster] = fp.SourceClusterID
		}
		if fpMode != FingerprintModeDisabled && fp != nil {
			labels[LabelFingerprintStatus] = fp.Status
		}
		if err := c.Importer.ImportBackup(ctx, targetNs, p.Spec.SourceBSL, name, labels); err != nil {
			res.RejectedCount++
			res.Errors = append(res.Errors, fmt.Sprintf("%s: import: %v", name, err))
			// 不推进 highestSeen — import 失败下个 tick 重试.
			continue
		}
		res.ImportedCount++
		res.NewBackups = append(res.NewBackups, name)
		if name > highestSeen {
			highestSeen = name
		}
	}

	// 累加到 CR status (read-modify-write — patchStatus 用 mergePatch).
	c.patchStatus(ctx, p.Name, statusPatch{
		Phase:              PhaseActive,
		LastSyncAt:         time.Now().UTC().Format(time.RFC3339),
		LastSeenBackupName: highestSeen,
		ImportedDelta:      res.ImportedCount,
		RejectedDelta:      res.RejectedCount,
		ClearLastError:     true,
		ExistingImported:   p.Status.ImportedCount,
		ExistingRejected:   p.Status.RejectedCount,
	})
	return res, nil
}

// ─────────────────────────────────────────────────────────────────────
// helpers — CR decode + status patch
// ─────────────────────────────────────────────────────────────────────

// fromUnstructured 把 dynamic-client 返回的 unstructured CR 解码成
// strongly-typed ImportPolicy. 单字段 nested 取法繁琐, 这里走
// json round-trip 偷懒 — 性能可忽略 (一个 CR 100B 量级).
func fromUnstructured(u *unstructured.Unstructured) (*ImportPolicy, error) {
	b, err := u.MarshalJSON()
	if err != nil {
		return nil, err
	}
	p := &ImportPolicy{}
	if err := json.Unmarshal(b, p); err != nil {
		return nil, err
	}
	return p, nil
}

// generationOf 返回一个"代际号", 任何会影响 runner 行为的字段变了就变.
// metadata.generation 也会变, 但我们要包含 status.lastSeenBackupName 之
// 外的 spec 字段; 直接哈希相关字段最稳.
func generationOf(p *ImportPolicy) int64 {
	// 用 ObjectMeta.Generation (spec 改 K8s 自动 +1) 作为基线, 再叠加
	// paused 标志 (paused 不一定改 spec — patch 一个布尔可能不变 Generation;
	// 实际 K8s 会变, 这里防御性).
	g := p.Generation
	if p.Spec.Paused {
		g = g*2 + 1
	} else {
		g = g * 2
	}
	return g
}

// statusPatch 是 patchStatus 的输入参数 (扁平化, 比直接传 map 易读).
type statusPatch struct {
	Phase              Phase
	LastSyncAt         string
	LastSeenBackupName string
	// Delta 字段 (累加而不是覆盖). patchStatus 把 ExistingX + Delta 写回.
	ImportedDelta    int
	RejectedDelta    int
	ExistingImported int
	ExistingRejected int
	LastError        string
	ClearLastError   bool
}

// patchStatus 把 statusPatch 序列化成 merge-patch 写到 CR /status
// subresource. 失败只 log 不 panic; 下个 tick 会再写.
func (c *Controller) patchStatus(ctx context.Context, name string, p statusPatch) {
	status := map[string]interface{}{}
	if p.Phase != "" {
		status["phase"] = string(p.Phase)
	}
	if p.LastSyncAt != "" {
		status["lastSyncAt"] = p.LastSyncAt
	}
	if p.LastSeenBackupName != "" {
		status["lastSeenBackupName"] = p.LastSeenBackupName
	}
	if p.ImportedDelta != 0 || p.RejectedDelta != 0 {
		// 用 existing+delta 计算, 这样并发 syncOnce 不至于互相覆盖到 0.
		// (本 controller 设计下同一 CR 一次只一个 goroutine, 但 run-once
		// handler 可能和 ticker 撞上; existing 取自最近一次 List 的快照,
		// 不是最严的并发安全但够用 — 极端情况下计数偏差 ±1, admin 可接受).
		status["importedCount"] = p.ExistingImported + p.ImportedDelta
		status["rejectedCount"] = p.ExistingRejected + p.RejectedDelta
	}
	if p.LastError != "" {
		status["lastError"] = p.LastError
	} else if p.ClearLastError {
		status["lastError"] = ""
	}
	if len(status) == 0 {
		return
	}
	patch := map[string]interface{}{"status": status}
	body, err := json.Marshal(patch)
	if err != nil {
		log.Printf("[importpolicy] %s: marshal status patch: %v", name, err)
		return
	}
	_, err = c.DynCli.Resource(GVR).Namespace(Namespace).Patch(
		ctx, name, types.MergePatchType, body, metav1.PatchOptions{}, "status",
	)
	if err != nil {
		// 404 = CR 刚被删, 静默跳过.
		if apierrors.IsNotFound(err) {
			return
		}
		log.Printf("[importpolicy] %s: patch status: %v", name, err)
	}
}

// parseDurationOr 解析 Go duration; 失败返回默认值. controller 防御性 —
// handler 已经验过, 但 CR 也可能被 kubectl apply 绕过去.
func parseDurationOr(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// closedChan 返回一个已 close 的 chan struct{}. 用于 paused-runner 占位.
func closedChan() chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}
