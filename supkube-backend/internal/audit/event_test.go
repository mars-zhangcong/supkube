package audit

import (
	"testing"
	"time"
)

func sample(id, prev string) ActivityEvent {
	return ActivityEvent{
		ID: id, TaskID: "task-1", ClusterID: "clu-1",
		ActionType: ActionDeleteBackup, Phase: PhaseSubmitted,
		ResourceRef: "backup/rp-1", Triggered: TriggerUser,
		Metadata:  map[string]string{"ns": "prod"},
		Timestamp: time.Unix(1000, 0).UTC(), PrevHash: prev,
	}
}

func TestComputeHash_Deterministic(t *testing.T) {
	e := sample("01", "")
	h1, h2 := e.ComputeHash(), e.ComputeHash()
	if h1 != h2 {
		t.Fatal("同输入应同 hash(确定性)")
	}
	if h1 == "" || len(h1) != 64 {
		t.Fatalf("应为 64 hex 的 sha256,got %q", h1)
	}
}

func TestComputeHash_SelfHashFieldExcluded(t *testing.T) {
	e := sample("01", "")
	h := e.ComputeHash()
	e.Hash = "已经填了别的值"
	if e.ComputeHash() != h {
		t.Fatal("ComputeHash 应排除 Hash 字段自身,填值不影响结果")
	}
}

func TestComputeHash_TamperBreaksChain(t *testing.T) {
	// 链:e1 → e2(e2.PrevHash = e1.Hash)。
	e1 := sample("01", "")
	e1.Hash = e1.ComputeHash()
	e2 := sample("02", e1.Hash)
	e2.Hash = e2.ComputeHash()

	// 校验器口径:重算每条 hash 并核对 prev 链接。
	verify := func(chain []ActivityEvent) (bool, string) {
		prev := ""
		for _, e := range chain {
			if e.PrevHash != prev {
				return false, e.ID + ":prev 断链"
			}
			if e.ComputeHash() != e.Hash {
				return false, e.ID + ":内容被篡改"
			}
			prev = e.Hash
		}
		return true, ""
	}

	if ok, _ := verify([]ActivityEvent{e1, e2}); !ok {
		t.Fatal("完好链应校验通过")
	}
	// 篡改 e1 的内容(不重算 hash)→ 应被检出。
	tampered := []ActivityEvent{e1, e2}
	tampered[0].ResourceRef = "backup/rp-EVIL"
	if ok, at := verify(tampered); ok || at == "" {
		t.Fatalf("篡改 e1 内容应断链被检出,got ok=%v at=%q", ok, at)
	}
}

func TestComputeHash_PrevLinkageMatters(t *testing.T) {
	a := sample("01", "")
	b := sample("01", "different-prev")
	if a.ComputeHash() == b.ComputeHash() {
		t.Fatal("不同 PrevHash 应产生不同 hash(链接入算)")
	}
}
