// Package auditwatch 是 PRD-008 Phase 2b:为【异步级联删除】(DeleteBackup→DBR)补写终态审计。
//
// DeleteBackup handler 只能同步确证到 DBR-Created;之后 BSL/VSC 清理与 Backup CR 移除由 Velero 的
// DBR 控制器异步完成。直接轮询 DBR 不可靠——DBR 被处理后即被 Velero 清掉,轮询常错过其 Processed 瞬态。
// 故本 watcher 以【Backup CR 消失】作为删除完成的【持久信号】:对"有 DBR-Created 却无终态"的在飞删除
// Task,若其 Backup CR 已不在集群,则补 CR-Removed→Completed。状态全在审计流(重启安全、天然幂等:
// 一旦补了终态,下轮即被排除)。
package auditwatch

import (
	"context"
	"log"
	"strings"
	"time"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/audit"
	"github.com/supkube/supkube-backend/internal/velerons"
)

const defaultInterval = 30 * time.Second

// Run 起周期 watcher(对齐本仓 gc.Run 等控制器:ticker 轮询,goroutine 随进程生命周期)。
func Run(ctx context.Context, cl client.Client) {
	t := time.NewTicker(defaultInterval)
	defer t.Stop()
	RunOnce(ctx, cl)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			RunOnce(ctx, cl)
		}
	}
}

// RunOnce 扫一遍在飞异步删除,给"Backup CR 已消失"的补终态。audit 未接线则无操作。
func RunOnce(ctx context.Context, cl client.Client) {
	store := audit.Default()
	if store == nil || cl == nil {
		return
	}
	events, err := store.List(ctx, audit.ListOpts{Limit: 500})
	if err != nil {
		log.Printf("[auditwatch] list 审计失败: %v", err)
		return
	}
	for _, t := range InflightDeletes(events) {
		b := &velerov1.Backup{}
		gerr := cl.Get(ctx, client.ObjectKey{Name: t.Backup, Namespace: velerons.Namespace()}, b)
		if apierrors.IsNotFound(gerr) {
			// Backup CR 已消失 = 级联删除完成 → 补终态(同 taskID 接续 hash-chain)。
			audit.EmitDeleteTask(ctx, t.TaskID, t.Backup, audit.ActionDeleteBackup, audit.PhaseCRRemoved, "", nil)
			audit.EmitDeleteTask(ctx, t.TaskID, t.Backup, audit.ActionDeleteBackup, audit.PhaseCompleted, "", nil)
			log.Printf("[auditwatch] delete task %s (%s) Backup CR 已删 → 补终态 Completed", t.TaskID, t.Backup)
		}
		// Found / 其它错误:仍在删或暂态,留待下轮(不误判完成)。
	}
}

// Inflight 是一条在飞的异步删除 Task(有 DBR-Created、无终态)。
type Inflight struct {
	TaskID string
	Backup string
}

// InflightDeletes 从审计流挑出:DeleteBackup 类、有 DBR-Created、却无任何终态
// (Completed/CR-Removed/Failed)的 taskID。纯函数,可单测。
func InflightDeletes(events []audit.ActivityEvent) []Inflight {
	type agg struct {
		backup            string
		hasDBR, terminal  bool
	}
	m := map[string]*agg{}
	order := []string{} // 保稳定输出序(便于测试)
	for _, e := range events {
		if e.ActionType != audit.ActionDeleteBackup {
			continue
		}
		a := m[e.TaskID]
		if a == nil {
			a = &agg{}
			m[e.TaskID] = a
			order = append(order, e.TaskID)
		}
		if a.backup == "" {
			a.backup = strings.TrimPrefix(e.ResourceRef, "backup/")
		}
		switch e.Phase {
		case audit.PhaseDBRCreated:
			a.hasDBR = true
		case audit.PhaseCompleted, audit.PhaseCRRemoved, audit.PhaseFailed:
			a.terminal = true
		}
	}
	var out []Inflight
	for _, id := range order {
		if a := m[id]; a.hasDBR && !a.terminal && a.backup != "" {
			out = append(out, Inflight{TaskID: id, Backup: a.backup})
		}
	}
	return out
}
