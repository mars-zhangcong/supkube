// Package transform — public types and constants for the two-layer
// Transform / TransformSet model (PRD-002 v1.3).
//
// These constants are the single source of truth for the labels,
// namespace, and ConfigMap data keys used across the codebase. The
// `internal/api/v1` package imports them so the CRUD handlers and the
// startup seeder agree with the compiler on storage layout.
//
// Authoritative model
// ───────────────────
//   - Both Transforms and TransformSets live in ns `supkube`.
//   - Distinct kind labels:
//     atomic Transform    → supkube.io/kind=transform
//     wrapper TransformSet → supkube.io/kind=transform-set
//   - Each carries exactly one data key (ADR-003 / Velero invariant):
//     Transform     → data["rules.yaml"]            (Velero ResourceModifierDoc)
//     TransformSet  → data["transform-set.yaml"]    (TransformSetSpec)
//   - Derived (compiled) CMs live in `supkube` ns as well, carry
//     supkube.io/managed-by=transform-set-compiler, and use the same
//     rules.yaml data key as atomic Transforms (Velero's parser only
//     accepts that shape).
//   - The `velero` namespace is LEGACY-ONLY (pre-v1.3 single-layer CMs
//     that the startup migration splits into the two-layer shape).
package transform

const (
	// SupKubeNamespace is the canonical home for Transforms,
	// TransformSets, and derived (compiled) ConfigMaps.
	SupKubeNamespace = "supkube"

	// LegacyNamespace is the pre-PRD-002-v1.3 home for single-layer
	// TransformSets. The startup migration (see
	// internal/api/v1/transform_sets_seed.go) splits any CMs found here
	// into the two-layer shape and annotates the source. No NEW writes
	// should land in this namespace.
	LegacyNamespace = "velero"

	// LabelKind is the discriminator label used to identify SupKube-managed
	// ConfigMaps and to distinguish the two layers.
	LabelKind = "supkube.io/kind"

	// KindTransform marks an atomic Transform CM (the "brick").
	KindTransform = "transform"

	// KindTransformSet marks a TransformSet CM (the "menu" of refs).
	KindTransformSet = "transform-set"

	// TransformRulesKey is the SINGLE data key on an atomic Transform CM
	// AND on a derived (compiled) CM. Velero's resource_modifiers parser
	// rejects len(Data) != 1 — see the v0.8.2 bug postmortem in
	// transform_sets.go for why this is non-negotiable.
	TransformRulesKey = "rules.yaml"

	// TransformSetSpecKey is the single data key on a TransformSet wrapper
	// CM. The body is YAML-encoded TransformSetSpec. We use a distinct
	// key from TransformRulesKey so a TransformSet wrapper is never
	// mistakenly handed to Velero as a ResourceModifier.
	TransformSetSpecKey = "transform-set.yaml"

	// LabelManagedBy / LabelManagedByCompiler tag derived CMs so the GC
	// loop can list-and-clean them without confusing them with atomic
	// Transforms (which carry kind=transform too).
	LabelManagedBy         = "supkube.io/managed-by"
	LabelManagedByCompiler = "transform-set-compiler"

	// LabelSourceTransformSet records which TransformSet produced this
	// derived CM. Used for telemetry and for cache-bust on TS edits.
	LabelSourceTransformSet = "supkube.io/source-transform-set"

	// AnnotationRestoreRefs holds the list of Restore names currently
	// referencing this derived CM, as a JSON array.
	//
	// Why annotation not label: a CM can be referenced by many Restores
	// when reuse-by-hash hits, and labels can't hold structured values.
	// On Compile we append; on GC(restoreName) we remove and delete the
	// CM only when the list goes empty.
	AnnotationRestoreRefs = "supkube.io/restore-refs"

	// DerivedCMNamePrefix is the fixed prefix for compiled CMs.
	// Full format: <prefix><tsName>-<12-char-hash>.
	DerivedCMNamePrefix = "supkube-restore-rm-"

	// ─── compile.go (Agent N) 引用的额外常量 (2026-05-31 合并自 compile.go) ───

	// VeleroNamespace is where DERIVED CMs (compile results) land.
	// Velero's restore controller resolves resourceModifierRef in the
	// same ns as the Restore, which is conventionally `velero`. This is
	// semantically DIFFERENT from LegacyNamespace (which marks where the
	// pre-v1.3 single-layer CMs *used to* live before the migration ran).
	// Concretely: LegacyNamespace == VeleroNamespace == "velero" today,
	// but they serve two distinct roles, hence two names.
	VeleroNamespace = "velero"

	// KindCompiledRM marks a derived (compiled) ConfigMap so the GC
	// loop can list-and-clean them without confusing them with atomic
	// Transforms.
	KindCompiledRM = "compiled-resource-modifier"

	// LabelCompiledFrom records the source TransformSet name on a
	// derived CM. Used by cache-bust on TS edits + GC orphan detection.
	LabelCompiledFrom = "supkube.io/compiled-from"

	// LabelCompileHash is the 12-char SHA256 prefix of the compile
	// input. Same input ⇒ same hash ⇒ reuse existing derived CM.
	LabelCompileHash = "supkube.io/compile-hash"

	// LabelRestoreRef on a derived CM names the Restore currently
	// referencing it. Set by Compile, cleared by GC(restoreName).
	LabelRestoreRef = "supkube.io/restore-ref"
)

// TransformSetSpec is the on-disk YAML shape stored under
// data["transform-set.yaml"] on a TransformSet wrapper CM.
//
// Fields:
//   - TransformRefs: ordered list of atomic Transform NAMES the set
//     references. Order matters — Velero applies rules with the
//     "same path, later wins" semantic, so reordering changes
//     observable behavior. The compiler preserves this order verbatim.
//   - Description: human-prose blurb mirrored from the
//     supkube.io/description annotation. Kept here too for round-trip
//     cleanliness (the description survives a kubectl get -oyaml round
//     trip without consulting metadata).
//   - Defaults: optional ${VAR} substitution values applied during
//     Compile. Caller-provided params override matching keys here.
//     Omitempty so legacy TSes without defaults round-trip cleanly.
//
// Per-ref params (the PRD §4.1 (b) yaml example) are NOT modeled here:
// the v1.3 simplification keeps params global to the compile call.
// Re-introducing per-ref params is a future schema bump; this struct
// will gain a sibling list and the YAML tag will switch on its type.
type TransformSetSpec struct {
	TransformRefs []string          `json:"transformRefs" yaml:"transformRefs"`
	Description   string            `json:"description,omitempty" yaml:"description,omitempty"`
	Defaults      map[string]string `json:"defaults,omitempty" yaml:"defaults,omitempty"`
}
