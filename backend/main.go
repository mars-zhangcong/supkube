package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	db *pgxpool.Pool
}

type RestorePoint struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	CompanyName      string    `json:"company_name"`
	Owner            string    `json:"owner"`
	LifecycleStage   string    `json:"lifecycle_stage"`
	Status           string    `json:"status"`
	LatestBackupTime time.Time `json:"latest_backup_time"`
	RPOMinutes       int       `json:"rpo_minutes"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	AgeMinutes       int64     `json:"age_minutes"`
	AgeDisplay       string    `json:"age_display"`
	RPOBreached      bool      `json:"rpo_breached"`
}

type RestorePointInput struct {
	Name             string    `json:"name"`
	CompanyName      string    `json:"company_name"`
	Owner            string    `json:"owner"`
	LifecycleStage   string    `json:"lifecycle_stage"`
	Status           string    `json:"status"`
	LatestBackupTime time.Time `json:"latest_backup_time"`
	RPOMinutes       int       `json:"rpo_minutes"`
}

type ListResponse struct {
	Items    []RestorePoint `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

type OptionsResponse struct {
	LifecycleStages []string `json:"lifecycle_stages"`
	Statuses        []string `json:"statuses"`
}

var allowedLifecycleStages = map[string]bool{
	"planning":    true,
	"active":      true,
	"maintenance": true,
	"retired":     true,
}

var allowedStatuses = map[string]bool{
	"healthy":  true,
	"warning":  true,
	"critical": true,
	"disabled": true,
}

var sortableColumns = map[string]string{
	"id":                 "id",
	"name":               "name",
	"company_name":       "company_name",
	"owner":              "owner",
	"lifecycle_stage":    "lifecycle_stage",
	"status":             "status",
	"latest_backup_time": "latest_backup_time",
	"rpo_minutes":        "rpo_minutes",
	"created_at":         "created_at",
	"updated_at":         "updated_at",
	"age_minutes":        "latest_backup_time",
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("LF_PG_DSN")
	}
	if dsn == "" {
		log.Fatal("DATABASE_URL or LF_PG_DSN is required")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	app := &App{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/options", app.handleOptions)
	mux.HandleFunc("/api/restore-points", app.handleRestorePoints)
	mux.HandleFunc("/api/restore-points/", app.handleRestorePointByID)

	handler := corsMiddleware(loggingMiddleware(mux))
	addr := ":8080"
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

func (a *App) handleOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, OptionsResponse{
		LifecycleStages: []string{"planning", "active", "maintenance", "retired"},
		Statuses:        []string{"healthy", "warning", "critical", "disabled"},
	})
}

func (a *App) handleRestorePoints(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listRestorePoints(w, r)
	case http.MethodPost:
		a.createRestorePoint(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (a *App) handleRestorePointByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/restore-points/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.getRestorePoint(w, r, id)
	case http.MethodPut:
		a.updateRestorePoint(w, r, id)
	case http.MethodDelete:
		a.deleteRestorePoint(w, r, id)
	default:
		methodNotAllowed(w)
	}
}

