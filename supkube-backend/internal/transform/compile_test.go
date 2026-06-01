// Package transform: compile_test.go — unit tests for TransformSet compilation.
//
// PRD-002 v1.3 §4.3 (T1 编译契约抓手). Uses controller-runtime fake client
// so tests don't need an envtest binary — fast, hermetic.
//
// Cases (5 minimum per PRD-002 acceptance):
//  1. basic    — 1 TransformSet referencing 1 Transform (no params)
//  2. params   — Transform with ${FROM}/${TO}, substituted at compile time
//  3. missing  — Transform with ${MISSING}, no param provided → error
//  4. order    — TransformSet with 2 Transforms, rules.yaml preserves order
//  5. reuse    — second Compile with identical input reuses existing CM
package transform

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newFakeClient builds a fresh fake client with core/v1 registered.
func newFakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// makeTransform builds a Transform CM in supkube ns with the given rules.yaml body.
func makeTransform(name, rulesYAML string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: SupKubeNamespace,
			Labels:    map[string]string{LabelKind: KindTransform},
		},
		Data: map[string]string{TransformRulesKey: rulesYAML},
	}
}

// makeTransformSet builds a TransformSet CM in supkube ns referencing the given Transforms.
func makeTransformSet(name string, refs []string, defaults map[string]string) *corev1.ConfigMap {
	var sb strings.Builder
	sb.WriteString("transformRefs:\n")
	for _, r := range refs {
		sb.WriteString("  - ")
		sb.WriteString(r)
		sb.WriteString("\n")
	}
	if len(defaults) > 0 {
		sb.WriteString("defaults:\n")
		for k, v := range defaults {
			sb.WriteString("  ")
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(v)
			sb.WriteString("\n")
		}
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: SupKubeNamespace,
			Labels:    map[string]string{LabelKind: KindTransformSet},
		},
		Data: map[string]string{TransformSetSpecKey: sb.String()},
	}
}

// helper YAML body for a single-rule Transform.
const sampleRulesYAML = `version: v1
resourceModifierRules:
  - conditions:
      groupResource: services
      resourceNameRegex: ".*"
    mergePatches:
      - patchData: '{"spec":{"clusterIP":null}}'
`

// rulesWithVars exercises ${FROM}/${TO} substitution in two fields.
const rulesWithVarsYAML = `version: v1
resourceModifierRules:
  - conditions:
      groupResource: services
      resourceNameRegex: "^${FROM}$"
    mergePatches:
      - patchData: '{"metadata":{"namespace":"${TO}"}}'
`

const rulesWithMissingVarYAML = `version: v1
resourceModifierRules:
  - conditions:
      groupResource: services
      resourceNameRegex: ".*"
    mergePatches:
      - patchData: '{"spec":{"clusterIP":"${MISSING}"}}'
`

const rulesAlternate = `version: v1
resourceModifierRules:
  - conditions:
      groupResource: persistentvolumeclaims
      resourceNameRegex: ".*"
    mergePatches:
      - patchData: '{"spec":{"volumeName":null}}'
`

func TestCompile_Basic_NoParams(t *testing.T) {
	tr := makeTransform("strip-clusterip", sampleRulesYAML)
	ts := makeTransformSet("ts-basic", []string{"strip-clusterip"}, nil)
	cl := newFakeClient(tr, ts)

	res, err := Compile(context.Background(), cl, "ts-basic", nil)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(res.RulesYAML, "clusterIP") {
		t.Errorf("rules.yaml missing clusterIP patch: %s", res.RulesYAML)
	}
	if len(res.AppliedRefs) != 1 || res.AppliedRefs[0] != "strip-clusterip" {
		t.Errorf("AppliedRefs = %v, want [strip-clusterip]", res.AppliedRefs)
	}
	if !strings.HasPrefix(res.ConfigMap.Name, DerivedCMNamePrefix+"ts-basic-") {
		t.Errorf("CM name %q does not match expected pattern", res.ConfigMap.Name)
	}
	if res.ConfigMap.Namespace != VeleroNamespace {
		t.Errorf("CM ns %q want %q", res.ConfigMap.Namespace, VeleroNamespace)
	}
	// Verify CM was actually created in cluster.
	got := &corev1.ConfigMap{}
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: VeleroNamespace, Name: res.ConfigMap.Name}, got); err != nil {
		t.Errorf("derived CM not found in cluster: %v", err)
	}
	// Single data key invariant (ADR-003).
	if len(got.Data) != 1 {
		t.Errorf("derived CM has %d data keys, want exactly 1 (Velero rejects multi-key)", len(got.Data))
	}
	if _, ok := got.Data[TransformRulesKey]; !ok {
		t.Errorf("derived CM missing data[%s]", TransformRulesKey)
	}
}

