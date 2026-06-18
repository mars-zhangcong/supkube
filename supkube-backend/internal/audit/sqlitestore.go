package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动(无 CGO,airgap/distroless 友好,ADR-039 主选)
)

// SQLiteStore 是 AuditStore 的主选实现(ADR-039:嵌入式 SQLite on PV)。
// 设计:append-only 审计流;单写者(mu 串行 Append/Prune 维护 hash-chain);WAL 允许并发读。
// data 列存整条事件的规范 JSON,hash-chain 校验直接对 data 重算(不依赖结构体往返,稳)。
type SQLiteStore struct {
	db       *sql.DB
	mu       sync.Mutex // 串行写(Append/PruneBefore),并护 lastHash 内存缓存
	lastHash string     // 链尾 hash(Open 时从末条恢复;Append 后更新)
	defLimit int
}

const defaultListLimit = 200

const schemaSQL = `
CREATE TABLE IF NOT EXISTS activity_event (
  seq        INTEGER PRIMARY KEY AUTOINCREMENT, -- 权威 append 顺序(=hash-chain 顺序)
  id         TEXT NOT NULL UNIQUE,              -- 业务唯一键(NewID,时间序)
  task_id    TEXT NOT NULL,
  cluster_id TEXT NOT NULL,
  ts         INTEGER NOT NULL,                  -- UTC unix-nano(时间范围过滤)
  prev_hash  TEXT NOT NULL,
  hash       TEXT NOT NULL,
  data       BLOB NOT NULL                      -- 整条 ActivityEvent 的规范 JSON
);
CREATE INDEX IF NOT EXISTS idx_ae_ts ON activity_event(ts);
CREATE INDEX IF NOT EXISTS idx_ae_task ON activity_event(task_id);
CREATE INDEX IF NOT EXISTS idx_ae_cluster ON activity_event(cluster_id);
`

// OpenSQLite 打开(或建)audit DB。path 为 PVC 上的文件路径。
func OpenSQLite(path string) (*SQLiteStore, error) {
	// WAL+NORMAL=吞吐/durability 平衡;busy_timeout 容忍并发读写;foreign_keys 无害置位。
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open audit sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping audit sqlite: %w", err)
	}
	s := &SQLiteStore{db: db, defLimit: defaultListLimit}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate audit schema: %w", err)
	}
	if err := s.loadLastHash(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) loadLastHash() error {
	var h sql.NullString
	err := s.db.QueryRow(`SELECT hash FROM activity_event ORDER BY seq DESC LIMIT 1`).Scan(&h)
	if err == sql.ErrNoRows {
		s.lastHash = ""
		return nil
	}
	if err != nil {
		return fmt.Errorf("load last hash: %w", err)
	}
	s.lastHash = h.String
	return nil
}

// Append 串行入流:填 ID/Timestamp(缺省时)→ 接续 hash-chain → 落库 → 更新链尾。
func (s *SQLiteStore) Append(ctx context.Context, e *ActivityEvent) error {
	if e == nil {
		return fmt.Errorf("nil event")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = NewID()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	} else {
		e.Timestamp = e.Timestamp.UTC()
	}
	e.PrevHash = s.lastHash
	e.Hash = e.ComputeHash()
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO activity_event(id,task_id,cluster_id,ts,prev_hash,hash,data) VALUES(?,?,?,?,?,?,?)`,
		e.ID, e.TaskID, e.ClusterID, e.Timestamp.UnixNano(), e.PrevHash, e.Hash, data)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	s.lastHash = e.Hash
	return nil
}

// List 倒序(最近优先)+ TaskID/集群/时间范围过滤。
func (s *SQLiteStore) List(ctx context.Context, opt ListOpts) ([]ActivityEvent, error) {
	q := `SELECT data FROM activity_event WHERE 1=1`
	var args []any
	if opt.TaskID != "" {
		q += ` AND task_id=?`
		args = append(args, opt.TaskID)
	}
	if opt.Cluster != "" {
		q += ` AND cluster_id=?`
		args = append(args, opt.Cluster)
	}
	if !opt.Since.IsZero() {
		q += ` AND ts>=?`
		args = append(args, opt.Since.UTC().UnixNano())
	}
	if !opt.Until.IsZero() {
		q += ` AND ts<?`
		args = append(args, opt.Until.UTC().UnixNano())
	}
	limit := opt.Limit
	if limit <= 0 {
		limit = s.defLimit
	}
	q += ` ORDER BY seq DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()
	out := make([]ActivityEvent, 0, limit)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var e ActivityEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("unmarshal event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Get 取单条(按业务 id)。
func (s *SQLiteStore) Get(ctx context.Context, id string) (*ActivityEvent, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT data FROM activity_event WHERE id=?`, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil // 未找到=nil,nil(调用方据 nil 判断)
	}
	if err != nil {
		return nil, err
	}
	var e ActivityEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	return &e, nil
}

// PruneBefore 删 ts<cutoff 的旧事件(TTL)。返回删除条数。
// 注:裁掉链首前缀后,剩余首条的 prev_hash 指向已删事件——VerifyChain 只校验相邻链接,故 TTL 兼容。
func (s *SQLiteStore) PruneBefore(ctx context.Context, cutoff time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM activity_event WHERE ts<?`, cutoff.UTC().UnixNano())
	if err != nil {
		return 0, fmt.Errorf("prune: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// VerifyChain 按 seq 升序校验 hash-chain(D2):
//   - 每条 data 重算 hash 须等于存的 hash(内容未被篡改);
//   - 相邻两条 prev_hash 须接上一条 hash(链未断);首条 prev 不强校验(TTL 裁前缀后合法)。
func (s *SQLiteStore) VerifyChain(ctx context.Context) (bool, string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,prev_hash,hash,data FROM activity_event ORDER BY seq ASC`)
	if err != nil {
		return false, "", fmt.Errorf("verify query: %w", err)
	}
	defer rows.Close()
	prev := ""
	first := true
	for rows.Next() {
		var id, ph, h string
		var data []byte
		if err := rows.Scan(&id, &ph, &h, &data); err != nil {
			return false, "", err
		}
		var e ActivityEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return false, id, fmt.Errorf("unmarshal %s: %w", id, err)
		}
		if e.ComputeHash() != h { // 内容被篡改 → 重算对不上
			return false, id, nil
		}
		if !first && ph != prev { // 链断(中间被删/重排)
			return false, id, nil
		}
		prev = h
		first = false
	}
	return true, "", rows.Err()
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

// 确保 *SQLiteStore 实现 AuditStore。
var _ AuditStore = (*SQLiteStore)(nil)
