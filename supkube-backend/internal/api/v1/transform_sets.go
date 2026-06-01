// Package v1: transform_sets.go
//
// TransformSet CRUD (PRD-002 v1.3 two-layer schema)
// ─────────────────────────────────────────────────
// A TransformSet is now a thin CONTAINER referencing one or more atomic
// Transforms (see transforms.go) plus optional defaults for ${VAR}
// placeholders. The actual Velero ResourceModifier rules live in the
// referenced Transforms — TransformSet is composition + parameterization.
//
// Why the split (vs the v0.8.2 single-layer design):
//   - Atomic Transforms are reusable across many TransformSets (e.g.
//     `strip-clusterip` plugged into multiple cross-cluster restore packs).
//   - TransformSets stay declarative & lightweight — no Velero schema
//     leakage in the container layer.
//   - Compilation (internal/transform/compile.go) flattens a TransformSet
//     into a single Velero-compatible ConfigMap at Restore time, with
//     a content-addressed hash so identical inputs reuse the same CM.
//
// Storage model:
//   - ns:    "supkube" (not "velero" — derived CMs live in velero ns)
//   - label: supkube.io/kind=transform-set
//   - data:  exactly one key, "transform-set.yaml", holding the
//     TransformSetSpec (transformRefs[], defaults{}, description).
//
// Single-key invariant: kept consistent with Transform + the derived
// ConfigMap (ADR-003), even though Velero never reads TransformSet
// directly (only the compile.go-produced CM). Predictability > local
// optimization.
//
// Migration from v0.8.x single-layer layout (velero ns, kind=transform,
// data.rules.yaml directly) → split into:
//   - 1 atomic Transform `<orig>-rules` in supkube ns (holds rules.yaml)
//   - 1 wrapper TransformSet `<orig>` in supkube ns (transformRefs=[<orig>-rules])
//
// See migrateBrokenTransformSets in transform_sets_seed.go.
package v1

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	sigyaml "sigs.k8s.io/yaml"

	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/transform"
)

// ─── Constants (re-exported aliases of internal/transform package) ──────
// Single source of truth lives in internal/transform/compile.go so the
// compiler & the CRUD share label values, ns, and data-key names without
// risk of drift. We re-alias here only because the local file already
// referenced the old constant names; keep them as locals for clarity.
const (
	transformSetNamespace = transform.SupKubeNamespace
	transformSetLabelKind = transform.LabelKind
	// transformSetKindValue is the value of supkube.io/kind for the
	// CONTAINER layer ("transform-set"). The atomic layer uses the
	// distinct value "transform" (see transforms.go).
	transformSetKindValue = transform.KindTransformSet
	// transformSetSpecKey is the SINGLE permitted data key on a
	// TransformSet ConfigMap. ADR-003: keep len(Data)==1 for every CM
	// we manage so the invariant holds uniformly.
	transformSetSpecKey = transform.TransformSetSpecKey
	// transformRulesKey is the single key on a derived/atomic CM
	// (used by the compile.go consumer + the migration codepath).
	transformRulesKey = transform.TransformRulesKey

	// Legacy values present in clusters upgraded from v0.8.x.
	// migrateBrokenTransformSets rewrites these on startup.
	transformSetLegacyNamespace = "velero"
	transformSetLegacyKindValue = "transform" // single-layer label value
)

// ─── Annotations ────────────────────────────────────────────────────────
const (
	tsDescriptionAnnotation = "supkube.io/description"
	// Migration tracking annotations (ADR-014 startup migration).
	// The legacy CM in velero ns gets annotated (not deleted) so admins
	// can still see the original; subsequent migration runs detect this
	// and skip.
	annotMigratedTo   = "supkube.io/migrated-to"
	annotMigratedFrom = "supkube.io/migrated-from"
	annotMigrationErr = "supkube.io/migration-error"
)

// ─── User-facing types ─────────────────────────────────────────────────

// TransformSetRef is one entry in TransformSet.transformRefs[].
// Params are optional per-ref overrides; the TransformSet.defaults map
// is a flat fallback applied to every ref.
//
// NOTE (PRD-002 v1.3): the on-disk spec format
// (data["transform-set.yaml"]) keeps transformRefs as a STRING ARRAY
// (`["t1", "t2"]`) per compile.go's TransformSetSpec, which is what
// the compiler reads. The API layer accepts the richer object form
// `[{name, params}]` for forward-compat but currently only the `name`
// field round-trips through the compiler (per-ref params live in the
// TransformSet.defaults map for now — see compile.go merging logic).
type TransformSetRef struct {
	Name   string            `json:"name"`
	Params map[string]string `json:"params,omitempty"`
}

