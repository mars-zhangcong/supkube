// Package v1: transform_sets_test.go
//
// Unit tests for the PRD-002 v1.3 two-layer schema CRUD + the startup
// migration that splits legacy single-layer ConfigMaps.
//
// Uses k8s.io/client-go/kubernetes/fake injected through the
// transformClientFactory test seam. No envtest required — hermetic,
// sub-millisecond tests.
package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// ─── Fixtures + helpers ─────────────────────────────────────────────────

// installFakeClient swaps transformClientFactory for the duration of a
// test and returns a restore func. Caller does `defer restore()`.
func installFakeClient(t *testing.T, seed ...*corev1.ConfigMap) (kubernetes.Interface, func()) {
	t.Helper()
	objs := make([]runtime.Object, len(seed))
	for i, c := range seed {
		objs[i] = c
	}
	cl := k8sfake.NewSimpleClientset(objs...)
	prev := transformClientFactory
	transformClientFactory = func() (kubernetes.Interface, error) {
		return cl, nil
	}
	return cl, func() { transformClientFactory = prev }
}

// makeAtomicTransformCM constructs a Transform CM (kind=transform, ns=supkube).
func makeAtomicTransformCM(name, rulesYAML string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: transformSetNamespace,
			Labels:    map[string]string{transformSetLabelKind: transformKindValue},
		},
		Data: map[string]string{transformRulesKey: rulesYAML},
	}
}

// makeWrapperTransformSetCM constructs a TransformSet CM (kind=transform-set, ns=supkube).
func makeWrapperTransformSetCM(name string, refs []string) *corev1.ConfigMap {
	var sb strings.Builder
	sb.WriteString("transformRefs:\n")
	for _, r := range refs {
		sb.WriteString("  - ")
		sb.WriteString(r)
		sb.WriteString("\n")
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: transformSetNamespace,
			Labels:    map[string]string{transformSetLabelKind: transformSetKindValue},
		},
		Data: map[string]string{transformSetSpecKey: sb.String()},
	}
}

// makeLegacyVeleroCM constructs the v0.8.x single-layer CM in velero ns.
func makeLegacyVeleroCM(name, rulesYAML, kindLabelValue string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: transformSetLegacyNamespace,
			Labels:    map[string]string{transformSetLabelKind: kindLabelValue},
			Annotations: map[string]string{
				tsDescriptionAnnotation: "legacy " + name,
			},
		},
		Data: map[string]string{transformRulesKey: rulesYAML},
	}
}

const minimalRulesYAML = `version: v1
resourceModifierRules:
  - conditions:
      groupResource: services
    mergePatches:
      - patchData: '{"spec":{"clusterIP":null}}'
`

// ─── HTTP-roundtrip helpers ────────────────────────────────────────────

func doReq(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, _ := http.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func buildRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.GET("/transform-sets", ListTransformSets)
		api.POST("/transform-sets", CreateTransformSet)
		api.GET("/transform-sets/:name", GetTransformSet)
		api.PUT("/transform-sets/:name", UpdateTransformSet)
		api.DELETE("/transform-sets/:name", DeleteTransformSet)
		api.GET("/transforms", ListTransforms)
		api.POST("/transforms", CreateTransform)
		api.GET("/transforms/:name", GetTransform)
		api.PUT("/transforms/:name", UpdateTransform)
		api.DELETE("/transforms/:name", DeleteTransform)
	}
	return r
}

// ─── Tests ──────────────────────────────────────────────────────────────