func (a *App) listRestorePoints(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := parseIntDefault(q.Get("page"), 1)
	pageSize := parseIntDefault(q.Get("page_size"), 10)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	sortBy := q.Get("sort_by")
	if sortBy == "" {
		sortBy = "latest_backup_time"
	}
	column, ok := sortableColumns[sortBy]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid sort_by")
		return
	}
	sortOrder := strings.ToLower(q.Get("sort_order"))
	if sortOrder == "" {
		sortOrder = "desc"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		writeError(w, http.StatusBadRequest, "invalid sort_order")
		return
	}

	where := []string{"1=1"}
	args := []any{}
	argPos := 1

	if v := strings.TrimSpace(q.Get("q")); v != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR company_name ILIKE $%d OR owner ILIKE $%d)", argPos, argPos, argPos))
		args = append(args, "%"+v+"%")
		argPos++
	}
	if v := strings.TrimSpace(q.Get("company_name")); v != "" {
		where = append(where, fmt.Sprintf("company_name ILIKE $%d", argPos))
		args = append(args, "%"+v+"%")
		argPos++
	}
	if v := strings.TrimSpace(q.Get("owner")); v != "" {
		where = append(where, fmt.Sprintf("owner ILIKE $%d", argPos))
		args = append(args, "%"+v+"%")
		argPos++
	}
	if v := strings.TrimSpace(q.Get("lifecycle_stage")); v != "" {
		if !allowedLifecycleStages[v] {
			writeError(w, http.StatusBadRequest, "invalid lifecycle_stage")
			return
		}
		where = append(where, fmt.Sprintf("lifecycle_stage = $%d", argPos))
		args = append(args, v)
		argPos++
	}
	if v := strings.TrimSpace(q.Get("status")); v != "" {
		if !allowedStatuses[v] {
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
		where = append(where, fmt.Sprintf("status = $%d", argPos))
		args = append(args, v)
		argPos++
	}
	if v := strings.TrimSpace(q.Get("rpo_breached")); v != "" {
		if v != "true" && v != "false" {
			writeError(w, http.StatusBadRequest, "invalid rpo_breached")
			return
		}
		if v == "true" {
			where = append(where, "EXTRACT(EPOCH FROM (NOW() - latest_backup_time)) / 60 > rpo_minutes")
		} else {
			where = append(where, "EXTRACT(EPOCH FROM (NOW() - latest_backup_time)) / 60 <= rpo_minutes")
		}
	}

	whereSQL := strings.Join(where, " AND ")
	ctx := r.Context()

	countSQL := "SELECT COUNT(*) FROM restore_points WHERE " + whereSQL
	var total int
	if err := a.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	offset := (page - 1) * pageSize
	orderSQL := column + " " + sortOrder
	if sortBy == "age_minutes" {
		if sortOrder == "asc" {
			orderSQL = "latest_backup_time DESC"
		} else {
			orderSQL = "latest_backup_time ASC"
		}
	}
	if sortBy != "id" {
		orderSQL += ", id DESC"
	}

	listSQL := `
		SELECT id, name, company_name, owner, lifecycle_stage, status, latest_backup_time, rpo_minutes, created_at, updated_at
		FROM restore_points
		WHERE ` + whereSQL + `
		ORDER BY ` + orderSQL + `
		LIMIT $` + strconv.Itoa(argPos) + ` OFFSET $` + strconv.Itoa(argPos+1)
	args = append(args, pageSize, offset)

	rows, err := a.db.Query(ctx, listSQL, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := make([]RestorePoint, 0)
	now := time.Now()
	for rows.Next() {
		var rp RestorePoint
		if err := rows.Scan(&rp.ID, &rp.Name, &rp.CompanyName, &rp.Owner, &rp.LifecycleStage, &rp.Status, &rp.LatestBackupTime, &rp.RPOMinutes, &rp.CreatedAt, &rp.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		enrichRestorePoint(&rp, now)
		items = append(items, rp)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ListResponse{Items: items, Total: total, Page: page, PageSize: pageSize})
}

func (a *App) getRestorePoint(w http.ResponseWriter, r *http.Request, id int64) {
	ctx := r.Context()
	var rp RestorePoint
	err := a.db.QueryRow(ctx, `
		SELECT id, name, company_name, owner, lifecycle_stage, status, latest_backup_time, rpo_minutes, created_at, updated_at
		FROM restore_points WHERE id=$1
	`, id).Scan(&rp.ID, &rp.Name, &rp.CompanyName, &rp.Owner, &rp.LifecycleStage, &rp.Status, &rp.LatestBackupTime, &rp.RPOMinutes, &rp.CreatedAt, &rp.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "restore point not found")
		return
	}
	enrichRestorePoint(&rp, time.Now())
	writeJSON(w, http.StatusOK, rp)
}

func (a *App) createRestorePoint(w http.ResponseWriter, r *http.Request) {
	var in RestorePointInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateInput(in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	var rp RestorePoint
	err := a.db.QueryRow(ctx, `
		INSERT INTO restore_points (name, company_name, owner, lifecycle_stage, status, latest_backup_time, rpo_minutes, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())
		RETURNING id, name, company_name, owner, lifecycle_stage, status, latest_backup_time, rpo_minutes, created_at, updated_at
	`, in.Name, in.CompanyName, in.Owner, in.LifecycleStage, in.Status, in.LatestBackupTime, in.RPOMinutes).Scan(
		&rp.ID, &rp.Name, &rp.CompanyName, &rp.Owner, &rp.LifecycleStage, &rp.Status, &rp.LatestBackupTime, &rp.RPOMinutes, &rp.CreatedAt, &rp.UpdatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	enrichRestorePoint(&rp, time.Now())
	writeJSON(w, http.StatusCreated, rp)
}

func (a *App) updateRestorePoint(w http.ResponseWriter, r *http.Request, id int64) {
	var in RestorePointInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateInput(in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	var rp RestorePoint
	err := a.db.QueryRow(ctx, `
		UPDATE restore_points
		SET name=$2, company_name=$3, owner=$4, lifecycle_stage=$5, status=$6, latest_backup_time=$7, rpo_minutes=$8, updated_at=NOW()
		WHERE id=$1
		RETURNING id, name, company_name, owner, lifecycle_stage, status, latest_backup_time, rpo_minutes, created_at, updated_at
	`, id, in.Name, in.CompanyName, in.Owner, in.LifecycleStage, in.Status, in.LatestBackupTime, in.RPOMinutes).Scan(
		&rp.ID, &rp.Name, &rp.CompanyName, &rp.Owner, &rp.LifecycleStage, &rp.Status, &rp.LatestBackupTime, &rp.RPOMinutes, &rp.CreatedAt, &rp.UpdatedAt,
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "restore point not found")
		return
	}
	enrichRestorePoint(&rp, time.Now())
	writeJSON(w, http.StatusOK, rp)
}

func (a *App) deleteRestorePoint(w http.ResponseWriter, r *http.Request, id int64) {
	ctx := r.Context()
	cmd, err := a.db.Exec(ctx, `DELETE FROM restore_points WHERE id=$1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cmd.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "restore point not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func validateInput(in RestorePointInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(in.CompanyName) == "" {
		return errors.New("company_name is required")
	}
	if strings.TrimSpace(in.Owner) == "" {
		return errors.New("owner is required")
	}
	if !allowedLifecycleStages[in.LifecycleStage] {
		return errors.New("invalid lifecycle_stage")
	}
	if !allowedStatuses[in.Status] {
		return errors.New("invalid status")
	}
	if in.LatestBackupTime.IsZero() {
		return errors.New("latest_backup_time is required")
	}
	if in.RPOMinutes <= 0 {
		return errors.New("rpo_minutes must be greater than 0")
	}
	return nil
}

func enrichRestorePoint(rp *RestorePoint, now time.Time) {
	age := now.Sub(rp.LatestBackupTime)
	if age < 0 {
		age = 0
	}
	rp.AgeMinutes = int64(age / time.Minute)
	rp.AgeDisplay = formatDuration(age)
	rp.RPOBreached = rp.AgeMinutes > int64(rp.RPOMinutes)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "刚刚"
	}
	minutes := int64(d / time.Minute)
	days := minutes / (24 * 60)
	minutes %= 24 * 60
	hours := minutes / 60
	minutes %= 60
	parts := make([]string, 0)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d天", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d小时", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d分钟", minutes))
	}
	if len(parts) == 0 {
		return "刚刚"
	}
	return strings.Join(parts, "")
}

func parseIntDefault(v string, def int) int {
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
