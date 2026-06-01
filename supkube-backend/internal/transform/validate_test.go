// Package transform: validate_test.go — security-focused tests for the
// ${VAR} substitution gate and the rules.yaml structural validator.
//
// PRD-002 v1.3 §4 finding #4 + task #135 explicit requirement:
//
//	"Validate rejects ${VAR} substitutions that would inject regex
//	metacharacters into a conditions.resourceNameRegex."
//
// Design note (and the reason this file exists alongside the other
// compile_test.go): the gate against regex-meta injection lives in
// applyParams (the substitution step), not in Validate. Validate keeps
// only the regexp.Compile syntactic probe so operators can still author
// Transforms with patterns like `.*` directly — that's not the threat
// model. The threat is unsafe DATA flowing through ${VAR} into a regex
// field; the substitution-time whitelist (paramValueSafeRE) closes that
// vector without breaking authored regex.
//
// What this file covers:
//   - paramValueSafeRE rejects regex metas in substituted values.
//   - End-to-end Compile rejects the canonical attack: ${name}=evil.*.
//   - paramValueSafeRE still accepts legitimate values (hostnames,
//     paths, versions, storage class names) so the gate isn't a false
//     positive on real operator inputs.
//   - Validate's existing structural checks (parse, version,
//     resourceModifierRules non-empty, regexp.Compile, size cap,
//     placeholder residue) still work — regression coverage in case
//     someone tightens or loosens Validate without re-reading the
//     contract.
package transform

import (
	"context"
	"strings"
	"testing"
)

// ─── Substitution-time gate (the task's explicit requirement) ──────────

func TestApplyParams_RejectsRegexMetaInValue(t *testing.T) {
	// Each case is a value that, if substituted naively into
	// resourceNameRegex, would widen the match scope beyond what the
	// operator wrote.
	cases := map[string]string{
		"task-spec-evil-star": "evil.*",
		"plus-quantifier":     "foo+",
		"question-mark":       "foo?",
		"alternation":         "a|b",
		"capture-group":       "(foo)",
		"char-class":          "[abc]",
		"bounded":             "foo{1,3}",
		"escape":              `foo\d`,
		"anchor-injection":    "^pwned$", // injects new anchors
		"trailing-star":       "foo*",
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := applyParams("resourceNameRegex: ${X}", map[string]string{"X": bad})
			if err == nil {
				t.Errorf("applyParams should reject ${X}=%q (regex meta injection), got nil", bad)
			}
		})
	}
}

func TestApplyParams_AcceptsLegitimateValues(t *testing.T) {
	// These values match real-world operator inputs: storage class
	// names, registry hostnames, namespace names, image paths, versions.
	// The whitelist must let all of these through, otherwise the gate
	// is too tight to ship.
	cases := map[string]string{
		"sc-name":        "csi-hostpath",
		"sc-with-digits": "gp3",
		"hostname":       "harbor.local",
		"host-port":      "harbor.local:5000",
		"registry-path":  "gcr.io/project-id",
		"image-path":     "harbor.local:5000/library/nginx",
		"k8s-ns":         "app-staging",
		"k8s-name":       "my-deployment-0",
		"version":        "v1.18.0",
		"semver-pre":     "v1.18.0-rc.1",
		"underscore":     "my_value",
		"dots":           "foo.bar.baz",
	}
	for name, good := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := applyParams("foo: ${X}", map[string]string{"X": good})
			if err != nil {
				t.Errorf("applyParams should accept legitimate value ${X}=%q, got error: %v", good, err)
			}
			if !strings.Contains(out, good) {
				t.Errorf("applyParams should substitute %q, got: %s", good, out)
			}
		})
	}
}