// TestList_TwoLayerSchema seeds 1 atomic Transform + 1 wrapper TransformSet
// and verifies each list endpoint returns only its own kind.
func TestList_TwoLayerSchema(t *testing.T) {
	tr := makeAtomicTransformCM("strip-clusterip", minimalRulesYAML)
	ts := makeWrapperTransformSetCM("pack-1", []string{"strip-clusterip"})
	_, restore := installFakeClient(t, tr, ts)
	defer restore()
	r := buildRouter()

	// /transforms returns only the atomic.
	w := doReq(t, r, "GET", "/api/v1/transforms", nil)
	if w.Code != 200 {
		t.Fatalf("GET /transforms: %d, body=%s", w.Code, w.Body.String())
	}
	var trResp struct {
		Items []Transform `json:"items"`
		Total int         `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &trResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if trResp.Total != 1 || trResp.Items[0].Name != "strip-clusterip" {
		t.Errorf("expected 1 Transform 'strip-clusterip', got %+v", trResp)
	}

	// /transform-sets returns only the wrapper.
	w = doReq(t, r, "GET", "/api/v1/transform-sets", nil)
	if w.Code != 200 {
		t.Fatalf("GET /transform-sets: %d, body=%s", w.Code, w.Body.String())
	}
	var tsResp struct {
		Items []TransformSet `json:"items"`
		Total int            `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tsResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tsResp.Total != 1 || tsResp.Items[0].Name != "pack-1" {
		t.Errorf("expected 1 TransformSet 'pack-1', got %+v", tsResp)
	}
	if len(tsResp.Items[0].TransformRefs) != 1 ||
		tsResp.Items[0].TransformRefs[0].Name != "strip-clusterip" {
		t.Errorf("TransformSet didn't preserve transformRefs: %+v", tsResp.Items[0])
	}
}

// TestCreate_Transform exercises POST /transforms.
func TestCreate_Transform(t *testing.T) {
	cl, restore := installFakeClient(t)
	defer restore()
	r := buildRouter()

	body := Transform{
		Name:        "drop-finalizers",
		Description: "remove .metadata.finalizers",
		Rules: []TSRule{
			{
				Conditions: TSCondition{GroupResource: "pods"},
				MergePatches: []TSMergePatch{
					{PatchData: `{"metadata":{"finalizers":null}}`},
				},
			},
		},
	}
	w := doReq(t, r, "POST", "/api/v1/transforms", body)
	if w.Code != 201 {
		t.Fatalf("POST /transforms: %d, body=%s", w.Code, w.Body.String())
	}
	// Verify CM landed in supkube ns with right label + data key.
	cm, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "drop-finalizers", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("post-create Get: %v", err)
	}
	if cm.Labels[transformSetLabelKind] != transformKindValue {
		t.Errorf("wrong kind label: %s", cm.Labels[transformSetLabelKind])
	}
	if _, ok := cm.Data[transformRulesKey]; !ok {
		t.Errorf("data[%s] missing", transformRulesKey)
	}
	if len(cm.Data) != 1 {
		t.Errorf("data must be single-key (ADR-003), got %d keys", len(cm.Data))
	}
}

// TestCreate_TransformSet exercises POST /transform-sets with a transformRefs[].
func TestCreate_TransformSet(t *testing.T) {
	cl, restore := installFakeClient(t)
	defer restore()
	r := buildRouter()

	body := TransformSet{
		Name:        "ns-relocate",
		Description: "rename and strip cluster-pinned fields",
		TransformRefs: []TransformSetRef{
			{Name: "strip-clusterip"},
			{Name: "strip-pv-binding"},
		},
		Defaults: map[string]string{"FROM": "app-old", "TO": "app-new"},
	}
	w := doReq(t, r, "POST", "/api/v1/transform-sets", body)
	if w.Code != 201 {
		t.Fatalf("POST /transform-sets: %d, body=%s", w.Code, w.Body.String())
	}
	cm, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "ns-relocate", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("post-create Get: %v", err)
	}
	if cm.Labels[transformSetLabelKind] != transformSetKindValue {
		t.Errorf("wrong kind label: %s", cm.Labels[transformSetLabelKind])
	}
	if _, ok := cm.Data[transformSetSpecKey]; !ok {
		t.Errorf("data[%s] missing", transformSetSpecKey)
	}
	if len(cm.Data) != 1 {
		t.Errorf("data must be single-key (ADR-003), got %d keys", len(cm.Data))
	}
	// Round-trip: GET should return both refs in order + defaults.
	w = doReq(t, r, "GET", "/api/v1/transform-sets/ns-relocate", nil)
	if w.Code != 200 {
		t.Fatalf("GET: %d, %s", w.Code, w.Body.String())
	}
	var got TransformSet
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.TransformRefs) != 2 ||
		got.TransformRefs[0].Name != "strip-clusterip" ||
		got.TransformRefs[1].Name != "strip-pv-binding" {
		t.Errorf("transformRefs order not preserved: %+v", got.TransformRefs)
	}
	if got.Defaults["FROM"] != "app-old" || got.Defaults["TO"] != "app-new" {
		t.Errorf("defaults not preserved: %+v", got.Defaults)
	}
}

