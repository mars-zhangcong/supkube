package v1_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	v1 "github.com/supkube/supkube-backend/internal/api/v1"
	"github.com/supkube/supkube-backend/internal/auth"
	"github.com/supkube/supkube-backend/internal/version"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// In production these handlers sit behind the auth middleware, which
	// populates the context with the authenticated user. The handlers' RBAC
	// checks (auth.RequireNamespaceAccess) abort with 401 when no user is
	// present, so we inject an admin here to exercise the handler logic
	// itself rather than re-testing the auth gate (which has its own tests).
	r.Use(func(c *gin.Context) {
		c.Set("user", &auth.User{Subject: "test", Username: "test", Role: auth.RoleAdmin})
		c.Next()
	})

	api := r.Group("/api/v1")
	{
		api.GET("/status", v1.GetStatus)
		api.GET("/namespaces", v1.ListNamespaces)
		api.GET("/backups", v1.ListBackups)
		api.POST("/backups", v1.CreateBackup)
		api.GET("/backups/:name", v1.GetBackup)
		api.DELETE("/backups/:name", v1.DeleteBackup)
		api.GET("/restores", v1.ListRestores)
		api.POST("/restores", v1.CreateRestore)
		api.GET("/restores/:name", v1.GetRestore)
	}

	return r
}

// --- Status endpoint ---

func TestGetStatus(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%v'", resp["status"])
	}
	// version.Version is "dev" unless overridden via -ldflags at build time
	// (the Dockerfile injects the real release). Assert the handler is wired
	// to the version package rather than hardcoding a literal that goes stale.
	if resp["version"] != version.Version {
		t.Errorf("expected version '%s' (version.Version), got '%v'", version.Version, resp["version"])
	}
}

// --- Backup endpoints ---

// TestListBackups_NoCluster tests that ListBackups returns an error
// when no Kubernetes cluster is available (expected in CI/test env).
func TestListBackups_NoCluster(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/backups", nil)
	router.ServeHTTP(w, req)

	// Without a cluster, we expect 500 with an error message
	if w.Code != http.StatusInternalServerError {
		t.Logf("response: %s", w.Body.String())
		// If we get 200, a cluster is available — that's also fine
		if w.Code == http.StatusOK {
			t.Log("cluster available, got 200 — validating response structure")
			var resp map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			// BackupList should have items field (may be null or array)
			if _, ok := resp["items"]; !ok {
				t.Error("expected 'items' field in backup list response")
			}
		} else {
			t.Errorf("expected status 500 (no cluster) or 200 (cluster available), got %d", w.Code)
		}
	} else {
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if _, ok := resp["error"]; !ok {
			t.Error("expected 'error' field in error response")
		}
	}
}

func TestCreateBackup_InvalidBody(t *testing.T) {
	router := setupRouter()

	// Test with empty body
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/backups", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing required field 'name', got %d", w.Code)
	}
}

func TestCreateBackup_ValidBody_NoCluster(t *testing.T) {
	router := setupRouter()

	body := map[string]interface{}{
		"name":               "test-backup-1",
		"includedNamespaces": []string{"default"},
		"ttl":                "24h",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/backups", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Validation order: the handler asserts the BSL (backup storage location)
	// exists FIRST and returns an actionable 422 when it doesn't (see
	// handlers.go assertBSLExists). So without a configured BSL → 422; without a
	// cluster connection → 500; with a valid cluster + BSL → 201.
	// (Pre-BSL-validation this path only returned 500/201; this test predates it.)
	if w.Code != http.StatusUnprocessableEntity &&
		w.Code != http.StatusInternalServerError &&
		w.Code != http.StatusCreated {
		t.Errorf("expected 422 (BSL not found) / 500 (no cluster) / 201 (created), got %d", w.Code)
	}
}

func TestGetBackup_NoCluster(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/backups/nonexistent", nil)
	router.ServeHTTP(w, req)

	// Without cluster: 500 (can't connect); with cluster: 404 (not found)
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Errorf("expected status 500 or 404, got %d", w.Code)
	}
}

func TestDeleteBackup_NoCluster(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/backups/nonexistent", nil)
	router.ServeHTTP(w, req)

	// Without cluster: 500; with cluster: 404
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Errorf("expected status 500 or 404, got %d", w.Code)
	}
}

// --- Restore endpoints ---

func TestListRestores_NoCluster(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/restores", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Errorf("expected status 500 (no cluster) or 200, got %d", w.Code)
	}
}

func TestCreateRestore_InvalidBody(t *testing.T) {
	router := setupRouter()

	// Missing required fields
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/restores", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing required fields, got %d", w.Code)
	}
}

func TestCreateRestore_MissingBackupName(t *testing.T) {
	router := setupRouter()

	body := map[string]interface{}{
		"name": "test-restore-1",
		// backupName is required but missing
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/restores", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing backupName, got %d", w.Code)
	}
}

func TestCreateRestore_ValidBody_NoCluster(t *testing.T) {
	router := setupRouter()

	body := map[string]interface{}{
		"name":               "test-restore-1",
		"backupName":         "test-backup-1",
		"includedNamespaces": []string{"default"},
		"namespaceMapping":   map[string]string{"source-ns": "target-ns"},
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/restores", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Without cluster: 500; with cluster: 201
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusCreated {
		t.Errorf("expected status 500 (no cluster) or 201 (created), got %d", w.Code)
	}
}

func TestGetRestore_NoCluster(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/restores/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Errorf("expected status 500 or 404, got %d", w.Code)
	}
}

// --- Input validation tests ---

func TestCreateBackup_MalformedJSON(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/backups", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for malformed JSON, got %d", w.Code)
	}
}

func TestCreateRestore_MalformedJSON(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/restores", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for malformed JSON, got %d", w.Code)
	}
}

// --- CORS test ---

func TestCORSHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Add CORS middleware matching server.go
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.GET("/api/v1/status", v1.GetStatus)

	// Test OPTIONS preflight
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/api/v1/status", nil)
	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS Allow-Origin header")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("missing CORS Allow-Methods header")
	}
}
