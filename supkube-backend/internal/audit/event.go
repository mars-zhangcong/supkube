// Package audit 实现 PRD-008 的 Activity 持久化层:把 Velero CR 派生的实时 Activity 列表升级为
// 独立审计事件流 —— 每个 Task(Backup/Restore/DeleteBackup/ForceDeleteBackup/OrphanCleanup)的
// 5 阶段生命周期都落审计事件,CR 死后卡片仍在。存储选型见 ADR-039(主选 SQLite on PV,Phase 0 实测锁定)。
//
// 本文件是 Phase 0 交付的【契约】(Rule H 抽象先行,选型前实现可换):ActivityEvent 数据模型 +
// hash-chain(D2 不可篡改,与具体 store 无关、可单测)+ AuditStore 接口。SQLite 实现属 Phase 1。
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// ActionType 是被审计的 Task 类别(PRD-008 §4.1)。
type ActionType string

const (
	ActionBackup            ActionType = "Backup"
	ActionRestore           ActionType = "Restore"
	ActionDeleteBackup      ActionType = "DeleteBackup"
	ActionForceStripFinalizer ActionType = "ForceStripFinalizer" // 原 ForceDeleteBackup,PRD-008 §4.5 改名
	ActionOrphanCleanup     ActionType = "OrphanCleanup"
	ActionTTLPrune          ActionType = "ActivityTTLPrune" // TTL 清理本身也写审计(PRD-008 §4.x)
)

// Phase 是 Task 生命周期阶段(PRD-008 §4.2 删除 Task 5 阶段;不同 ActionType 阶段集可不同)。
type Phase string

const (
	PhaseSubmitted  Phase = "Submitted"
	PhaseDBRCreated Phase = "DBR-Created"
	PhaseBSLCleanup Phase = "BSL-Cleanup"
	PhaseVSCCleanup Phase = "VSC-Cleanup"
	PhaseCRRemoved  Phase = "CR-Removed"
	PhaseCompleted  Phase = "Completed"
	PhaseFailed     Phase = "Failed"
)

// Trigger 是触发来源(PRD-008 §4.1)。
type Trigger string

const (
	TriggerUser   Trigger = "user"
	TriggerCron   Trigger = "cron"
	TriggerAPI    Trigger = "api"
	TriggerSystem Trigger = "system"
)

// ActivityEvent 是审计事件流的一条记录(PRD-008 §4.1 数据模型)。
// 持久化为 append-only;PrevHash/Hash 构成 hash-chain(D2 不可篡改),由 AuditStore 在 Append 时维护。
type ActivityEvent struct {
	ID          string            `json:"id"`                    // ULID(单调、含时间,天然倒序)
	TaskID      string            `json:"task_id"`               // 一个 Task 的多条阶段事件共享
	ClusterID   string            `json:"cluster_id"`            // 目标集群
	ActionType  ActionType        `json:"action_type"`           //
	Phase       Phase             `json:"phase"`                 // 本条记录的阶段
	ResourceRef string            `json:"resource_ref"`          // 如 backup/rp-123
	Triggered   Trigger           `json:"triggered"`             // user/cron/api/system
	ErrorCode   string            `json:"error_code,omitempty"`  // ERR_*(遵循 ADR-035),失败时填
	Metadata    map[string]string `json:"metadata,omitempty"`    // ns/by/... 扩展位
	Timestamp   time.Time         `json:"ts"`                    // 事件时刻(UTC)
	PrevHash    string            `json:"prev_hash"`             // 上一条的 Hash(链首为空)
	Hash        string            `json:"hash"`                  // 本条内容的 hash(见 ComputeHash)
}

var idCounter uint32

// NewID 生成一个【字典序=时间序】的唯一 id(ULID 同序语义,无外部依赖):
// 16 hex 的 UTC unix-nano + 8 hex 进程内自增计数(同纳秒去重)。链顺序的权威是 store 的自增 seq,
// 本 id 仅作业务唯一键,故计数器进程内复位不影响链完整性。
func NewID() string {
	return fmt.Sprintf("%016X%08X", uint64(time.Now().UTC().UnixNano()), atomic.AddUint32(&idCounter, 1))
}

// ComputeHash 计算本事件的 hash-chain 值:sha256(prevHash + 事件去 Hash 字段的规范化 JSON)。
// 与具体 store 无关、确定性、可单测;篡改任一字段或断链都会使后续校验失败(D2)。
// 调用方应在 Append 时:e.PrevHash = 上一条.Hash;e.Hash = e.ComputeHash()。
func (e ActivityEvent) ComputeHash() string {
	c := e
	c.Hash = "" // 自身 Hash 不参与计算
	b, _ := json.Marshal(c)
	sum := sha256.Sum256(append([]byte(e.PrevHash), b...))
	return hex.EncodeToString(sum[:])
}

// ListOpts 是 List 查询条件(PRD-008:倒序 + 时间范围 + TaskID/集群过滤)。零值=全量倒序。
type ListOpts struct {
	Limit   int       // 0=默认上限(实现决定);倒序取最近 N
	TaskID  string    // 非空则仅该 Task
	Cluster string    // 非空则仅该集群
	Since   time.Time // 非零则 ts >= Since
	Until   time.Time // 非零则 ts <  Until
}

// AuditStore 是 audit event 持久化抽象(ADR-039 §8 Rule H)。
// 主选实现 = SQLite on PV(Phase 1);备选 = bbolt(同接口可换)。实现须保证:
//   - Append 原子维护 hash-chain(单写者串行);
//   - List 倒序(最近优先),按 opt 过滤;
//   - 写为 append-only(不改历史),TTL 仅靠 PruneBefore 删旧。
type AuditStore interface {
	Append(ctx context.Context, e *ActivityEvent) error              // 入流(填 PrevHash/Hash 后落库)
	List(ctx context.Context, opt ListOpts) ([]ActivityEvent, error) // 倒序 + 过滤
	Get(ctx context.Context, id string) (*ActivityEvent, error)      // 取单条
	PruneBefore(ctx context.Context, cutoff time.Time) (int, error)  // TTL:删 ts<cutoff,返回删除条数
	VerifyChain(ctx context.Context) (ok bool, brokenAt string, err error) // D2:校验 hash-chain 连续
	Close() error
}
