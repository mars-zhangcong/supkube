package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// E2E tests require a running backend at localhost:8080
// and a K8S cluster with Velero installed.
//
// Run with: go test ./e2e/ -v -tags=e2e -timeout 60s

const baseURL = "http://localhost:8080/api/v1"

func requireBackend(t *testing.T) {
	t.Helper()
	resp, err := http.Get(baseURL + "/status")
	if err != nil {
		t.Skipf("Backend not available at %s: %v", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skipf("Backend returned status %d", resp.StatusCode)
	}
}

// --- E2E: Full backup lifecycle ---

func TestE2E_BackupLifecycle(t *testing.T) {
	requireBackend(t)

	backupName := fmt.Sprintf("e2e-backup-%d", time.Now().Unix())

	// Step 1: Create a backup
	t.Run("CreateBackup", func(t *testing.T) {
		body := map[string]interface{}{
			"name":               backupName,
			"includedNamespaces": []string{"default"},
			"ttl":                "24h",
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(baseURL+"/backups", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("failed to create backup: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		metadata := result["metadata"].(map[string]interface{})
		if metadata["name"] != backupName {
			t.Errorf("expected backup name %s, got %v", backupName, metadata["name"])
		}
	})

	// Step 2: Verify backup appears in list
	t.Run("ListContainsBackup", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/backups")
		if err != nil {
			t.Fatalf("failed to list backups: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		items := result["items"].([]interface{})

		found := false
		for _, item := range items {
			backup := item.(map[string]interface{})
			metadata := backup["metadata"].(map[string]interface{})
			if metadata["name"] == backupName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("backup %s not found in list", backupName)
		}
	})

	// Step 3: Get specific backup
	t.Run("GetBackup", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/backups/" + backupName)
		if err != nil {
			t.Fatalf("failed to get backup: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		metadata := result["metadata"].(map[string]interface{})
		if metadata["name"] != backupName {
			t.Errorf("expected name %s, got %v", backupName, metadata["name"])
		}
	})

	// Step 4: Delete backup
	t.Run("DeleteBackup", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", baseURL+"/backups/"+backupName, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to delete backup: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
		}
	})

	// Step 5: Verify backup is gone
	t.Run("VerifyDeleted", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/backups/" + backupName)
		if err != nil {
			t.Fatalf("failed to get backup: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404 after deletion, got %d", resp.StatusCode)
		}
	})
}

// --- E2E: Restore lifecycle ---

func TestE2E_RestoreLifecycle(t *testing.T) {
	requireBackend(t)

	// Use existing test-backup for restore (known to exist from Phase 1 verification)
	restoreName := fmt.Sprintf("e2e-restore-%d", time.Now().Unix())

	// Step 1: Create a restore from test-backup
	t.Run("CreateRestore", func(t *testing.T) {
		body := map[string]interface{}{
			"name":               restoreName,
			"backupName":         "test-backup",
			"includedNamespaces": []string{"default"},
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(baseURL+"/restores", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("failed to create restore: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(bodyBytes))
		}
	})

	// Step 2: Verify restore appears in list
	t.Run("ListContainsRestore", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/restores")
		if err != nil {
			t.Fatalf("failed to list restores: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		items := result["items"].([]interface{})

		found := false
		for _, item := range items {
			restore := item.(map[string]interface{})
			metadata := restore["metadata"].(map[string]interface{})
			if metadata["name"] == restoreName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("restore %s not found in list", restoreName)
		}
	})

	// Step 3: Get specific restore
	t.Run("GetRestore", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/restores/" + restoreName)
		if err != nil {
			t.Fatalf("failed to get restore: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		spec := result["spec"].(map[string]interface{})
		if spec["backupName"] != "test-backup" {
			t.Errorf("expected backupName 'test-backup', got %v", spec["backupName"])
		}
	})
}

// --- E2E: Namespace listing ---

func TestE2E_NamespaceList(t *testing.T) {
	requireBackend(t)

	resp, err := http.Get(baseURL + "/namespaces")
	if err != nil {
		t.Fatalf("failed to list namespaces: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Verify unified format
	items, ok := result["items"].([]interface{})
	if !ok {
		t.Fatal("expected 'items' to be an array")
	}
	if len(items) == 0 {
		t.Error("expected at least one namespace")
	}

	total, ok := result["total"].(float64)
	if !ok {
		t.Fatal("expected 'total' field")
	}
	if int(total) != len(items) {
		t.Errorf("total (%d) doesn't match items length (%d)", int(total), len(items))
	}

	// Verify known namespaces exist
	namespaceSet := make(map[string]bool)
	for _, item := range items {
		namespaceSet[item.(string)] = true
	}
	for _, expected := range []string{"default", "velero", "kube-system"} {
		if !namespaceSet[expected] {
			t.Errorf("expected namespace %s not found", expected)
		}
	}
}

// --- E2E: Dashboard summary ---

func TestE2E_DashboardSummary(t *testing.T) {
	requireBackend(t)

	resp, err := http.Get(baseURL + "/dashboard/summary")
	if err != nil {
		t.Fatalf("failed to get dashboard summary: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Verify cluster info
	cluster, ok := result["cluster"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'cluster' object in response")
	}
	if _, ok := cluster["nodes"]; !ok {
		t.Error("expected 'nodes' in cluster")
	}
	if _, ok := cluster["namespaces"]; !ok {
		t.Error("expected 'namespaces' in cluster")
	}
	if cluster["nodes"].(float64) < 1 {
		t.Error("expected at least 1 node")
	}
	if cluster["namespaces"].(float64) < 1 {
		t.Error("expected at least 1 namespace")
	}

	// Verify backup summary
	backupSummary, ok := result["backupSummary"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'backupSummary' object in response")
	}
	for _, field := range []string{"total", "completed", "failed", "inProgress"} {
		if _, ok := backupSummary[field]; !ok {
			t.Errorf("expected '%s' in backupSummary", field)
		}
	}

	// Verify recent backups
	recentBackups, ok := result["recentBackups"].([]interface{})
	if !ok {
		t.Fatal("expected 'recentBackups' array in response")
	}
	if len(recentBackups) > 0 {
		firstBackup := recentBackups[0].(map[string]interface{})
		for _, field := range []string{"name", "namespace", "phase", "createdAt"} {
			if _, ok := firstBackup[field]; !ok {
				t.Errorf("expected '%s' in recent backup entry", field)
			}
		}
	}

	// Verify storage locations count
	if _, ok := result["storageLocations"]; !ok {
		t.Error("expected 'storageLocations' in response")
	}
}

// --- E2E: Schedule pause/resume ---

func TestE2E_SchedulePauseResume(t *testing.T) {
	requireBackend(t)

	scheduleName := fmt.Sprintf("e2e-schedule-%d", time.Now().Unix())

	// Step 1: Create a schedule
	t.Run("CreateSchedule", func(t *testing.T) {
		body := map[string]interface{}{
			"name":               scheduleName,
			"schedule":           "0 2 * * *",
			"includedNamespaces": []string{"default"},
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(baseURL+"/schedules", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("failed to create schedule: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(bodyBytes))
		}
	})

	// Step 2: Pause the schedule
	t.Run("PauseSchedule", func(t *testing.T) {
		body := map[string]interface{}{
			"paused": true,
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("PATCH", baseURL+"/schedules/"+scheduleName, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to pause schedule: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
		}
	})

	// Step 3: Resume the schedule
	t.Run("ResumeSchedule", func(t *testing.T) {
		body := map[string]interface{}{
			"paused": false,
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("PATCH", baseURL+"/schedules/"+scheduleName, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to resume schedule: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
		}
	})

	// Step 4: Cleanup - delete the schedule
	t.Run("DeleteSchedule", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", baseURL+"/schedules/"+scheduleName, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to delete schedule: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
		}
	})
}

// --- E2E: Applications API ---

func TestE2E_ApplicationsList(t *testing.T) {
	requireBackend(t)

	resp, err := http.Get(baseURL + "/applications")
	if err != nil {
		t.Fatalf("failed to get applications: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Verify response structure
	items, ok := result["items"].([]interface{})
	if !ok {
		t.Fatal("expected 'items' array in response")
	}

	total, ok := result["total"].(float64)
	if !ok {
		t.Fatal("expected 'total' field in response")
	}
	if int(total) != len(items) {
		t.Errorf("total (%d) doesn't match items length (%d)", int(total), len(items))
	}

	// Verify each item has required fields
	for i, item := range items {
		app := item.(map[string]interface{})
		if _, ok := app["namespace"]; !ok {
			t.Errorf("item %d: expected 'namespace' field", i)
		}
		if _, ok := app["workloads"]; !ok {
			t.Errorf("item %d: expected 'workloads' field", i)
		}
		if _, ok := app["protected"]; !ok {
			t.Errorf("item %d: expected 'protected' field", i)
		}
		// Protected namespaces should have lastBackupTime and lastBackupName
		if protected, ok := app["protected"].(bool); ok && protected {
			if _, ok := app["lastBackupTime"]; !ok {
				t.Errorf("item %d: protected namespace should have 'lastBackupTime'", i)
			}
			if _, ok := app["lastBackupName"]; !ok {
				t.Errorf("item %d: protected namespace should have 'lastBackupName'", i)
			}
		}
	}

	// Verify system namespaces are excluded (kube-system, kube-public, velero)
	for _, item := range items {
		app := item.(map[string]interface{})
		ns := app["namespace"].(string)
		if ns == "kube-system" || ns == "kube-public" || ns == "velero" || ns == "kube-node-lease" {
			t.Errorf("system namespace %s should be excluded from applications list", ns)
		}
	}
}

// --- E2E: Error handling ---

func TestE2E_GetNonexistentBackup(t *testing.T) {
	requireBackend(t)

	resp, err := http.Get(baseURL + "/backups/does-not-exist-12345")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if _, ok := result["error"]; !ok {
		t.Error("expected 'error' field in response")
	}
}

func TestE2E_CreateBackupInvalidInput(t *testing.T) {
	requireBackend(t)

	// Missing required 'name' field
	body := map[string]interface{}{
		"includedNamespaces": []string{"default"},
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(baseURL+"/backups", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestE2E_CreateRestoreInvalidInput(t *testing.T) {
	requireBackend(t)

	// Missing required 'backupName' field
	body := map[string]interface{}{
		"name": "invalid-restore",
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(baseURL+"/restores", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// --- E2E: Cross-namespace restore ---

func TestE2E_CrossNamespaceRestore(t *testing.T) {
	requireBackend(t)

	restoreName := fmt.Sprintf("e2e-cross-ns-%d", time.Now().Unix())

	body := map[string]interface{}{
		"name":             restoreName,
		"backupName":       "test-backup",
		"namespaceMapping": map[string]string{"default": "restored-ns"},
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(baseURL+"/restores", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Verify namespace mapping is stored
	getResp, err := http.Get(baseURL + "/restores/" + restoreName)
	if err != nil {
		t.Fatalf("failed to get restore: %v", err)
	}
	defer getResp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&result)
	spec := result["spec"].(map[string]interface{})
	nsMapping := spec["namespaceMapping"].(map[string]interface{})
	if nsMapping["default"] != "restored-ns" {
		t.Errorf("expected namespace mapping default->restored-ns, got %v", nsMapping)
	}
}
