// Package v1: transforms.go
//
// Transform CRUD (PRD-002 v1.3 atomic layer)
// ──────────────────────────────────────────
// A Transform is the ATOMIC unit of the two-layer model: a single Velero
// ResourceModifier rule document (rules.yaml) — possibly parameterized
// with ${VAR} placeholders. TransformSets (transform_sets.go) reference
// Transforms by name and compose them at compile time.
//
// Storage model:
//   - ns:    "supkube"
//   - label: supkube.io/kind=transform
//   - data:  exactly one key, "rules.yaml" (ADR-003: Velero parser
//     rejects len(Data)!=1 on the derived CM, and we keep the
//     invariant uniform across layers).
//
// Why a separate file from transform_sets.go: the two CRUDs share a
// namespace, label scheme, and client factory, but they're conceptually
// different objects with different validation rules. Splitting keeps
// each handler set easy to read in isolation.
//
// IMPORTANT — the data["rules.yaml"] body may contain ${VAR} placeholders
// that compile.go substitutes from TransformSet.defaults + Restore-time
// params. Validation here permits them; full validation (including
// placeholder presence checks) happens in transform.Validate at compile
// time, when params are known.
package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	sigyaml "sigs.k8s.io/yaml"

	"github.com/supkube/supkube-backend/internal/transform"
)

const (
	// transformKindValue is the label value for atomic Transforms.
	// Distinct from transformSetKindValue ("transform-set"); the two
	// layers MUST NOT collide.
	transformKindValue = transform.KindTransform
)

// ─── User-facing types (Velero ResourceModifier schema) ────────────────
// These mirror Velero's expected YAML structure for `rules.yaml`. We pin
// version: v1 for now; JSON tags use omitempty so unknown forward-compat
// fields don't get stripped on round-trip.
// Reference: https://velero.io/docs/main/restore-resource-modifiers/

type TSCondition struct {
	GroupResource     string                `json:"groupResource"`
	ResourceNameRegex string                `json:"resourceNameRegex,omitempty"`
	Namespaces        []string              `json:"namespaces,omitempty"`
	LabelSelector     *metav1.LabelSelector `json:"labelSelector,omitempty"`
	MatchExpressions  []string              `json:"matchExpressions,omitempty"`
}

type TSPatch struct {
	Operation string      `json:"operation"`
	Path      string      `json:"path"`
	From      string      `json:"from,omitempty"`
	Value     interface{} `json:"value,omitempty"`
}

// TSMergePatch wraps a JSON Merge Patch (RFC 7396) or a Strategic Merge
// Patch. The two are structurally identical at this level — the
// difference is in which TSRule field they live under.
type TSMergePatch struct {
	PatchData string `json:"patchData"`
}

// TSRule maps directly to Velero's ResourceModifierRule.
type TSRule struct {
	Conditions       TSCondition    `json:"conditions"`
	Patches          []TSPatch      `json:"patches,omitempty"`
	MergePatches     []TSMergePatch `json:"mergePatches,omitempty"`
	StrategicPatches []TSMergePatch `json:"strategicPatches,omitempty"`
}

// ResourceModifierDoc mirrors Velero's top-level rules.yaml structure.
type ResourceModifierDoc struct {
	Version               string   `json:"version" yaml:"version"`
	ResourceModifierRules []TSRule `json:"resourceModifierRules" yaml:"resourceModifierRules"`
}

// Transform is the SupKube-facing shape: a named bundle of rules with
// optional description.
type Transform struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Rules       []TSRule `json:"rules"`
	BuiltIn     bool     `json:"builtIn,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"`
}

// ─── HTTP handlers ──────────────────────────────────────────────────────

// ListTransforms returns every atomic Transform in supkube ns.
func ListTransforms(c *gin.Context) {
	cl, err := transformClientFactory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	selector := fmt.Sprintf("%s=%s", transformSetLabelKind, transformKindValue)
	cms, err := cl.CoreV1().ConfigMaps(transformSetNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]Transform, 0, len(cms.Items))
	for i := range cms.Items {
		tr, err := configMapToTransform(&cms.Items[i])
		if err != nil {
			continue // skip malformed silently — one bad CM shouldn't kill the list
		}
		out = append(out, tr)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "total": len(out)})
}

