// handlers_test.go — REST handler 单测.
//
// 覆盖:
//  1. POST create — 合法 spec → 201 + 默认值生效
//  2. POST create — bad cron → 400 ERR_IMPORTPOLICY_CRON_INVALID
//  3. POST create — interval < 30s → 400 ERR_IMPORTPOLICY_INTERVAL_TOO_SHORT
//  4. POST create — BSL 不存在 → 400 ERR_IMPORTPOLICY_BSL_NOTFOUND
//  5. POST create — 不合法 name → 400 ERR_IMPORTPOLICY_INVALID_NAME
//  6. GET list — empty + 多条
//  7. GET one + PUT update + DELETE
//  8. POST pause / resume — patch spec.paused
//  9. POST run-once — 触发 SyncOnce, 返回 importedCount + errors
package importpolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// setup 构造一个 router + 注入 fake dyn + fakeLister + fakeImporter,
// 同时把 controller 注册到 handler 单例. teardown 解注册.
func setup(t *testing.T, bslOK bool, initial ...*unstructured.Unstructured) (*gin.Engine, *Controller, *fakeImporter, *fakeLister) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()

	dyn := newDyn(t, initial...)
	lister := &fakeLister{}
	importer := &fakeImporter{}
	c := &Controller{DynCli: dyn, Lister: lister, Importer: importer, Validator: &fakeValidator{}}

	bslChecker := func(_ context.Context, _ string) (bool, error) { return bslOK, nil }
	RegisterController(c, dyn, bslChecker)
	t.Cleanup(func() {
		RegisterController(nil, nil, nil)
	})

	api := r.Group("/api/v1")
	api.GET("/import-policies", ListImportPolicies)
	api.POST("/import-policies", CreateImportPolicy)
	api.GET("/import-policies/:name", GetImportPolicy)
	api.PUT("/import-policies/:name", UpdateImportPolicy)
	api.DELETE("/import-policies/:name", DeleteImportPolicy)
	api.POST("/import-policies/:name/run-once", RunOnce)
	api.POST("/import-policies/:name/pause", PauseImportPolicy)
	api.POST("/import-policies/:name/resume", ResumeImportPolicy)
	return r, c, importer, lister
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeErr(t *testing.T, w *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode err response: %v (body=%s)", err, w.Body.String())
	}
	code, _ := resp["code"].(string)
	msg, _ := resp["error"].(string)
	return code, msg
}

func TestCreate_HappyPath(t *testing.T) {
	r, _, _, _ := setup(t, true)
	body := CreateRequest{
		Name: "p1",
		Spec: ImportPolicySpec{
			SourceBSL:          "default",
			Mode:               ImportModeContinuous,
			ContinuousInterval: "60s",
		},
	}
	w := doJSON(t, r, "POST", "/api/v1/import-policies", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var dto ImportPolicyDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode dto: %v", err)
	}
	// Defaults should be filled.
	if dto.Spec.FingerprintMode != FingerprintModeEnforce {
		t.Errorf("default fingerprintMode should be enforce, got %q", dto.Spec.FingerprintMode)
	}
	if dto.Spec.TargetVeleroNamespace != "velero" {
		t.Errorf("default targetVeleroNamespace should be velero, got %q", dto.Spec.TargetVeleroNamespace)
	}
}