func TestCompile_VarSubstitution(t *testing.T) {
	tr := makeTransform("ns-remap", rulesWithVarsYAML)
	ts := makeTransformSet("ts-remap", []string{"ns-remap"}, nil)
	cl := newFakeClient(tr, ts)

	res, err := Compile(context.Background(), cl, "ts-remap", map[string]string{
		"FROM": "app-old",
		"TO":   "app-new",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(res.RulesYAML, "app-old") {
		t.Errorf("rules.yaml missing FROM substitution (app-old): %s", res.RulesYAML)
	}
	if !strings.Contains(res.RulesYAML, "app-new") {
		t.Errorf("rules.yaml missing TO substitution (app-new): %s", res.RulesYAML)
	}
	if strings.Contains(res.RulesYAML, "${FROM}") || strings.Contains(res.RulesYAML, "${TO}") {
		t.Errorf("rules.yaml still contains unsubstituted placeholder: %s", res.RulesYAML)
	}
}

func TestCompile_MissingParamErrors(t *testing.T) {
	tr := makeTransform("needs-missing", rulesWithMissingVarYAML)
	ts := makeTransformSet("ts-needs-missing", []string{"needs-missing"}, nil)
	cl := newFakeClient(tr, ts)

	_, err := Compile(context.Background(), cl, "ts-needs-missing", nil)
	if err == nil {
		t.Fatalf("Compile should have errored for missing param ${MISSING}")
	}
	if !strings.Contains(err.Error(), "MISSING") {
		t.Errorf("err message should mention the missing var name; got: %v", err)
	}
}

func TestCompile_RefOrderPreserved(t *testing.T) {
	trA := makeTransform("first-clusterip", sampleRulesYAML)
	trB := makeTransform("then-pvc", rulesAlternate)
	// Order: clusterip first, pvc second.
	ts := makeTransformSet("ts-order", []string{"first-clusterip", "then-pvc"}, nil)
	cl := newFakeClient(trA, trB, ts)

	res, err := Compile(context.Background(), cl, "ts-order", nil)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(res.AppliedRefs) != 2 ||
		res.AppliedRefs[0] != "first-clusterip" ||
		res.AppliedRefs[1] != "then-pvc" {
		t.Errorf("AppliedRefs order wrong: %v", res.AppliedRefs)
	}
	// Body order: services patch must appear before persistentvolumeclaims.
	idxSvc := strings.Index(res.RulesYAML, "services")
	idxPVC := strings.Index(res.RulesYAML, "persistentvolumeclaims")
	if idxSvc < 0 || idxPVC < 0 {
		t.Fatalf("expected both groupResources in rules.yaml; got: %s", res.RulesYAML)
	}
	if idxSvc > idxPVC {
		t.Errorf("services rule should appear BEFORE persistentvolumeclaims rule; got: %s", res.RulesYAML)
	}
}

func TestCompile_ReusesExistingCM(t *testing.T) {
	tr := makeTransform("strip-clusterip", sampleRulesYAML)
	ts := makeTransformSet("ts-reuse", []string{"strip-clusterip"}, nil)
	cl := newFakeClient(tr, ts)

	res1, err := Compile(context.Background(), cl, "ts-reuse", map[string]string{"X": "y"})
	if err != nil {
		t.Fatalf("first Compile: %v", err)
	}
	rv1 := res1.ConfigMap.ResourceVersion

	res2, err := Compile(context.Background(), cl, "ts-reuse", map[string]string{"X": "y"})
	if err != nil {
		t.Fatalf("second Compile: %v", err)
	}
	if res1.ConfigMap.Name != res2.ConfigMap.Name {
		t.Errorf("second call produced different CM name: %s vs %s", res1.ConfigMap.Name, res2.ConfigMap.Name)
	}
	if res1.Hash != res2.Hash {
		t.Errorf("hash differs across identical inputs: %s vs %s", res1.Hash, res2.Hash)
	}
	// Reused CM should have the SAME ResourceVersion (no Update happened).
	if rv1 != res2.ConfigMap.ResourceVersion {
		t.Errorf("second Compile mutated the CM (rv %s → %s); should have reused unchanged", rv1, res2.ConfigMap.ResourceVersion)
	}
}

// TestApplyParams_UnsafeValue rejects shell-metacharacter values per PRD-002 §4 safety.
func TestApplyParams_UnsafeValue(t *testing.T) {
	cases := map[string]string{
		"semicolon": "evil;rm -rf",
		"pipe":      "a|b",
		"backtick":  "`whoami`",
		"dollar":    "$(echo)",
		"yaml-doc":  "---",
		"newline":   "a\nb",
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := applyParams("foo: ${X}", map[string]string{"X": bad})
			if err == nil {
				t.Errorf("applyParams accepted unsafe value %q", bad)
			}
		})
	}
}

