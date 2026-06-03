// Tests for LifecyclePreflight (PRD-007 §4.3 + §4.5 联动).
//
// TC-COPY-003 — Lifecycle.Preflight() detect Policy retention 7d vs Layer 4 target retention 30d
//
//	→ 返回 ERR_LIFECYCLE_RETENTION_CONFLICT (拟错误码, 见 等待决策.md)
//
// 设计澄清: 跟 prompt 字面 "Policy retention 7d vs Layer 4 target retention 30d" 的方向
// 略反——真冲突场景是 *source > target*, 即 source 想保留 30d 但 target 7d 就删, 备份从 source
// 复制过去 7d 就在 target 没了. 本测试覆盖两个方向, 确保只对 *source > target* 报冲突,
// 反方向 (target ≥ source) 不报错.
package backupcopy

import (
	"context"
	"testing"
	"time"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeSchedule(name string, ttl time.Duration) *velerov1.Schedule {
	return &velerov1.Schedule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "velero"},
		Spec: velerov1.ScheduleSpec{
			Schedule: "0 3 * * *",
			Template: velerov1.BackupSpec{
				TTL: metav1.Duration{Duration: ttl},
			},
		},
	}
}

// TC-COPY-003: 真冲突 — source retention 30d > target delete 7d → ERR_LIFECYCLE_RETENTION_CONFLICT.
//
// 业务语义: Velero schedule 配 ttl=30d (源保留 30 天), 但 target BSL lifecycle 7 天就删了
// → 复制过去的备份在 target 上 7 天就消失, Layer 4 冗余被静默削弱 (客户不会知道).
func TestLifecycle_TC_COPY_003_RetentionConflict_SourceLongerThanTarget(t *testing.T) {
	sched := makeSchedule("daily-app1", 30*24*time.Hour)
	res, err := LifecyclePreflight(context.Background(), LifecyclePreflightRequest{
		Schedule:      sched,
		TargetBSLName: "dr-target",
		TargetRules: []TargetLifecycleRule{
			{Prefix: "", DeleteAfter: 7 * 24 * time.Hour},
		},
	})
	if err != nil {
		t.Fatalf("LifecyclePreflight: %v", err)
	}
	if res.OK {
		t.Fatal("expected conflict (source 30d > target 7d), got OK=true")
	}
	if res.Conflict == nil {
		t.Fatal("Conflict must be populated when OK=false")
	}
	if res.Conflict.ErrorCode != ErrLifecycleRetentionConflict {
		t.Errorf("ErrorCode = %q, want %q", res.Conflict.ErrorCode, ErrLifecycleRetentionConflict)
	}
	if res.Conflict.SourceRetention != 30*24*time.Hour {
		t.Errorf("SourceRetention = %v, want 30d", res.Conflict.SourceRetention)
	}
	if res.Conflict.TargetDelete != 7*24*time.Hour {
		t.Errorf("TargetDelete = %v, want 7d", res.Conflict.TargetDelete)
	}
	if res.Conflict.Reason == "" || res.Conflict.Suggestion == "" {
		t.Errorf("Reason/Suggestion must be populated: reason=%q sug=%q", res.Conflict.Reason, res.Conflict.Suggestion)
	}
}

// 反方向不报冲突: target 比 source 长 → OK.
//
// 业务语义: source 7 天就删, target 30 天保留 → 复制过去的备份在 target 上活更久,
// Layer 4 反而提供了 *延展* 保留窗口, 不是冲突.
func TestLifecycle_TargetLongerThanSource_OK(t *testing.T) {
	sched := makeSchedule("weekly", 7*24*time.Hour)
	res, err := LifecyclePreflight(context.Background(), LifecyclePreflightRequest{
		Schedule:      sched,
		TargetBSLName: "long-term",
		TargetRules: []TargetLifecycleRule{
			{Prefix: "", DeleteAfter: 30 * 24 * time.Hour},
		},
	})
	if err != nil {
		t.Fatalf("LifecyclePreflight: %v", err)
	}
	if !res.OK {
		t.Errorf("expected OK (target 30d >= source 7d), got conflict: %+v", res.Conflict)
	}
}

// Edge: target 永不删 (DeleteAfter=0) → 无冲突.
func TestLifecycle_TargetNeverDeletes_OK(t *testing.T) {
	sched := makeSchedule("monthly", 90*24*time.Hour)
	res, err := LifecyclePreflight(context.Background(), LifecyclePreflightRequest{
		Schedule:      sched,
		TargetBSLName: "forever",
		TargetRules: []TargetLifecycleRule{
			{Prefix: "kopia/", DeleteAfter: 0},
			{Prefix: "backups/", DeleteAfter: 0},
		},
	})
	if err != nil {
		t.Fatalf("LifecyclePreflight: %v", err)
	}
	if !res.OK {
		t.Errorf("expected OK when target never deletes, got conflict: %+v", res.Conflict)
	}
}