func TestCreate_BadCron(t *testing.T) {
	r, _, _, _ := setup(t, true)
	body := CreateRequest{
		Name: "p1",
		Spec: ImportPolicySpec{
			SourceBSL: "default",
			Mode:      ImportModeScheduled,
			Schedule:  "nonsense",
		},
	}
	w := doJSON(t, r, "POST", "/api/v1/import-policies", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	code, _ := decodeErr(t, w)
	if code != ErrCronInvalid {
		t.Errorf("want code %s, got %s", ErrCronInvalid, code)
	}
}

func TestCreate_IntervalTooShort(t *testing.T) {
	r, _, _, _ := setup(t, true)
	body := CreateRequest{
		Name: "p1",
		Spec: ImportPolicySpec{
			SourceBSL:          "default",
			Mode:               ImportModeContinuous,
			ContinuousInterval: "5s",
		},
	}
	w := doJSON(t, r, "POST", "/api/v1/import-policies", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	code, _ := decodeErr(t, w)
	if code != ErrIntervalTooShort {
		t.Errorf("want %s, got %s", ErrIntervalTooShort, code)
	}
}

func TestCreate_BSLNotFound(t *testing.T) {
	r, _, _, _ := setup(t, false /* BSL check returns false */)
	body := CreateRequest{
		Name: "p1",
		Spec: ImportPolicySpec{
			SourceBSL: "missing-bsl",
			Mode:      ImportModeContinuous,
		},
	}
	w := doJSON(t, r, "POST", "/api/v1/import-policies", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	code, _ := decodeErr(t, w)
	if code != ErrBSLNotFound {
		t.Errorf("want %s, got %s", ErrBSLNotFound, code)
	}
}

func TestCreate_InvalidName(t *testing.T) {
	r, _, _, _ := setup(t, true)
	body := CreateRequest{
		Name: "InvalidName_With_Caps",
		Spec: ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous},
	}
	w := doJSON(t, r, "POST", "/api/v1/import-policies", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	code, _ := decodeErr(t, w)
	if code != ErrInvalidName {
		t.Errorf("want %s, got %s", ErrInvalidName, code)
	}
}

func TestList_GetUpdateDelete(t *testing.T) {
	spec := ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous, ContinuousInterval: "60s"}
	cr := policyCR("p1", spec, ImportPolicyStatus{})
	r, _, _, _ := setup(t, true, cr)

	// List
	w := doJSON(t, r, "GET", "/api/v1/import-policies", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list want 200, got %d", w.Code)
	}
	var listResp struct {
		Items []ImportPolicyDTO `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if len(listResp.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(listResp.Items))
	}

	// Get
	w = doJSON(t, r, "GET", "/api/v1/import-policies/p1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get want 200, got %d", w.Code)
	}

	// Update — change interval
	updBody := UpdateRequest{Spec: ImportPolicySpec{
		SourceBSL: "default", Mode: ImportModeContinuous, ContinuousInterval: "120s",
	}}
	w = doJSON(t, r, "PUT", "/api/v1/import-policies/p1", updBody)
	if w.Code != http.StatusOK {
		t.Fatalf("put want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var updDto ImportPolicyDTO
	_ = json.Unmarshal(w.Body.Bytes(), &updDto)
	if updDto.Spec.ContinuousInterval != "120s" {
		t.Errorf("update should persist interval=120s, got %q", updDto.Spec.ContinuousInterval)
	}

	// Delete
	w = doJSON(t, r, "DELETE", "/api/v1/import-policies/p1", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete want 204, got %d", w.Code)
	}
	w = doJSON(t, r, "GET", "/api/v1/import-policies/p1", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("after delete want 404, got %d", w.Code)
	}
}

func TestPauseResume(t *testing.T) {
	spec := ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous, ContinuousInterval: "60s"}
	cr := policyCR("p1", spec, ImportPolicyStatus{})
	r, _, _, _ := setup(t, true, cr)

	w := doJSON(t, r, "POST", "/api/v1/import-policies/p1/pause", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pause want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var dto ImportPolicyDTO
	_ = json.Unmarshal(w.Body.Bytes(), &dto)
	if !dto.Spec.Paused {
		t.Errorf("after pause, spec.paused should be true")
	}

	w = doJSON(t, r, "POST", "/api/v1/import-policies/p1/resume", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("resume want 200, got %d body=%s", w.Code, w.Body.String())
	}
	// Fresh DTO — json.Unmarshal into reused struct leaves prior fields set
	// when the new payload omits them (Paused has omitempty, so a false
	// value isn't even on the wire).
	dto = ImportPolicyDTO{}
	_ = json.Unmarshal(w.Body.Bytes(), &dto)
	if dto.Spec.Paused {
		t.Errorf("after resume, spec.paused should be false (body=%s)", w.Body.String())
	}
}

func TestRunOnce(t *testing.T) {
	spec := ImportPolicySpec{SourceBSL: "default", Mode: ImportModeContinuous, FingerprintMode: FingerprintModeEnforce}
	cr := policyCR("p1", spec, ImportPolicyStatus{})
	r, _, importer, lister := setup(t, true, cr)
	lister.add("default", "backup-x", "backup-y")

	w := doJSON(t, r, "POST", "/api/v1/import-policies/p1/run-once", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("run-once want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if got := int(resp["importedCount"].(float64)); got != 2 {
		t.Errorf("want importedCount=2, got %d", got)
	}
	if len(importer.all()) != 2 {
		t.Errorf("importer should have 2 records, got %d", len(importer.all()))
	}
}

func TestRunOnce_NotFound(t *testing.T) {
	r, _, _, _ := setup(t, true)
	w := doJSON(t, r, "POST", "/api/v1/import-policies/nope/run-once", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
	code, _ := decodeErr(t, w)
	if code != ErrNotFound {
		t.Errorf("want %s, got %s", ErrNotFound, code)
	}
}