// TestHashInput_Deterministic confirms map iteration order doesn't change the hash.
func TestHashInput_Deterministic(t *testing.T) {
	refs := []string{"a", "b"}
	p1 := map[string]string{"X": "1", "Y": "2", "Z": "3"}
	p2 := map[string]string{"Z": "3", "Y": "2", "X": "1"} // same content, different declaration order
	h1 := hashInput("ts", refs, p1)
	h2 := hashInput("ts", refs, p2)
	if h1 != h2 {
		t.Errorf("hash not deterministic across param map orderings: %s vs %s", h1, h2)
	}
	// Different content → different hash.
	h3 := hashInput("ts", refs, map[string]string{"X": "1", "Y": "2", "Z": "4"})
	if h1 == h3 {
		t.Errorf("hash collision on different content: %s == %s", h1, h3)
	}
}

// TestCompile_TransformSetMissing reports a clean error (not panic).
func TestCompile_TransformSetMissing(t *testing.T) {
	cl := newFakeClient()
	_, err := Compile(context.Background(), cl, "nonexistent", nil)
	if err == nil {
		t.Fatalf("Compile should error when TransformSet doesn't exist")
	}
}

// TestCompile_TransformRefMissing surfaces broken ref clearly.
func TestCompile_TransformRefMissing(t *testing.T) {
	ts := makeTransformSet("ts-broken", []string{"does-not-exist"}, nil)
	cl := newFakeClient(ts)
	_, err := Compile(context.Background(), cl, "ts-broken", nil)
	if err == nil {
		t.Fatalf("Compile should error when transformRef target doesn't exist")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("err should name the missing Transform; got: %v", err)
	}
}

// TestCompile_LabelMismatch refuses a CM that isn't labeled as a TransformSet.
func TestCompile_LabelMismatch(t *testing.T) {
	notATS := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "imposter",
			Namespace: SupKubeNamespace,
			Labels:    map[string]string{LabelKind: "something-else"},
		},
		Data: map[string]string{TransformSetSpecKey: "transformRefs: [a]"},
	}
	cl := newFakeClient(notATS)
	_, err := Compile(context.Background(), cl, "imposter", nil)
	if err == nil {
		t.Fatalf("Compile should reject CM lacking kind=transform-set label")
	}
}

// TestCompileDryRun_DoesNotCreate verifies the preview-resolution path doesn't
// pollute the cluster.
func TestCompileDryRun_DoesNotCreate(t *testing.T) {
	tr := makeTransform("strip-clusterip", sampleRulesYAML)
	ts := makeTransformSet("ts-dry", []string{"strip-clusterip"}, nil)
	cl := newFakeClient(tr, ts)

	res, err := CompileDryRun(context.Background(), cl, "ts-dry", nil)
	if err != nil {
		t.Fatalf("CompileDryRun: %v", err)
	}
	// Probe: the derived CM should NOT exist in the fake cluster.
	got := &corev1.ConfigMap{}
	err = cl.Get(context.Background(), types.NamespacedName{Namespace: VeleroNamespace, Name: res.ConfigMap.Name}, got)
	if err == nil {
		t.Errorf("CompileDryRun should not have created derived CM, but %s exists", res.ConfigMap.Name)
	}
}
