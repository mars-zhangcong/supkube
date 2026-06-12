package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type restoreResultsHandler struct {
	cfg *rest.Config
}

type restoreResultItem struct {
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	Phase             string `json:"phase"`
	Warnings          int    `json:"warnings"`
	Errors            int    `json:"errors"`
	StartedAt         string `json:"startedAt,omitempty"`
	CompletedAt       string `json:"completedAt,omitempty"`
	BackupName        string `json:"backupName,omitempty"`
	ScheduleName      string `json:"scheduleName,omitempty"`
	IncludedResources int    `json:"includedResources,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`
	Age               string `json:"age,omitempty"`
	Stale             bool   `json:"stale"`
	Message           string `json:"message,omitempty"`
}

func RegisterRestoreResultsRoutes(mux *http.ServeMux, cfg *rest.Config) {
	h := &restoreResultsHandler{cfg: cfg}
	mux.HandleFunc("/api/v1/restore-results", h.handle)
}

func (h *restoreResultsHandler) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	clientset, err := kubernetes.NewForConfig(h.cfg)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("build kubernetes client: %v", err))
		return
	}

	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	staleOnly := parseBoolQuery(r, "staleOnly")
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	data, err := clientset.RESTClient().Get().AbsPath("/apis/velero.io/v1/restores").DoRaw(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("list restores: %v", err))
		return
	}

	var list map[string]any
	if err := json.Unmarshal(data, &list); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("decode restores: %v", err))
		return
	}

	itemsAny, _ := list["items"].([]any)
	items := make([]restoreResultItem, 0, len(itemsAny))
	for _, raw := range itemsAny {
		obj, _ := raw.(map[string]any)
		if obj == nil {
			continue
		}
		meta, _ := obj["metadata"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		status, _ := obj["status"].(map[string]any)

		itemNamespace := stringValue(meta, "namespace")
		if namespace != metav1.NamespaceAll && itemNamespace != namespace {
			continue
		}

		item := restoreResultItem{
			Name:              stringValue(meta, "name"),
			Namespace:         itemNamespace,
			Phase:             stringValue(status, "phase"),
			Warnings:          intValue(status, "warnings"),
			Errors:            intValue(status, "errors"),
			StartedAt:         stringValue(status, "startTimestamp"),
			CompletedAt:       stringValue(status, "completionTimestamp"),
			BackupName:        stringValue(spec, "backupName"),
			ScheduleName:      nestedStringValue(spec, "scheduleName"),
			IncludedResources: intValue(status, "itemsRestored"),
			CreatedAt:         stringValue(meta, "creationTimestamp"),
			Message:           stringValue(status, "validationErrors"),
		}

		if item.Message == "" {
			item.Message = stringValue(status, "failureReason")
		}
		if item.StartedAt == "" {
			item.StartedAt = item.CreatedAt
		}
		if item.CompletedAt == "" {
			item.CompletedAt = stringValue(status, "completionTimestamp")
		}
		item.Age = humanizeAge(item.CreatedAt)
		item.Stale = computeRestoreStale(item)
		if staleOnly && !item.Stale {
			continue
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return sortTimeDesc(items[i].CreatedAt, items[j].CreatedAt)
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

func computeRestoreStale(item restoreResultItem) bool {
	phase := strings.ToLower(strings.TrimSpace(item.Phase))
	if phase == "partiallyfailed" || phase == "failedvalidation" || phase == "failed" {
		return true
	}
	if item.Errors > 0 {
		return true
	}
	completedAt := strings.TrimSpace(item.CompletedAt)
	startedAt := strings.TrimSpace(item.StartedAt)
	base := completedAt
	if base == "" {
		base = startedAt
	}
	if base == "" {
		base = item.CreatedAt
	}
	if base == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, base)
	if err != nil {
		return false
	}
	return time.Since(parsed) > 7*24*time.Hour
}

func parseBoolQuery(r *http.Request, key string) bool {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return v
}

func stringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func nestedStringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func intValue(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

func humanizeAge(ts string) string {
	if strings.TrimSpace(ts) == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func sortTimeDesc(a, b string) bool {
	ta, errA := time.Parse(time.RFC3339, a)
	tb, errB := time.Parse(time.RFC3339, b)
	if errA != nil && errB != nil {
		return a > b
	}
	if errA != nil {
		return false
	}
	if errB != nil {
		return true
	}
	return ta.After(tb)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