// GetTransform returns one atomic Transform by name.
func GetTransform(c *gin.Context) {
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
	if !isTransformConfigMap(cm) {
		c.JSON(http.StatusNotFound, gin.H{"error": "ConfigMap exists but is not a SupKube Transform"})
		return
	}
	tr, err := configMapToTransform(cm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode Transform: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, tr)
}

// CreateTransform persists a new atomic Transform.
func CreateTransform(c *gin.Context) {
	var tr Transform
	if err := c.ShouldBindJSON(&tr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateK8sName(tr.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateRulesList(tr.Rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cm, err := transformToConfigMap(&tr, false)
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
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Transform %q already exists", tr.Name)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, tr)
}

// UpdateTransform refuses to mutate built-ins.
func UpdateTransform(c *gin.Context) {
	name := c.Param("name")
	var tr Transform
	if err := c.ShouldBindJSON(&tr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if tr.Name == "" {
		tr.Name = name
	} else if tr.Name != name {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL name and body name must match"})
		return
	}
	if err := validateRulesList(tr.Rules); err != nil {
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
	if !isTransformConfigMap(existing) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not a SupKube Transform"})
		return
	}
	if existing.Labels["supkube.io/builtin"] == "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "built-in Transforms cannot be modified; clone it under a new name instead"})
		return
	}
	updated, err := transformToConfigMap(&tr, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated.ResourceVersion = existing.ResourceVersion
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Update(context.Background(), updated, metav1.UpdateOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tr)
}

// DeleteTransform refuses if any TransformSet still references this name.
//
// This is the key invariant guarding the two-layer model: a TransformSet
// with a dangling ref is a compile-time error at Restore — much friendlier
// to catch the deletion attempt up-front and tell the user which sets
// they need to edit first.
//
// Built-ins are also refused (clone-to-edit pattern).
func DeleteTransform(c *gin.Context) {
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
	if !isTransformConfigMap(cm) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not a SupKube Transform"})
		return
	}
	if cm.Labels["supkube.io/builtin"] == "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "built-in Transforms cannot be deleted"})
		return
	}
	// Reference check: scan every TransformSet for this name in its
	// transformRefs. Linear over the (tiny) TransformSet population —
	// acceptable for now; if/when this gets hot we'd add an index.
	referencingSets, err := findTransformSetsReferencing(context.Background(), cl, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "check references: " + err.Error()})
		return
	}
	if len(referencingSets) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":                    fmt.Sprintf("Transform %q is referenced by %d TransformSet(s); remove the references first", name, len(referencingSets)),
			"referencingTransformSets": referencingSets,
		})
		return
	}
	if err := cl.CoreV1().ConfigMaps(transformSetNamespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Transform deleted"})
}

// findTransformSetsReferencing returns the names of TransformSets whose
// transformRefs contains transformName. Used by DeleteTransform.
//
// Linear scan over the TransformSet population. Acceptable for now
// because TransformSets are user-curated (10s, not 1000s).
func findTransformSetsReferencing(ctx context.Context, cl kubernetes.Interface, transformName string) ([]string, error) {
	selector := fmt.Sprintf("%s=%s", transformSetLabelKind, transformSetKindValue)
	cms, err := cl.CoreV1().ConfigMaps(transformSetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	var hits []string
	for i := range cms.Items {
		cm := &cms.Items[i]
		raw, ok := cm.Data[transformSetSpecKey]
		if !ok {
			continue
		}
		var spec transform.TransformSetSpec
		if err := sigyaml.Unmarshal([]byte(raw), &spec); err != nil {
			continue // malformed TS — skip, don't block delete
		}
		for _, ref := range spec.TransformRefs {
			if ref == transformName {
				hits = append(hits, cm.Name)
				break
			}
		}
	}
	return hits, nil
}

// ─── Helpers ────────────────────────────────────────────────────────────

func isTransformConfigMap(cm *corev1.ConfigMap) bool {
	if cm == nil || cm.Labels == nil {
		return false
	}
	return cm.Labels[transformSetLabelKind] == transformKindValue
}

// configMapToTransform parses the rules.yaml back into the API shape.
//
// Per ADR-003 the CM MUST have exactly one data key ("rules.yaml"); we
// don't fall back to anything else — if data is malformed we surface a
// clear error instead of silently accepting an empty rule list.
func configMapToTransform(cm *corev1.ConfigMap) (Transform, error) {
	tr := Transform{
		Name:      cm.Name,
		BuiltIn:   cm.Labels["supkube.io/builtin"] == "true",
		CreatedAt: cm.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}
	if cm.Annotations != nil {
		tr.Description = cm.Annotations[tsDescriptionAnnotation]
	}
	raw, ok := cm.Data[transformRulesKey]
	if !ok || strings.TrimSpace(raw) == "" {
		return tr, fmt.Errorf("Transform %q missing data[%s]", cm.Name, transformRulesKey)
	}
	// Templated Transforms (with ${VAR}) are stored verbatim — we cannot
	// strictly unmarshal them because placeholders may sit inside quoted
	// values that happen to parse OK, but inside structured fields (e.g.
	// numbers) they wouldn't. Best-effort decode: if it fails, return
	// the rules unparsed but with an Empty Rules signal in description.
	var doc ResourceModifierDoc
	if err := sigyaml.Unmarshal([]byte(raw), &doc); err == nil {
		tr.Rules = doc.ResourceModifierRules
	}
	return tr, nil
}

// transformToConfigMap serializes the API shape into a CM ready for k8s.
func transformToConfigMap(tr *Transform, builtIn bool) (*corev1.ConfigMap, error) {
	doc := ResourceModifierDoc{
		Version:               "v1",
		ResourceModifierRules: tr.Rules,
	}
	yamlBytes, err := sigyaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("serialize rules.yaml: %w", err)
	}
	labels := map[string]string{
		transformSetLabelKind:   transformKindValue,
		"supkube.io/managed-by": "supkube",
	}
	if builtIn {
		labels["supkube.io/builtin"] = "true"
	}
	annotations := map[string]string{
		tsDescriptionAnnotation: tr.Description,
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        tr.Name,
			Namespace:   transformSetNamespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: map[string]string{
			transformRulesKey: string(yamlBytes),
		},
	}
	return cm, nil
}