// Edge: source 永不删 (TTL=0) vs target 有限 → 冲突.
func TestLifecycle_SourceForever_TargetFinite_Conflict(t *testing.T) {
	res, err := LifecyclePreflight(context.Background(), LifecyclePreflightRequest{
		PolicyRetention: 0, // source 永不删
		TargetBSLName:   "limited",
		TargetRules: []TargetLifecycleRule{
			{Prefix: "", DeleteAfter: 90 * 24 * time.Hour},
		},
	})
	if err != nil {
		t.Fatalf("LifecyclePreflight: %v", err)
	}
	if res.OK {
		t.Fatal("expected conflict (source=forever, target=90d), got OK=true")
	}
	if res.Conflict == nil || res.Conflict.ErrorCode != ErrLifecycleRetentionConflict {
		t.Errorf("expected ERR_LIFECYCLE_RETENTION_CONFLICT, got %+v", res.Conflict)
	}
	if res.Conflict.SourceRetention != 0 {
		t.Errorf("SourceRetention should be 0 (forever), got %v", res.Conflict.SourceRetention)
	}
}

// Edge: 双方都永不删 → 无冲突.
func TestLifecycle_BothForever_OK(t *testing.T) {
	res, err := LifecyclePreflight(context.Background(), LifecyclePreflightRequest{
		PolicyRetention: 0,
		TargetBSLName:   "forever",
		TargetRules: []TargetLifecycleRule{
			{Prefix: "", DeleteAfter: 0},
		},
	})
	if err != nil {
		t.Fatalf("LifecyclePreflight: %v", err)
	}
	if !res.OK {
		t.Errorf("expected OK when both never delete, got %+v", res.Conflict)
	}
}

// Edge: 多 target rule, 取最严苛 (最短 delete) 作为冲突判定基准.
func TestLifecycle_MultipleRules_PickShortest(t *testing.T) {
	sched := makeSchedule("daily", 60*24*time.Hour)
	res, err := LifecyclePreflight(context.Background(), LifecyclePreflightRequest{
		Schedule:      sched,
		TargetBSLName: "mixed",
		TargetRules: []TargetLifecycleRule{
			{Prefix: "kopia/", DeleteAfter: 180 * 24 * time.Hour},  // 长, 不冲突
			{Prefix: "backups/", DeleteAfter: 14 * 24 * time.Hour}, // 短, 真冲突源
		},
	})
	if err != nil {
		t.Fatalf("LifecyclePreflight: %v", err)
	}
	if res.OK {
		t.Fatal("expected conflict (source 60d > shortest target 14d)")
	}
	if res.Conflict.TargetDelete != 14*24*time.Hour {
		t.Errorf("should pick shortest rule (14d), got %v", res.Conflict.TargetDelete)
	}
	if res.Conflict.TargetPrefix != "backups/" {
		t.Errorf("should report backups/ prefix as conflicting, got %q", res.Conflict.TargetPrefix)
	}
}

// Edge: 等长 (target == source) → 无冲突 (边界 ≥ 不 <).
func TestLifecycle_Equal_OK(t *testing.T) {
	sched := makeSchedule("equal", 30*24*time.Hour)
	res, err := LifecyclePreflight(context.Background(), LifecyclePreflightRequest{
		Schedule:      sched,
		TargetBSLName: "equal-target",
		TargetRules: []TargetLifecycleRule{
			{Prefix: "", DeleteAfter: 30 * 24 * time.Hour},
		},
	})
	if err != nil {
		t.Fatalf("LifecyclePreflight: %v", err)
	}
	if !res.OK {
		t.Errorf("expected OK when target == source, got conflict: %+v", res.Conflict)
	}
}

// 无 Schedule 时用 PolicyRetention 字段.
func TestLifecycle_PolicyRetentionField_Used(t *testing.T) {
	res, err := LifecyclePreflight(context.Background(), LifecyclePreflightRequest{
		Schedule:        nil,
		PolicyRetention: 30 * 24 * time.Hour,
		TargetBSLName:   "no-schedule",
		TargetRules: []TargetLifecycleRule{
			{Prefix: "", DeleteAfter: 7 * 24 * time.Hour},
		},
	})
	if err != nil {
		t.Fatalf("LifecyclePreflight: %v", err)
	}
	if res.OK {
		t.Fatal("expected conflict")
	}
	if res.Conflict.SourceRetention != 30*24*time.Hour {
		t.Errorf("should use PolicyRetention=30d, got %v", res.Conflict.SourceRetention)
	}
}

// Schedule 优先于 PolicyRetention.
func TestLifecycle_ScheduleOverridesPolicyRetention(t *testing.T) {
	sched := makeSchedule("override", 60*24*time.Hour) // 60d
	res, err := LifecyclePreflight(context.Background(), LifecyclePreflightRequest{
		Schedule:        sched,
		PolicyRetention: 5 * time.Hour, // 5h, 远小于 schedule TTL, 应被忽略
		TargetBSLName:   "x",
		TargetRules: []TargetLifecycleRule{
			{Prefix: "", DeleteAfter: 7 * 24 * time.Hour}, // 7d, < 60d 应冲突
		},
	})
	if err != nil {
		t.Fatalf("LifecyclePreflight: %v", err)
	}
	if res.OK {
		t.Fatal("expected conflict (schedule TTL 60d > target 7d)")
	}
	if res.Conflict.SourceRetention != 60*24*time.Hour {
		t.Errorf("should use Schedule.TTL=60d not PolicyRetention=5h, got %v", res.Conflict.SourceRetention)
	}
}
