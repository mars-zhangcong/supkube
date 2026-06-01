// cron.go — 5-field cron parser + next-trigger 计算.
//
// 为什么不用 robfig/cron/v3: 当前 go.mod 没引入, sprint 约束 "不引入新依
// 赖". 5-field cron (分 时 日 月 周) 语法对 ImportPolicy 已经足够, 而且
// 我们只需要 "Parse + Next(time)" 两个 API.
//
// 支持的语法 (与 Velero schedule 默认子集一致):
//   - 任意值
//     N           固定值 (N 在字段值域内)
//     N-M         区间 (闭区间)
//     N,M,...     列表
//     */N         步长 (等价于 0,N,2N,... 在字段值域内)
//     N-M/S       区间 + 步长
//
// 不支持: 命名月份/周几 (jan/mon), L/W/#, ?, @yearly 等 alias. 不打算支
// 持 — Velero 也只默认放行数字语法, admin 想用 cron alias 用 @hourly 写
// 成 "0 * * * *" 即可.
package importpolicy

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronSchedule 是 ParseCron 的返回. Next(t) 给定一个时间, 返回下一次触
// 发的时间 (在 t 之后, 不含 t 本身).
type CronSchedule struct {
	minute     []int // 0-59
	hour       []int // 0-23
	dayOfMonth []int // 1-31
	month      []int // 1-12
	dayOfWeek  []int // 0-6 (0=Sun, 7 mapped to 0)
}

// ParseCron 解析 5-field 表达式. 失败返回 error (会被 handler 转成
// ERR_IMPORTPOLICY_CRON_INVALID).
func ParseCron(expr string) (*CronSchedule, error) {
	expr = strings.TrimSpace(expr)
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron must have exactly 5 fields (minute hour dom month dow), got %d", len(fields))
	}
	s := &CronSchedule{}
	var err error
	if s.minute, err = parseField(fields[0], 0, 59); err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	if s.hour, err = parseField(fields[1], 0, 23); err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	if s.dayOfMonth, err = parseField(fields[2], 1, 31); err != nil {
		return nil, fmt.Errorf("dayOfMonth: %w", err)
	}
	if s.month, err = parseField(fields[3], 1, 12); err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	if s.dayOfWeek, err = parseField(fields[4], 0, 7); err != nil {
		return nil, fmt.Errorf("dayOfWeek: %w", err)
	}
	// 把 7 (Sunday alias) 归一到 0
	for i, d := range s.dayOfWeek {
		if d == 7 {
			s.dayOfWeek[i] = 0
		}
	}
	s.dayOfWeek = dedupSort(s.dayOfWeek)
	return s, nil
}

// parseField 把单个字段展开成有序去重的 int 列表.
func parseField(field string, min, max int) ([]int, error) {
	out := map[int]struct{}{}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty list element")
		}
		// step (a/b 形式)
		step := 1
		core := part
		if idx := strings.Index(part, "/"); idx >= 0 {
			core = part[:idx]
			stepStr := part[idx+1:]
			s, err := strconv.Atoi(stepStr)
			if err != nil || s <= 0 {
				return nil, fmt.Errorf("invalid step %q", stepStr)
			}
			step = s
		}
		var lo, hi int
		switch {
		case core == "*":
			lo, hi = min, max
		case strings.Contains(core, "-"):
			r := strings.SplitN(core, "-", 2)
			a, errA := strconv.Atoi(r[0])
			b, errB := strconv.Atoi(r[1])
			if errA != nil || errB != nil {
				return nil, fmt.Errorf("invalid range %q", core)
			}
			lo, hi = a, b
		default:
			v, err := strconv.Atoi(core)
			if err != nil {
				return nil, fmt.Errorf("invalid value %q", core)
			}
			lo, hi = v, v
		}
		if lo < min || hi > max || lo > hi {
			return nil, fmt.Errorf("value %d-%d out of range [%d,%d]", lo, hi, min, max)
		}
		for v := lo; v <= hi; v += step {
			out[v] = struct{}{}
		}
	}
	res := make([]int, 0, len(out))
	for k := range out {
		res = append(res, k)
	}
	return dedupSort(res), nil
}

// dedupSort 排序并去重 (parseField 已用 map 去重, 这里只排序; List=fast).
func dedupSort(in []int) []int {
	// 简易插入排序; cron 字段最多 60 个值, O(n^2) 无所谓且零依赖.
	out := append([]int(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	// 去重 (出问题时防御性).
	w := 0
	for i := range out {
		if w == 0 || out[w-1] != out[i] {
			out[w] = out[i]
			w++
		}
	}
	return out[:w]
}

// Next 返回 after 之后下一次满足表达式的时间 (本地时区 = after 的时区).
// 算法: 暴力 minute-by-minute 向前走, 上限 4 年 (闰年/2月29日组合的最坏
// 情况). 4 年还找不到说明表达式不可能 trigger (e.g. "0 0 31 2 *" — 2月31号
// 不存在), 返回 zero time. 对 ImportPolicy 这个频率 (分钟级触发) 完全够;
// 谁要写跨年才触发一次的 cron 也根本不该用 Scheduled mode.
func (s *CronSchedule) Next(after time.Time) time.Time {
	// 从 after 的下一分钟整点开始 (秒/纳秒清零).
	t := after.Truncate(time.Minute).Add(time.Minute)
	limit := t.Add(4 * 366 * 24 * time.Hour)
	for t.Before(limit) {
		if s.matches(t) {
			return t
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}
}

// matches 检查时间 t 是否满足表达式. cron 的 day-of-month 和 day-of-week
// 的语义是 OR (POSIX): 任一字段非*时, 满足其一即可; 都是*则两个都默认通
// 过. 这里按 POSIX 标准实现.
func (s *CronSchedule) matches(t time.Time) bool {
	if !contains(s.minute, t.Minute()) {
		return false
	}
	if !contains(s.hour, t.Hour()) {
		return false
	}
	if !contains(s.month, int(t.Month())) {
		return false
	}
	// dom + dow OR semantics
	domStar := isFullRange(s.dayOfMonth, 1, 31)
	dowStar := isFullRange(s.dayOfWeek, 0, 6)
	domMatch := contains(s.dayOfMonth, t.Day())
	dowMatch := contains(s.dayOfWeek, int(t.Weekday()))
	switch {
	case domStar && dowStar:
		return true
	case domStar && !dowStar:
		return dowMatch
	case !domStar && dowStar:
		return domMatch
	default:
		return domMatch || dowMatch
	}
}

func contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// isFullRange 判断字段是不是 "*" (即覆盖整个值域).
func isFullRange(xs []int, min, max int) bool {
	if len(xs) != max-min+1 {
		return false
	}
	for i, v := range xs {
		if v != min+i {
			return false
		}
	}
	return true
}