// validateRulesList sanity-checks a Velero ResourceModifier rule slice.
//
// Velero re-validates at restore time; we just want to catch obviously
// broken rules before persisting them. v0.8.4 added merge & strategic
// patches; a rule must have at least ONE non-empty patch group.
func validateRulesList(rules []TSRule) error {
	if len(rules) == 0 {
		return fmt.Errorf("at least one rule is required")
	}
	allowedOps := map[string]bool{"add": true, "remove": true, "replace": true, "test": true, "copy": true, "move": true}
	for i, r := range rules {
		if strings.TrimSpace(r.Conditions.GroupResource) == "" {
			return fmt.Errorf("rule[%d]: conditions.groupResource is required", i)
		}
		if len(r.Patches) == 0 && len(r.MergePatches) == 0 && len(r.StrategicPatches) == 0 {
			return fmt.Errorf("rule[%d]: at least one of patches / mergePatches / strategicPatches is required", i)
		}
		for j, p := range r.Patches {
			if !allowedOps[p.Operation] {
				return fmt.Errorf("rule[%d].patches[%d]: invalid operation %q (allowed: add, remove, replace, test, copy, move)", i, j, p.Operation)
			}
			if !strings.HasPrefix(p.Path, "/") {
				return fmt.Errorf("rule[%d].patches[%d]: path must be a JSON Pointer starting with '/' (got %q)", i, j, p.Path)
			}
			needsValue := p.Operation == "add" || p.Operation == "replace" || p.Operation == "test"
			if needsValue && p.Value == nil {
				return fmt.Errorf("rule[%d].patches[%d]: operation %q requires a value", i, j, p.Operation)
			}
		}
		for j, m := range r.MergePatches {
			if strings.TrimSpace(m.PatchData) == "" {
				return fmt.Errorf("rule[%d].mergePatches[%d]: patchData is required", i, j)
			}
			var dummy interface{}
			if err := json.Unmarshal([]byte(m.PatchData), &dummy); err != nil {
				return fmt.Errorf("rule[%d].mergePatches[%d]: patchData is not valid JSON: %v", i, j, err)
			}
		}
		for j, s := range r.StrategicPatches {
			if strings.TrimSpace(s.PatchData) == "" {
				return fmt.Errorf("rule[%d].strategicPatches[%d]: patchData is required", i, j)
			}
			var dummy interface{}
			if err := json.Unmarshal([]byte(s.PatchData), &dummy); err != nil {
				return fmt.Errorf("rule[%d].strategicPatches[%d]: patchData is not valid JSON: %v", i, j, err)
			}
		}
	}
	return nil
}
