// Package v1: transform_sets.go
//
// Transform Set CRUD (v0.8.2)
// ───────────────────────────
// A Transform Set is a named bundle of JSONPath patch rules applied during
// Velero restore. Velero's native primitive is `spec.resourceModifierRef`
// which points to a ConfigMap with a `version: v1` rules document. SupKube
// stores Transform Sets *as* those ConfigMaps (in the velero namespace,
// labeled supkube.io/kind=transform-set) so:
//
//   1. No new CRD to install / maintain
//   2. Velero understands them natively at restore time
//   3. Power users can `kubectl get cm -n velero` and see what we created
//
// Why we don't invent our own CRD layer above this:
//   - It would just be a thin wrapper around the same JSONPath rules
//   - We'd have to maintain a controller that syncs CRD → ConfigMap
//   - Velero already validates the rules at restore time; duplicating that
//     in a controller is wasted complexity
//
// Schema inside the ConfigMap data map:
//
//   data:
//     description:  free-form prose shown in UI list
//     rules.yaml:   the actual ResourceModifier YAML Velero consumes
//
// We also store a JSON-serialized "rules" key for fast UI rendering so the
// frontend doesn't have to parse YAML in the browser.
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
	sigyaml "sigs.k8s.io/yaml"

	"github.com/supkube/supkube-backend/internal/k8s"
)

const (
	transformSetNamespace = "velero"
	transformSetLabelKind = "supkube.io/kind"
	transformSetKindValue = "transform-set"
)

// ─── Schema types matching Velero's ResourceModifier format ─────────────
// Reference: https://velero.io/docs/main/restore-resource-modifiers/
//
// We pin to version "v1". A future Velero release may add fields; the JSON
// tags here use omitempty so unknown forward-compatible fields don't get
// stripped on round-trip.

type TSCondition struct {
	GroupResource     string                `json:"groupResource"`     // e.g. "services" or "deployments.apps"
	ResourceNameRegex string                `json:"resourceNameRegex,omitempty"`
	Namespaces        []string              `json:"namespaces,omitempty"`
	LabelSelector     *metav1.LabelSelector `json:"labelSelector,omitempty"`
	MatchExpressions  []string              `json:"matchExpressions,omitempty"`
}

type TSPatch struct {
	Operation string      `json:"operation"`       // "add" | "remove" | "replace" | "test" | "copy" | "move"
	Path      string      `json:"path"`            // JSON Pointer e.g. "/spec/ports/0/nodePort"
	From      string      `json:"from,omitempty"`  // for copy/move
	Value     interface{} `json:"value,omitempty"` // omitempty so "remove" doesn't include null
}

// TSMergePatch wraps a JSON Merge Patch (RFC 7396) — silently no-ops on
// missing fields, which is exactly what we want for "strip-if-present"
// operations like removing volumeName / clusterIP / loadBalancerIP from
// resources whose backup may or may not still carry the field.
type TSMergePatch struct {
	PatchData string `json:"patchData"`
}

// TSRule maps directly to Velero's ResourceModifierRule.
//
// A rule MUST have at least one of Patches / MergePatches / StrategicPatches
// (validation enforces this). Multiple types can co-exist in the same rule
// but in practice we generate one rule per modification kind for clarity.
type TSRule struct {
	Conditions       TSCondition    `json:"conditions"`
	Patches          []TSPatch      `json:"patches,omitempty"`
	// v0.8.4: merge & strategic patches. Velero applies them as
	//   - mergePatches:     RFC 7396 JSON Merge Patch (silent no-op on
	//                       missing fields; arrays REPLACE not merge)
	//   - strategicPatches: K8s strategic merge (knows array merge keys,
	//                       e.g. Service.spec.ports uses `port` as key)
	MergePatches     []TSMergePatch `json:"mergePatches,omitempty"`
	StrategicPatches []TSMergePatch `json:"strategicPatches,omitempty"`
}

// ResourceModifierDoc mirrors Velero's expected YAML structure.
type ResourceModifierDoc struct {
	Version                string   `json:"version" yaml:"version"`
	ResourceModifierRules  []TSRule `json:"resourceModifierRules" yaml:"resourceModifierRules"`
}

// TransformSet is the SupKube-facing shape. List endpoints return this;
// it's the ConfigMap deserialized + decorated.
type TransformSet struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Rules       []TSRule `json:"rules"`
	BuiltIn     bool     `json:"builtIn,omitempty"`     // true if managed by SupKube install
	UsageCount  int      `json:"usageCount,omitempty"`  // future: count of Restores referencing this
	CreatedAt   string   `json:"createdAt,omitempty"`
}

// ─── HTTP handlers ──────────────────────────────────────────────────────

// ListTransformSets returns every ConfigMap in velero ns that carries
// our identifying label. Built-in templates ship with
// supkube.io/builtin=true so the UI can prevent accidental deletion.
func ListTransformSets(c *gin.Context) {
	cl, err := k8s.GetClient()
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
			// Skip malformed entries silently — log-only would be nicer but
			// we don't want one bad CM to kill the whole list.
			continue
		}
		out = append(out, ts)
	}
	c.JSON(http.StatusOK, gin.H{"items": out, "total": len(out)})
}

