// Phase 0 实测(PRD-008 / ADR-039):嵌入式 audit event store 选型。
// 工作负载对齐 DoD #19:1万 ActivityEvent append(带 hash-chain)< 500ms / 倒序 list 100 条 < 100ms / 容量增长 < 10MB。
// 候选:go.etcd.io/bbolt(B+tree 单文件)· dgraph badger/v4(LSM)· modernc.org/sqlite(纯 Go SQL)。
// 三者均纯 Go(无 CGO),契合 SupKube 裸 K8s/airgap appliance。
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	bolt "go.etcd.io/bbolt"
	_ "modernc.org/sqlite"
)

const N = 10000
const listK = 100

// ActivityEvent 对齐 PRD-008 §4.1 数据模型 + hash-chain(D2 不可篡改)。
type ActivityEvent struct {
	ID          string            `json:"id"` // ULID(此处用 seq 代,顺序等价)
	TaskID      string            `json:"task_id"`
	ClusterID   string            `json:"cluster_id"`
	ActionType  string            `json:"action_type"`
	Phase       string            `json:"phase"`
	ResourceRef string            `json:"resource_ref"`
	Triggered   string            `json:"triggered"`
	ErrorCode   string            `json:"error_code,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Timestamp   time.Time         `json:"ts"`
	PrevHash    string            `json:"prev_hash"`
	Hash        string            `json:"hash"`
}

func mkEvent(i int, prev string) ([]byte, string) {
	e := ActivityEvent{
		ID: fmt.Sprintf("%016d", i), TaskID: fmt.Sprintf("task-%d", i%500),
		ClusterID: fmt.Sprintf("clu-%d", i%20), ActionType: "DeleteBackup",
		Phase:       []string{"Submitted", "DBR-Created", "BSL-Cleanup", "VSC-Cleanup", "CR-Removed", "Completed"}[i%6],
		ResourceRef: fmt.Sprintf("backup/rp-%d", i), Triggered: "user",
		Metadata:  map[string]string{"ns": "prod", "by": "zack@jumborca.net"},
		Timestamp: time.Unix(0, int64(i)*1000), PrevHash: prev,
	}
	// hash-chain:sha256(prevHash + 事件去 Hash 字段的规范字节)。
	tmp := e
	tmp.Hash = ""
	cb, _ := json.Marshal(tmp)
	sum := sha256.Sum256(append([]byte(prev), cb...))
	e.Hash = hex.EncodeToString(sum[:])
	b, _ := json.Marshal(e)
	return b, e.Hash
}

func key(i int) []byte { var k [8]byte; binary.BigEndian.PutUint64(k[:], uint64(i)); return k[:] }

func dirSize(p string) int64 {
	var s int64
	filepath.Walk(p, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			s += fi.Size()
		}
		return nil
	})
	return s
}

type result struct {
	name     string
	appendMS float64
	listMS   float64
	sizeMB   float64
}

func benchBolt(dir string) result {
	path := filepath.Join(dir, "audit.bolt")
	db, err := bolt.Open(path, 0600, nil)
	must(err)
	defer db.Close()
	t0 := time.Now()
	prev := ""
	// 批量提交(1 txn):测原始吞吐;真实可按批 flush 平衡 fsync。
	must(db.Update(func(tx *bolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte("events"))
		for i := 0; i < N; i++ {
			var v []byte
			v, prev = mkEvent(i, prev)
			if err := b.Put(key(i), v); err != nil {
				return err
			}
		}
		return nil
	}))
	ap := time.Since(t0)
	t1 := time.Now()
	var got int
	must(db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte("events")).Cursor()
		for k, _ := c.Last(); k != nil && got < listK; k, _ = c.Prev() {
			got++
		}
		return nil
	}))
	ls := time.Since(t1)
	db.Sync()
	return result{"bbolt", ms(ap), ms(ls), mb(fileSize(path))}
}

func benchBadger(dir string) result {
	path := filepath.Join(dir, "badger")
	db, err := badger.Open(badger.DefaultOptions(path).WithLoggingLevel(badger.ERROR))
	must(err)
	defer db.Close()
	t0 := time.Now()
	prev := ""
	wb := db.NewWriteBatch()
	for i := 0; i < N; i++ {
		var v []byte
		v, prev = mkEvent(i, prev)
		must(wb.Set(key(i), v))
	}
	must(wb.Flush())
	ap := time.Since(t0)
	t1 := time.Now()
	got := 0
	must(db.View(func(txn *badger.Txn) error {
		opt := badger.DefaultIteratorOptions
		opt.Reverse = true
		it := txn.NewIterator(opt)
		defer it.Close()
		for it.Rewind(); it.Valid() && got < listK; it.Next() {
			got++
		}
		return nil
	}))
	ls := time.Since(t1)
	return result{"badger", ms(ap), ms(ls), mb(dirSize(path))}
}

func benchSQLite(dir string) result {
	path := filepath.Join(dir, "audit.db")
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	must(err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE ev(seq INTEGER PRIMARY KEY, ts INTEGER, hash TEXT, data BLOB)`)
	must(err)
	t0 := time.Now()
	tx, _ := db.Begin()
	st, _ := tx.Prepare(`INSERT INTO ev(seq,ts,hash,data) VALUES(?,?,?,?)`)
	prev := ""
	for i := 0; i < N; i++ {
		var v []byte
		v, prev = mkEvent(i, prev)
		_, err := st.Exec(i, int64(i)*1000, prev, v)
		must(err)
	}
	must(tx.Commit())
	ap := time.Since(t0)
	t1 := time.Now()
	rows, err := db.Query(`SELECT data FROM ev ORDER BY seq DESC LIMIT ?`, listK)
	must(err)
	got := 0
	for rows.Next() {
		var b []byte
		rows.Scan(&b)
		got++
	}
	rows.Close()
	ls := time.Since(t1)
	return result{"sqlite", ms(ap), ms(ls), mb(dirSize(dir) - fileLike(dir, "audit.bolt") - fileLike(dir, "badger"))}
}

func main() {
	dir, _ := os.MkdirTemp("", "auditbench")
	defer os.RemoveAll(dir)
	fmt.Printf("Phase 0 实测:%d ActivityEvent(hash-chain)| 倒序 list %d | DoD#19 阈值 写<500ms list<100ms 容量<10MB\n\n", N, listK)
	rows := []result{benchBolt(dir), benchBadger(dir), benchSQLite(dir)}
	fmt.Printf("%-8s %12s %12s %10s %10s\n", "store", "append(ms)", "list(ms)", "size(MB)", "DoD#19")
	for _, r := range rows {
		ok := "✅"
		if r.appendMS >= 500 || r.listMS >= 100 || r.sizeMB >= 10 {
			ok = "❌"
		}
		fmt.Printf("%-8s %12.1f %12.2f %10.2f %10s\n", r.name, r.appendMS, r.listMS, r.sizeMB, ok)
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
func mb(b int64) float64         { return float64(b) / 1024 / 1024 }
func fileSize(p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return fi.Size()
}
func fileLike(dir, name string) int64 {
	p := filepath.Join(dir, name)
	if fi, err := os.Stat(p); err == nil {
		if fi.IsDir() {
			return dirSize(p)
		}
		return fi.Size()
	}
	return 0
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
