# audit store Phase 0 选型基准(ADR-039)

独立 module(自带 go.mod,父构建 `go build ./...` 跳过),不污染 supkube-backend 依赖。
复现:`cd internal/audit/bench && go run .`。工作负载对齐 PRD-008 DoD #19。

实测结论(2026-06-18):SQLite(modernc 纯 Go)唯一三项全过 → ADR-039 主选。