// TransformSet is the SupKube-facing shape returned by list/get and
// accepted by create/update. It maps 1:1 to a ConfigMap in supkube ns
// with one data key.
type TransformSet struct {
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	TransformRefs []TransformSetRef `json:"transformRefs"`
	Defaults      map[string]string `json:"defaults,omitempty"`
	BuiltIn       bool              `json:"builtIn,omitempty"`
	CreatedAt     string            `json:"createdAt,omitempty"`
}

// ─── Client factory (test seam) ─────────────────────────────────────────
//
// Tests override transformClientFactory to inject a fake Clientset; in
// production it falls through to k8s.GetClient(). Keeping the seam local
// (not a package-level helper everyone shares) limits blast radius.
var transformClientFactory = func() (kubernetes.Interface, error) {
	return k8s.GetClient()
}

// ─── HTTP handlers ──────────────────────────────────────────────────────

// ListTransformSets returns every TransformSet CM in supkube ns.
// Containers only — atomic Transforms are listed via ListTransforms.
func ListTransformSets(c *gin.Context) {
	cl, err := transformClientFactory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	selector := fmt.Sprintf("%s=%s", transformSetLabelKind, transformSetKindValue)
	cms, err := cl.CoreV1().ConfigMaps(transformSetNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]TransformSet, 0, len(cms.Items))
	for i := range cms.Items {
		ts, err := configMapToTransformSet(&cms.Items[i])
		if err != nil {
			// Skip malformed entries silently — one bad CM shouldn't
			// kill the whole list.
			continue
		}
		out = append(out, ts)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "total": len(out)})
}

// GetTransformSet returns one TransformSet by name.
func GetTransformSet(c *gin.Context) {
	name := c.Param("name")
	cl, err := transformClientFactory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cm, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if !isTransformSetConfigMap(cm) {
		c.JSON(http.StatusNotFound, gin.H{"error": "ConfigMap exists but is not a SupKube TransformSet"})
		return
	}
	ts, err := configMapToTransformSet(cm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode TransformSet: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, ts)
}