// TestCompile_E2E_RejectsRegexMetaInjection is the task's named acceptance
// case wired end-to-end through Compile. Mirrors the wording in the task
// brief: "Validate rejects `${name}` where name='evil.*'".
func TestCompile_E2E_RejectsRegexMetaInjection(t *testing.T) {
	tr := makeTransform("strip-by-name", `version: v1
resourceModifierRules:
  - conditions:
      groupResource: services
      resourceNameRegex: "^${name}$"
    mergePatches:
      - patchData: '{"spec":{"clusterIP":null}}'
`)
	ts := makeTransformSet("attack-set", []string{"strip-by-name"}, nil)
	cl := newFakeClient(tr, ts)

	_, err := Compile(context.Background(), cl, "attack-set", map[string]string{"name": "evil.*"})
	if err == nil {
		t.Fatalf("Compile must reject ${name}=evil.* (regex metacharacter injection); got nil")
	}
	// Don't pin a specific error string — Compile wraps applyParams's
	// error, and exact wording may evolve. Just confirm the param name
	// surfaces so the operator knows WHICH var was bad.
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should identify the offending param 'name'; got: %v", err)
	}
}

// ─── Validate structural regression coverage ───────────────────────────

func TestValidate_AcceptsLegitimateAuthoredRegex(t *testing.T) {
	// These patterns are LEGITIMATELY hand-authored by operators in
	// Transforms and must not be rejected — Validate gates structure
	// and syntactic validity, not regex content. (Content gating
	// happens upstream at the substitution boundary.)
	cases := map[string]string{
		"match-all":        ".*",
		"any-non-empty":    ".+",
		"anchored-literal": "^my-svc$",
		"prefix-glob":      "^pvc-data-.*$",
		"alternation":      "(staging|prod)-.*",
		"with-dots":        "my.svc.cluster.local",
	}
	for name, re := range cases {
		t.Run(name, func(t *testing.T) {
			doc := `version: v1
resourceModifierRules:
  - conditions:
      groupResource: services
      resourceNameRegex: "` + re + `"
    mergePatches:
      - patchData: '{"spec":{"clusterIP":null}}'
`
			if err := Validate(doc); err != nil {
				t.Errorf("Validate should accept authored regex %q, got: %v", re, err)
			}
		})
	}
}

func TestValidate_RejectsBrokenRegexSyntax(t *testing.T) {
	// regexp.Compile probe catches structurally invalid patterns.
	cases := map[string]string{
		"unclosed-group":        "(foo",
		"unclosed-class":        "[abc",
		"unbalanced-quantifier": "*foo",
	}
	for name, re := range cases {
		t.Run(name, func(t *testing.T) {
			doc := `version: v1
resourceModifierRules:
  - conditions:
      groupResource: services
      resourceNameRegex: "` + re + `"
    mergePatches:
      - patchData: '{"spec":{"clusterIP":null}}'
`
			if err := Validate(doc); err == nil {
				t.Errorf("Validate should reject syntactically broken regex %q", re)
			}
		})
	}
}

func TestValidate_RejectsResidualPlaceholder(t *testing.T) {
	// Defense in depth: even if applyParams forgot to substitute,
	// Validate catches it.
	doc := `version: v1
resourceModifierRules:
  - conditions:
      groupResource: services
      resourceNameRegex: "${forgotten}"
    mergePatches:
      - patchData: '{}'
`
	if err := Validate(doc); err == nil {
		t.Fatalf("Validate should reject rules.yaml with unsubstituted ${VAR}")
	}
}

func TestValidate_RejectsMissingVersion(t *testing.T) {
	doc := `resourceModifierRules:
  - conditions:
      groupResource: services
    mergePatches:
      - patchData: '{}'
`
	if err := Validate(doc); err == nil {
		t.Fatalf("Validate should reject rules.yaml without version field")
	}
}

func TestValidate_RejectsEmptyRules(t *testing.T) {
	doc := `version: v1
resourceModifierRules: []
`
	if err := Validate(doc); err == nil {
		t.Fatalf("Validate should reject empty resourceModifierRules")
	}
}