// TestCreate_TransformSet_RejectsEmptyRefs ensures validation triggers.
func TestCreate_TransformSet_RejectsEmptyRefs(t *testing.T) {
	_, restore := installFakeClient(t)
	defer restore()
	r := buildRouter()

	body := TransformSet{
		Name:          "empty",
		TransformRefs: []TransformSetRef{},
	}
	w := doReq(t, r, "POST", "/api/v1/transform-sets", body)
	if w.Code != 400 {
		t.Errorf("expected 400 for empty transformRefs, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCreate_TransformSet_RejectsDuplicateRefs catches a common mis-click.
func TestCreate_TransformSet_RejectsDuplicateRefs(t *testing.T) {
	_, restore := installFakeClient(t)
	defer restore()
	r := buildRouter()

	body := TransformSet{
		Name: "dup",
		TransformRefs: []TransformSetRef{
			{Name: "strip-clusterip"},
			{Name: "strip-clusterip"},
		},
	}
	w := doReq(t, r, "POST", "/api/v1/transform-sets", body)
	if w.Code != 400 {
		t.Errorf("expected 400 for duplicate refs, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMigration_SplitLegacySingleLayer simulates a v0.8.x velero-ns
// single-layer CM and verifies the migration creates the supkube-ns
// pair + annotates the source.
func TestMigration_SplitLegacySingleLayer(t *testing.T) {
	src := makeLegacyVeleroCM("old-pack", minimalRulesYAML, "transform-set")
	cl, restore := installFakeClient(t, src)
	defer restore()

	migrateBrokenTransformSets()

	// Atomic Transform now exists at supkube/<name>-rules.
	atomic, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "old-pack-rules", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("atomic Transform not created: %v", err)
	}
	if atomic.Labels[transformSetLabelKind] != transformKindValue {
		t.Errorf("atomic has wrong kind label: %s", atomic.Labels[transformSetLabelKind])
	}
	if atomic.Data[transformRulesKey] == "" {
		t.Errorf("atomic missing rules.yaml")
	}
	if atomic.Annotations[annotMigratedFrom] == "" {
		t.Errorf("atomic missing migrated-from annotation")
	}

	// Wrapper TransformSet exists at supkube/<name>.
	wrap, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "old-pack", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("wrapper TransformSet not created: %v", err)
	}
	if wrap.Labels[transformSetLabelKind] != transformSetKindValue {
		t.Errorf("wrapper has wrong kind label: %s", wrap.Labels[transformSetLabelKind])
	}
	if !strings.Contains(wrap.Data[transformSetSpecKey], "old-pack-rules") {
		t.Errorf("wrapper transformRefs missing atomic name: %s", wrap.Data[transformSetSpecKey])
	}

	// Source annotated migrated-to (not deleted).
	srcFresh, err := cl.CoreV1().ConfigMaps(transformSetLegacyNamespace).Get(context.Background(), "old-pack", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("source CM should still exist: %v", err)
	}
	if srcFresh.Annotations[annotMigratedTo] == "" {
		t.Errorf("source missing migrated-to annotation: %+v", srcFresh.Annotations)
	}
}

// TestMigration_Idempotent re-runs the migration twice; the second run
// must be a no-op (idempotency guard via annotation).
func TestMigration_Idempotent(t *testing.T) {
	src := makeLegacyVeleroCM("idem-pack", minimalRulesYAML, "transform")
	cl, restore := installFakeClient(t, src)
	defer restore()

	migrateBrokenTransformSets()
	rv1, _ := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "idem-pack", metav1.GetOptions{})

	migrateBrokenTransformSets()
	rv2, _ := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "idem-pack", metav1.GetOptions{})

	if rv1.ResourceVersion != rv2.ResourceVersion {
		t.Errorf("second migration mutated supkube wrapper (rv %s → %s); should have been no-op", rv1.ResourceVersion, rv2.ResourceVersion)
	}
}

// TestMigration_PartialFailureAnnotates ensures a per-CM failure is
// recorded as supkube.io/migration-error and doesn't block other CMs.
//
// We trigger failure by giving the source CM no rules.yaml at all —
// splitLegacyToTwoLayer rejects with "source CM has no rules.yaml".
func TestMigration_PartialFailureAnnotates(t *testing.T) {
	good := makeLegacyVeleroCM("good-one", minimalRulesYAML, "transform")
	bad := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-one",
			Namespace: transformSetLegacyNamespace,
			Labels:    map[string]string{transformSetLabelKind: "transform"},
		},
		// no Data — split will fail.
	}
	cl, restore := installFakeClient(t, good, bad)
	defer restore()

	migrateBrokenTransformSets()

	// Good one migrated.
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "good-one", metav1.GetOptions{}); err != nil {
		t.Errorf("good CM should have migrated: %v", err)
	}
	// Bad one annotated with the error.
	badFresh, err := cl.CoreV1().ConfigMaps(transformSetLegacyNamespace).Get(context.Background(), "bad-one", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("bad CM should still exist (best-effort): %v", err)
	}
	if badFresh.Annotations[annotMigrationErr] == "" {
		t.Errorf("bad CM should have migration-error annotation: %+v", badFresh.Annotations)
	}
}