// CreateTransformSet persists a new container.
func CreateTransformSet(c *gin.Context) {
	var ts TransformSet
	if err := c.ShouldBindJSON(&ts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateK8sName(ts.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateTransformSet(&ts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cm, err := transformSetToConfigMap(&ts, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl, err := transformClientFactory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Create(context.Background(), cm, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("TransformSet %q already exists", ts.Name)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ts)
}

// UpdateTransformSet refuses to mutate built-ins (clone-to-edit pattern).
func UpdateTransformSet(c *gin.Context) {
	name := c.Param("name")
	var ts TransformSet
	if err := c.ShouldBindJSON(&ts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if ts.Name == "" {
		ts.Name = name
	} else if ts.Name != name {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL name and body name must match"})
		return
	}
	if err := validateTransformSet(&ts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cl, err := transformClientFactory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	existing, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if !isTransformSetConfigMap(existing) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not a SupKube TransformSet"})
		return
	}
	if existing.Labels["supkube.io/builtin"] == "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "built-in TransformSets cannot be modified; clone it under a new name instead"})
		return
	}

	updated, err := transformSetToConfigMap(&ts, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated.ResourceVersion = existing.ResourceVersion // optimistic concurrency
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Update(context.Background(), updated, metav1.UpdateOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ts)
}

// DeleteTransformSet drops the container.
//
// IMPORTANT: deleting a TransformSet does NOT cascade-delete the atomic
// Transforms it references — they may be shared with other TransformSets.
// The caller (UI) is expected to delete unreferenced Transforms
// separately via DeleteTransform.
func DeleteTransformSet(c *gin.Context) {
	name := c.Param("name")
	cl, err := transformClientFactory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cm, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if !isTransformSetConfigMap(cm) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not a SupKube TransformSet"})
		return
	}
	if cm.Labels["supkube.io/builtin"] == "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "built-in TransformSets cannot be deleted"})
		return
	}
	if err := cl.CoreV1().ConfigMaps(transformSetNamespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "TransformSet deleted"})
}

// ─── ApplyConflictFixes (PRD-001 v2 跳转端点) ───────────────────────────
//
// In the v0.8.x single-layer model this endpoint materialized a
// consolidated Velero ResourceModifier ConfigMap directly. In PRD-002
// v1.3 the responsibility moves DOWN the stack: we now produce an
// atomic Transform (rules.yaml) representing the user's accepted fix
// set, and let the user wire it into a TransformSet (or auto-bind it
// to the Restore drawer's adhoc TransformSet) as a separate step.
//
// The returned `transformName` is what the UI navigates to (Transforms
// page filter or Edit drawer) for the user to review before adding to
// a TransformSet.
//
// Body: { restoreName, fixes: [ConflictFix] }
// Returns: { transformName, ruleCount, created }
func ApplyConflictFixes(c *gin.Context) {
	var req struct {
		RestoreName string        `json:"restoreName" binding:"required"`
		Fixes       []ConflictFix `json:"fixes" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Fixes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one fix is required"})
		return
	}

	// Build one rule per fix.
	var rules []TSRule
	var descriptionParts []string
	for _, fix := range req.Fixes {
		if fix.SuggestedTransform == nil {
			continue
		}
		gr := lowerKindToResource(fix.Artifact.Kind)
		if fix.Artifact.Group != "" {
			gr = gr + "." + fix.Artifact.Group
		}
		conditions := TSCondition{
			GroupResource:     gr,
			ResourceNameRegex: fmt.Sprintf("^%s$", regexpEscape(fix.Artifact.Name)),
			Namespaces:        []string{fix.Artifact.Namespace},
		}
		rule := TSRule{Conditions: conditions}
		st := fix.SuggestedTransform
		switch {
		case st.MergePatch != "":
			rule.MergePatches = []TSMergePatch{{PatchData: st.MergePatch}}
		case st.StrategicPatch != "":
			rule.StrategicPatches = []TSMergePatch{{PatchData: st.StrategicPatch}}
		case st.Operation != "" && st.Path != "":
			rule.Patches = []TSPatch{{
				Operation: st.Operation,
				Path:      st.Path,
				Value:     st.Value,
			}}
		default:
			continue
		}
		rules = append(rules, rule)
		descriptionParts = append(descriptionParts,
			fmt.Sprintf("%s on %s/%s", fix.ConflictKind, fix.Artifact.Namespace, fix.Artifact.Name))
	}
	if err := validateRulesList(rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "generated rules failed validation: " + err.Error()})
		return
	}

	// Stable atomic Transform name per restore session. Truncate if needed.
	prefix := "preflight-fix-"
	if len(req.RestoreName) > 253-len(prefix) {
		req.RestoreName = req.RestoreName[:253-len(prefix)]
	}
	trName := prefix + req.RestoreName

	tr := Transform{
		Name:        trName,
		Description: fmt.Sprintf("Auto-generated by Pre-flight (%d fix(es)): %s", len(rules), strings.Join(descriptionParts, "; ")),
		Rules:       rules,
	}

	cl, err := transformClientFactory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cm, err := transformToConfigMap(&tr, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cm.Labels["supkube.io/auto-generated"] = "true"

	// Upsert.
	created := true
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Create(context.Background(), cm, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		existing, gerr := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), trName, metav1.GetOptions{})
		if gerr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gerr.Error()})
			return
		}
		cm.ResourceVersion = existing.ResourceVersion
		if _, uerr := cl.CoreV1().ConfigMaps(transformSetNamespace).Update(context.Background(), cm, metav1.UpdateOptions{}); uerr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": uerr.Error()})
			return
		}
		created = false
	}
	c.JSON(http.StatusOK, gin.H{
		"transformName": trName,
		"created":       created,
		"ruleCount":     len(rules),
	})
}

// ConflictFix is one entry in the batch the UI sends to ApplyConflictFixes.
type ConflictFix struct {
	ConflictKind       string         `json:"conflictKind"`
	Artifact           ArtifactRef    `json:"artifact"`
	SuggestedTransform *TransformRule `json:"suggestedTransform"`
}

// ─── Helpers ────────────────────────────────────────────────────────────

func isTransformSetConfigMap(cm *corev1.ConfigMap) bool {
	if cm == nil || cm.Labels == nil {
		return false
	}
	return cm.Labels[transformSetLabelKind] == transformSetKindValue
}

// configMapToTransformSet decodes the spec key into the API shape.
func configMapToTransformSet(cm *corev1.ConfigMap) (TransformSet, error) {
	ts := TransformSet{
		Name:      cm.Name,
		BuiltIn:   cm.Labels["supkube.io/builtin"] == "true",
		CreatedAt: cm.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}
	if cm.Annotations != nil {
		ts.Description = cm.Annotations[tsDescriptionAnnotation]
	}
	raw, ok := cm.Data[transformSetSpecKey]
	if !ok || strings.TrimSpace(raw) == "" {
		return ts, fmt.Errorf("TransformSet %q missing data[%s]", cm.Name, transformSetSpecKey)
	}
	// Use transform.TransformSetSpec for parsing; the compiler is the
	// canonical reader of this file. We expand the parsed []string refs
	// into the API's []TransformSetRef shape (params=nil per-ref;
	// per-ref params not yet plumbed through compile.go).
	var spec transform.TransformSetSpec
	if err := sigyaml.Unmarshal([]byte(raw), &spec); err != nil {
		return ts, fmt.Errorf("parse TransformSet spec: %w", err)
	}
	if spec.Description != "" && ts.Description == "" {
		ts.Description = spec.Description
	}
	for _, name := range spec.TransformRefs {
		ts.TransformRefs = append(ts.TransformRefs, TransformSetRef{Name: name})
	}
	ts.Defaults = spec.Defaults
	return ts, nil
}

// transformSetToConfigMap serializes the API shape back into a CM.
//
// transformRefs are flattened to []string when written so the on-disk
// format matches what compile.go expects (TransformSetSpec.TransformRefs
// is `[]string`). Per-ref params from the API input are dropped at
// write time — a tracking warning is left in the description for now.
func transformSetToConfigMap(ts *TransformSet, builtIn bool) (*corev1.ConfigMap, error) {
	if ts.TransformRefs == nil {
		ts.TransformRefs = []TransformSetRef{}
	}
	refs := make([]string, 0, len(ts.TransformRefs))
	perRefParamsPresent := false
	for _, r := range ts.TransformRefs {
		refs = append(refs, r.Name)
		if len(r.Params) > 0 {
			perRefParamsPresent = true
		}
	}
	desc := ts.Description
	if perRefParamsPresent && !strings.Contains(desc, "per-ref params") {
		// Self-documenting drift signal: tells the next reader that
		// per-ref params were sent but the compiler currently only
		// honours TransformSet-level defaults.
		desc = strings.TrimSpace(desc + " [warning: per-ref params not yet honoured by compile.go — supply via Defaults]")
	}
	spec := transform.TransformSetSpec{
		TransformRefs: refs,
		Defaults:      ts.Defaults,
		Description:   desc,
	}
	yamlBytes, err := sigyaml.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("serialize transform-set.yaml: %w", err)
	}
	labels := map[string]string{
		transformSetLabelKind:   transformSetKindValue,
		"supkube.io/managed-by": "supkube",
	}
	if builtIn {
		labels["supkube.io/builtin"] = "true"
	}
	annotations := map[string]string{
		tsDescriptionAnnotation: desc,
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ts.Name,
			Namespace:   transformSetNamespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: map[string]string{
			transformSetSpecKey: string(yamlBytes),
		},
	}
	return cm, nil
}

// validateTransformSet sanity-checks the container before persisting.
//   - transformRefs[] non-empty
//   - each ref name validates as a k8s name (so Get can find them)
//   - duplicate ref names rejected (caller almost certainly mis-clicked)
//
// We deliberately do NOT verify each ref exists in supkube ns here:
//  1. CRUD is async-friendly — a TransformSet may be created before its
//     Transforms (e.g. installer Helm chart order isn't guaranteed).
//  2. Compile-time validation already errors loudly at Restore.
func validateTransformSet(ts *TransformSet) error {
	if len(ts.TransformRefs) == 0 {
		return fmt.Errorf("transformRefs must be non-empty")
	}
	seen := map[string]struct{}{}
	for i, r := range ts.TransformRefs {
		if err := validateK8sName(r.Name); err != nil {
			return fmt.Errorf("transformRefs[%d]: %w", i, err)
		}
		if _, dup := seen[r.Name]; dup {
			return fmt.Errorf("transformRefs[%d]: duplicate ref %q", i, r.Name)
		}
		seen[r.Name] = struct{}{}
	}
	return nil
}

// ─── Helpers shared with transforms.go + ApplyConflictFixes ─────────────
// (kept here because transforms.go imports them via package-scope.)

// lowerKindToResource maps PascalCase Kind → lowercase plural resource.
func lowerKindToResource(kind string) string {
	switch kind {
	case "Service":
		return "services"
	case "Ingress":
		return "ingresses"
	case "PersistentVolumeClaim":
		return "persistentvolumeclaims"
	case "Pod":
		return "pods"
	case "Deployment":
		return "deployments"
	case "StatefulSet":
		return "statefulsets"
	case "DaemonSet":
		return "daemonsets"
	case "Job":
		return "jobs"
	case "CronJob":
		return "cronjobs"
	default:
		return strings.ToLower(kind) + "s"
	}
}

// regexpEscape escapes ^ . $ ( ) [ ] { } | \ + * ? for a literal-match regex.
func regexpEscape(s string) string {
	const specials = `^.$()[]{}|\+*?`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(specials, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// (shortHash + mustJSON helpers from the v0.8.x single-layer schema were
// dropped during the PRD-002 v1.3 refactor — content-addressed hashing
// now lives in transform.hashInput, and JSON serialization uses
// encoding/json directly in the few callers that need it.)
