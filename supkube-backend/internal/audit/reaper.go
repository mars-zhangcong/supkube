package audit

import (
	"context"
	"log"
	"strconv"
	"time"
)

// TTL 清理(PRD-008 DoD #13):默认保留 90 天,企业版可调长。清理操作本身写一条审计事件
// (ActionTTLPrune),保证"谁在何时删了多少旧审计"也可追溯。

// DefaultRetention 是 audit 事件默认保留期(社区版 90 天;企业版应 ≥180 天,由调用方传入覆盖)。
const DefaultRetention = 90 * 24 * time.Hour

// PruneOnce 删掉早于 now-retention 的事件,并(若有删除)落一条 ActionTTLPrune 审计事件。返回删除条数。
func PruneOnce(ctx context.Context, s AuditStore, retention time.Duration) (int, error) {
	cutoff := time.Now().UTC().Add(-retention)
	n, err := s.PruneBefore(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		// 清理本身可审计(DoD):事件入同一流,接续 hash-chain。写失败必须留痕(否则删了数据却无记录)。
		if err := s.Append(ctx, &ActivityEvent{
			TaskID: "ttl-prune", ClusterID: "_system", ActionType: ActionTTLPrune,
			Phase: PhaseCompleted, ResourceRef: "audit", Triggered: TriggerSystem,
			Metadata: map[string]string{
				"pruned":    strconv.Itoa(n),
				"retention": retention.String(),
				"cutoff":    cutoff.Format(time.RFC3339),
			},
		}); err != nil {
			// 行已删但自审计未写成:不吞,至少留痕(数据已删=不回滚 n)。
			log.Printf("[audit-ttl] 已删 %d 条但 TTL 自审计事件写入失败: %v", n, err)
		}
	}
	return n, nil
}

// StartTTLReaper 起后台 reaper:立即跑一次,之后每 interval 跑一次,直到 ctx.Done()。返回的 stop 等待 goroutine 退出。
// 生产建议 interval=24h(对齐 PRD-008 每日 02:00 cron;精确到点的调度可由上层 cron 触发 PruneOnce 替代本 ticker)。
func StartTTLReaper(ctx context.Context, s AuditStore, retention, interval time.Duration) (stop func()) {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		run := func() {
			if n, err := PruneOnce(ctx, s, retention); err != nil {
				log.Printf("[audit-ttl] prune 失败: %v", err)
			} else if n > 0 {
				log.Printf("[audit-ttl] 清理 %d 条早于 %s 的审计事件", n, retention)
			}
		}
		run()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				run()
			}
		}
	}()
	return func() { <-done }
}