// GetTransformSet returns one Transform Set by name.
func GetTransformSet(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetClient()
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
		c.JSON(http.StatusNotFound, gin.H{"error": "ConfigMap exists but is not a SupKube Transform Set"})
		return
	}
	ts, err := configMapToTransformSet(cm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode transform set: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, ts)
}

// CreateTransformSet body schema is the user-facing TransformSet shape.
// We translate it back to the Velero ResourceModifierDoc + persist as
// ConfigMap.
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
	if err := validateTransformSetRules(ts.Rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cm, err := transformSetToConfigMap(&ts, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cl, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Create(context.Background(), cm, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Transform Set %q already exists", ts.Name)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ts)
}

// UpdateTransformSet refuses to mutate built-in templates so users can't
// silently break the seeded defaults.
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
	if err := validateTransformSetRules(ts.Rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cl, err := k8s.GetClient()
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
		c.JSON(http.StatusNotFound, gin.H{"error": "not a SupKube Transform Set"})
		return
	}
	if existing.Labels["supkube.io/builtin"] == "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "built-in Transform Sets cannot be modified; clone it under a new name instead"})
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

// ConflictFix is one entry in the batch the UI sends to ApplyConflictFixes.
// Matches the relevant fields of a Pre-flight Conflict.
type ConflictFix struct {
	ConflictKind       string         `json:"conflictKind"`
	Artifact           ArtifactRef    `json:"artifact"`
	SuggestedTransform *TransformRule `json:"suggestedTransform"`
}

// ApplyConflictFixes is the v0.8.3 batched "one-click fix" endpoint.
//
// Why batched instead of one-at-a-time (as v0.8.2 did):
//
//   Velero's Restore.spec.resourceModifier is a SINGLE LocalObjectReference.
//   A Restore can reference exactly ONE Transform Set ConfigMap. So if the
//   user clicks "Apply Fix" on 4 conflicts, we MUST merge them into one
//   ConfigMap, otherwise only the last one's patches actually run and the
//   restore fails on the other 3 conflicts.
//
// The frontend therefore sends the FULL CURRENT LIST of applied fixes every
// time. We replace the consolidated Transform Set with the new union. The
// ConfigMap name is derived from the restoreName so the user can re-open
// the drawer later and we end up with a stable, predictable CM.
//
// Body shape:
//
//	{
//	  "restoreName": "test-app-backup-...-restore-220823",
//	  "fixes": [ ConflictFix, ConflictFix, ... ]
//	}
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

	// Build one rule per fix, narrowly scoped to the exact ns + name.
	// v0.8.4: each SuggestedTransform carries exactly one variant —
	// JSON Patch, Merge Patch, or Strategic Merge Patch. We dispatch to
	// the right field in TSRule based on which is populated.
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
			// Skip — no patch payload, nothing actionable.
			continue
		}
		rules = append(rules, rule)
		descriptionParts = append(descriptionParts,
			fmt.Sprintf("%s on %s/%s", fix.ConflictKind, fix.Artifact.Namespace, fix.Artifact.Name))
	}
	if err := validateTransformSetRules(rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "generated rules failed validation: " + err.Error()})
		return
	}

	// Stable name per restore session. Truncate so the suffix fits.
	prefix := "preflight-fix-"
	if len(req.RestoreName) > 253-len(prefix) {
		req.RestoreName = req.RestoreName[:253-len(prefix)]
	}
	tsName := prefix + req.RestoreName

	ts := TransformSet{
		Name:        tsName,
		Description: fmt.Sprintf("Auto-generated by Pre-flight (%d fix(es)): %s", len(rules), strings.Join(descriptionParts, "; ")),
		Rules:       rules,
	}

	cl, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cm, err := transformSetToConfigMap(&ts, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Mark as auto-generated for future GC.
	cm.Labels["supkube.io/auto-generated"] = "true"

	// Upsert: try Create first, fall back to Update if it already exists.
	// Idempotent re-clicks just refresh the rules.
	created := true
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Create(context.Background(), cm, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Fetch + patch existing
		existing, gerr := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), tsName, metav1.GetOptions{})
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
		"transformSetName": tsName,
		"created":          created,
		"ruleCount":        len(rules),
	})
}

// lowerKindToResource is a tiny, hand-rolled pluralizer for the kinds we
// generate Transform Sets for. Full pluralization is in RESTMapper but we
// don't need it here — the Pre-flight detectors only emit a fixed set of
// Kinds.
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
		// Conservative fallback: lowercase + s. Works for most simple Kinds
		// but not e.g. Endpoints (already plural) — those aren't on our
		// Pre-flight whitelist anyway.
		return strings.ToLower(kind) + "s"
	}
}