// TestDelete_TransformWithDanglingRef refuses with 409 when any
// TransformSet still references the Transform.
func TestDelete_TransformWithDanglingRef(t *testing.T) {
	tr := makeAtomicTransformCM("strip-clusterip", minimalRulesYAML)
	ts := makeWrapperTransformSetCM("pack-using-it", []string{"strip-clusterip"})
	cl, restore := installFakeClient(t, tr, ts)
	defer restore()
	r := buildRouter()

	w := doReq(t, r, "DELETE", "/api/v1/transforms/strip-clusterip", nil)
	if w.Code != 409 {
		t.Fatalf("expected 409 dangling-ref, got %d: %s", w.Code, w.Body.String())
	}
	// Transform must still exist.
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "strip-clusterip", metav1.GetOptions{}); err != nil {
		t.Errorf("Transform should not have been deleted: %v", err)
	}
}

// TestDelete_TransformSetWithDanglingRef ensures deleting a TransformSet
// does NOT cascade-delete the atomic Transform (it may be shared).
// Spec asks: "删 TransformSet 但 Transform 还被别的 TS 引用 → 只删 TS, 不动 Transform".
func TestDelete_TransformSetWithDanglingRef(t *testing.T) {
	tr := makeAtomicTransformCM("strip-clusterip", minimalRulesYAML)
	ts1 := makeWrapperTransformSetCM("pack-a", []string{"strip-clusterip"})
	ts2 := makeWrapperTransformSetCM("pack-b", []string{"strip-clusterip"})
	cl, restore := installFakeClient(t, tr, ts1, ts2)
	defer restore()
	r := buildRouter()

	w := doReq(t, r, "DELETE", "/api/v1/transform-sets/pack-a", nil)
	if w.Code != 200 {
		t.Fatalf("DELETE /transform-sets/pack-a: %d, %s", w.Code, w.Body.String())
	}
	// pack-a gone.
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "pack-a", metav1.GetOptions{}); err == nil {
		t.Errorf("pack-a should have been deleted")
	}
	// pack-b still there.
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "pack-b", metav1.GetOptions{}); err != nil {
		t.Errorf("pack-b should still exist: %v", err)
	}
	// Atomic Transform untouched.
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), "strip-clusterip", metav1.GetOptions{}); err != nil {
		t.Errorf("atomic Transform should still exist: %v", err)
	}
}

// TestApplyConflictFixes_CreatesAtomicTransform verifies the
// PRD-001 v2 jump-endpoint produces an atomic Transform (not a
// single-layer TransformSet like the old design).
func TestApplyConflictFixes_CreatesAtomicTransform(t *testing.T) {
	cl, restore := installFakeClient(t)
	defer restore()
	r := gin.New()
	r.POST("/apply", ApplyConflictFixes)

	body := map[string]any{
		"restoreName": "test-restore",
		"fixes": []map[string]any{
			{
				"conflictKind": "ClusterIPCollision",
				"artifact": map[string]any{
					"kind":      "Service",
					"namespace": "default",
					"name":      "my-svc",
				},
				"suggestedTransform": map[string]any{
					"mergePatch": `{"spec":{"clusterIP":null}}`,
				},
			},
		},
	}
	w := doReq(t, r, "POST", "/apply", body)
	if w.Code != 200 {
		t.Fatalf("POST /apply: %d, %s", w.Code, w.Body.String())
	}
	var resp struct {
		TransformName string `json:"transformName"`
		RuleCount     int    `json:"ruleCount"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.RuleCount != 1 {
		t.Errorf("expected 1 rule, got %d", resp.RuleCount)
	}
	cm, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), resp.TransformName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("generated Transform not found: %v", err)
	}
	if cm.Labels[transformSetLabelKind] != transformKindValue {
		t.Errorf("must be labelled kind=transform (atomic), got %s", cm.Labels[transformSetLabelKind])
	}
	if cm.Labels["supkube.io/auto-generated"] != "true" {
		t.Errorf("must carry auto-generated label for future GC")
	}
}

// TestUpdate_TransformSetBuiltinRefused mirrors UpdateTransform refusal.
func TestUpdate_TransformSetBuiltinRefused(t *testing.T) {
	ts := makeWrapperTransformSetCM("builtin-ts", []string{"strip-clusterip"})
	ts.Labels["supkube.io/builtin"] = "true"
	_, restore := installFakeClient(t, ts)
	defer restore()
	r := buildRouter()

	body := TransformSet{
		Name:          "builtin-ts",
		TransformRefs: []TransformSetRef{{Name: "something-else"}},
	}
	w := doReq(t, r, "PUT", "/api/v1/transform-sets/builtin-ts", body)
	if w.Code != 403 {
		t.Errorf("expected 403 for builtin update, got %d: %s", w.Code, w.Body.String())
	}
}
