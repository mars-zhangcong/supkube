package audit

import (
	"context"
	"log"
	"sync"
)

// 进程级单例(对齐本仓 k8s.GetClient() 的取用习惯):server 启动时 SetDefault 接线;handler 经 Emit 写。
// feature gate(PRD-008 #24):未 SetDefault = 关,Emit 成无操作 → 审计可一键停而不动 handler 代码。

var (
	defMu    sync.RWMutex
	defStore AuditStore
)

// DefaultClusterID 是本实例所属集群标识(server 可在启动时按需覆盖;单集群默认 local)。
var DefaultClusterID = "local"

// SetDefault 接线全局 audit store(server 启动调用;传 nil 可热关审计)。
func SetDefault(s AuditStore) {
	defMu.Lock()
	defStore = s
	defMu.Unlock()
}

// Default 返回当前全局 store(未接线则 nil)。
func Default() AuditStore {
	defMu.RLock()
	defer defMu.RUnlock()
	return defStore
}

// Enabled 报告审计是否已接线(feature gate 状态)。
func Enabled() bool { return Default() != nil }

// Emit best-effort 落一条审计事件:未接线=无操作;写失败=记日志不阻塞。
// 铁律:审计是观测面,绝不能因审计写失败而拖垮数据保护操作(删除/恢复)本身。
func Emit(ctx context.Context, e *ActivityEvent) {
	s := Default()
	if s == nil || e == nil {
		return
	}
	if err := s.Append(ctx, e); err != nil {
		log.Printf("[audit] emit %s/%s 失败(忽略,不阻塞主流程): %v", e.ActionType, e.Phase, err)
	}
}

// EmitDeleteTask 便捷:为一次删除 Task 的某阶段落事件(同 taskID 串起多阶段;resourceRef=backup/<name>)。
func EmitDeleteTask(ctx context.Context, taskID, backup string, action ActionType, ph Phase, errCode string, meta map[string]string) {
	Emit(ctx, &ActivityEvent{
		TaskID: taskID, ClusterID: DefaultClusterID, ActionType: action, Phase: ph,
		ResourceRef: "backup/" + backup, Triggered: TriggerUser, ErrorCode: errCode, Metadata: meta,
	})
}