// regexpEscape escapes ^ . $ ( ) [ ] { } | \ + * ? — used to build a
// safe "name equals X" regex without pulling in the regexp/syntax package.
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

// shortHash returns a 6-char hash of input for ConfigMap-name suffixes.
// FNV-1a is fine — not cryptographic, just needs to be deterministic and
// short.
func shortHash(s string) string {
	const fnv64Offset = 0xcbf29ce484222325
	const fnv64Prime = 0x100000001b3
	var h uint64 = fnv64Offset
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnv64Prime
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var b [6]byte
	for i := range b {
		b[i] = alphabet[h%uint64(len(alphabet))]
		h /= uint64(len(alphabet))
	}
	return string(b[:])
}

// DeleteTransformSet refuses to delete built-in templates.
func DeleteTransformSet(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetClient()
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
		c.JSON(http.StatusNotFound, gin.H{"error": "not a SupKube Transform Set"})
		return
	}
	if cm.Labels["supkube.io/builtin"] == "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "built-in Transform Sets cannot be deleted"})
		return
	}
	if err := cl.CoreV1().ConfigMaps(transformSetNamespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Transform Set deleted"})
}

// ─── Helpers ────────────────────────────────────────────────────────────

func isTransformSetConfigMap(cm *corev1.ConfigMap) bool {
	if cm == nil || cm.Labels == nil {
		return false
	}
	return cm.Labels[transformSetLabelKind] == transformSetKindValue
}

// v0.8.3 ConfigMap layout — IMPORTANT
// ────────────────────────────────────
// Velero's resource_modifiers parser REQUIRES `len(cm.Data) == 1` (see
// velero/pkg/util/resourcemodifiers/resource_modifiers.go:GetResourceModifiersFromConfig).
// Any extra Data key triggers "illegal resource modifiers" at restore-time
// validation — exactly what bit us in v0.8.2.
//
// So we keep the ConfigMap squeaky-clean:
//   - data["rules.yaml"]               only thing Velero sees
//   - annotations["supkube.io/description"]  human-readable label
//   - annotations["supkube.io/rules-json"]   pre-parsed JSON for fast UI render
//   - labels["supkube.io/kind"] etc. — SupKube identification (Velero ignores)
const (
	tsDescriptionAnnotation = "supkube.io/description"
	tsRulesJSONAnnotation   = "supkube.io/rules-json"
)

func configMapToTransformSet(cm *corev1.ConfigMap) (TransformSet, error) {
	ts := TransformSet{
		Name:        cm.Name,
		Description: cm.Annotations[tsDescriptionAnnotation],
		BuiltIn:     cm.Labels["supkube.io/builtin"] == "true",
		CreatedAt:   cm.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}
	// Prefer the rules-json annotation (fast); fall back to parsing the
	// canonical rules.yaml in Data.
	if raw := cm.Annotations[tsRulesJSONAnnotation]; raw != "" {
		var doc ResourceModifierDoc
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			// Fall through to rules.yaml — annotation may be stale or
			// hand-edited by kubectl.
		} else {
			ts.Rules = doc.ResourceModifierRules
		}
	}
	if len(ts.Rules) == 0 {
		if raw, ok := cm.Data["rules.yaml"]; ok && raw != "" {
			var doc ResourceModifierDoc
			if err := sigyaml.Unmarshal([]byte(raw), &doc); err != nil {
				return ts, fmt.Errorf("parse rules.yaml: %w", err)
			}
			ts.Rules = doc.ResourceModifierRules
		}
	}
	return ts, nil
}

func transformSetToConfigMap(ts *TransformSet, builtIn bool) (*corev1.ConfigMap, error) {
	doc := ResourceModifierDoc{
		Version:               "v1",
		ResourceModifierRules: ts.Rules,
	}
	yamlBytes, err := sigyaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("serialize rules.yaml: %w", err)
	}
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("serialize rules.json: %w", err)
	}
	labels := map[string]string{
		transformSetLabelKind:   transformSetKindValue,
		"supkube.io/managed-by": "supkube",
	}
	if builtIn {
		labels["supkube.io/builtin"] = "true"
	}
	annotations := map[string]string{
		tsDescriptionAnnotation: ts.Description,
		tsRulesJSONAnnotation:   string(jsonBytes),
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ts.Name,
			Namespace:   transformSetNamespace,
			Labels:      labels,
			Annotations: annotations,
		},
		// Only ONE key allowed here — Velero rejects multi-key Data.
		Data: map[string]string{
			"rules.yaml": string(yamlBytes),
		},
	}
	return cm, nil
}

// validateTransformSetRules sanity-checks each rule before persisting.
// Velero will re-validate at restore time; this is just to avoid storing
// obviously broken rules.
//
// v0.8.4: rules may now contain JSON Patches OR Merge Patches OR
// Strategic Patches (Velero supports all three). A rule must have at least
// ONE non-empty patch group.
func validateTransformSetRules(rules []TSRule) error {
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
			// Quick JSON validity check — Velero parses this at restore
			// time but we'd rather catch obviously-broken JSON early.
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
