// Package v1: transform_sets_seed.go
//
// Built-in Transform/TransformSet seeding + startup migrations
// ────────────────────────────────────────────────────────────
//
// PRD-002 v1.3 two-layer model rollout:
//
//  1. Seed atomic Transforms (strip-clusterip / strip-loadbalancer-ip /
//     strip-pv-binding / strip-nodeport) in `supkube` ns with
//     label supkube.io/kind=transform, data.rules.yaml.
//
//  2. Seed a default "common-cluster-relocation" TransformSet (also in
//     supkube ns, label supkube.io/kind=transform-set, data.transform-set.yaml)
//     that references all four built-ins, so the user has a working
//     pack out of the box.
//
//  3. Migrate any v0.8.x single-layer ConfigMaps still living in
//     `velero` ns (label supkube.io/kind=transform OR transform-set,
//     data.rules.yaml directly) into the new two-layer shape:
//     - atomic Transform `<orig>-rules` in supkube ns (rules copied verbatim)
//     - wrapper TransformSet `<orig>`  in supkube ns (transformRefs=[<orig>-rules])
//     The legacy velero-ns CM is ANNOTATED (not deleted) with
//     supkube.io/migrated-to=<orig> so admins keep a forensic trail
//     and a fallback if rollback is needed.
//
// The migration is best-effort: per-CM failures get annotated with
// supkube.io/migration-error=<msg> and logged, but do NOT block startup
// or other migrations. SupKube continues to serve traffic regardless.
package v1

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	sigyaml "sigs.k8s.io/yaml"

	"github.com/supkube/supkube-backend/internal/transform"
)

// SeedBuiltinTransformSets is called from server startup. Non-blocking.
// Runs the legacy-schema migration first, then seeds built-ins
// (Transforms + a default TransformSet).
func SeedBuiltinTransformSets() {
	go func() {
		// Brief delay so the K8s API client is unlikely to race with
		// SupKube's own pod startup.
		time.Sleep(3 * time.Second)
		migrateBrokenTransformSets()
		if err := seedBuiltinsOnce(); err != nil {
			log.Printf("[transform-sets] seed failed: %v (will retry on next process restart)", err)
		} else {
			log.Printf("[transform-sets] built-in Transforms + TransformSet ensured")
		}
	}()
}

// migrateBrokenTransformSets runs THREE distinct migrations:
//
//  1. v0.8.2 → v0.8.3: delete velero-ns ConfigMaps with len(Data)>1
//     (multi-key Data is rejected by Velero's parser).
//
//  2. v0.8.x → PRD-002 v1.2 schema rename inside velero ns: the legacy
//     value "transform-set" → "transform" for label supkube.io/kind.
//     This step is now LEGACY-ONLY because the new model has both
//     "transform" AND "transform-set" living in supkube ns — but old
//     clusters may still have CMs in velero ns with either label.
//
//  3. v0.8.x → PRD-002 v1.3 two-layer split: for every velero-ns CM
//     with our kind label (either value), CREATE the matching atomic
//     Transform + wrapper TransformSet pair in supkube ns and ANNOTATE
//     the source as migrated-to. Idempotent: a second run sees the
//     annotation and skips.
//
// Phase-0 (Agent J) renamed the kind label in place inside velero ns.
// This expansion does cross-namespace split; the in-place rename remains
// for the brief window between server start and full split completion.
func migrateBrokenTransformSets() {
	cl, err := transformClientFactory()
	if err != nil {
		log.Printf("[transform-sets] migration: cannot reach K8s API: %v", err)
		return
	}
	ctx := context.Background()

	// ── Migration (1): delete v0.8.2 multi-key broken CMs in velero ns ──
	// Cheap, no cross-ns work; do it first so subsequent migrations
	// don't try to copy garbage.
	for _, kindVal := range []string{"transform", "transform-set"} {
		selector := transformSetLabelKind + "=" + kindVal
		cms, err := cl.CoreV1().ConfigMaps(transformSetLegacyNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			log.Printf("[transform-sets] migration list (velero ns, %s): %v", kindVal, err)
			continue
		}
		for i := range cms.Items {
			cm := &cms.Items[i]
			if len(cm.Data) <= 1 {
				continue
			}
			if err := cl.CoreV1().ConfigMaps(transformSetLegacyNamespace).Delete(ctx, cm.Name, metav1.DeleteOptions{}); err != nil {
				log.Printf("[transform-sets] migration: failed to delete broken v0.8.2 CM %q: %v", cm.Name, err)
				continue
			}
			log.Printf("[transform-sets] migration: deleted broken v0.8.2 CM %q (had %d data keys)", cm.Name, len(cm.Data))
		}
	}

	// ── Migration (3): cross-ns split (velero single-layer → supkube two-layer) ──
	// We accept BOTH legacy label values because pre-Phase-0 clusters
	// had "transform-set", post-Phase-0 (Agent J) clusters have
	// "transform". Either way it's a single-layer rules CM in velero ns.
	for _, kindVal := range []string{"transform", "transform-set"} {
		selector := transformSetLabelKind + "=" + kindVal
		cms, err := cl.CoreV1().ConfigMaps(transformSetLegacyNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			log.Printf("[transform-sets] migration: list legacy %s in velero ns: %v", kindVal, err)
			continue
		}
		split := 0
		for i := range cms.Items {
			cm := &cms.Items[i]
			// Idempotency guard.
			if cm.Annotations[annotMigratedTo] != "" {
				continue
			}
			if err := splitLegacyToTwoLayer(ctx, cl, cm); err != nil {
				log.Printf("[transform-sets] migration: split %q failed: %v", cm.Name, err)
				annotateMigrationError(ctx, cl, cm, err)
				continue
			}
			split++
			log.Printf("[transform-sets] migration: split %q into supkube/%s-rules + supkube/%s", cm.Name, cm.Name, cm.Name)
		}
		if split > 0 {
			log.Printf("[transform-sets] migration: split %d legacy %s CM(s) into two-layer schema", split, kindVal)
		}
	}
}

// splitLegacyToTwoLayer takes a single-layer ConfigMap (velero ns,
// data.rules.yaml directly) and creates the matching two-layer pair
// in supkube ns:
//   - Transform     `<orig>-rules`   (kind=transform, data.rules.yaml=<orig rules>)
//   - TransformSet  `<orig>`         (kind=transform-set, transformRefs=[<orig>-rules])
//
// On success the source CM is annotated supkube.io/migrated-to=<orig> so
// subsequent migration runs (e.g. crash-restart) see the marker and skip.
// The source is NOT deleted — admins can `kubectl get cm -n velero` and
// see the original layout for rollback / debugging.
//
// If either of the new CMs already exists in supkube ns (e.g. partial
// previous migration), we proceed idempotently: the existing one stays
// and we annotate the source. We do NOT overwrite supkube-ns content
// because the user may have edited it post-migration.
func splitLegacyToTwoLayer(ctx context.Context, cl kubernetes.Interface, src *corev1.ConfigMap) error {
	// Pick the rules.yaml body — prefer the canonical key, fall back to
	// the very old "spec" key just in case (defensive).
	rulesYAML, ok := src.Data[transformRulesKey]
	if !ok || rulesYAML == "" {
		rulesYAML, ok = src.Data["spec"]
	}
	if !ok || rulesYAML == "" {
		return fmt.Errorf("source CM has no rules.yaml (data keys: %d)", len(src.Data))
	}

	atomicName := src.Name + "-rules"
	wrapperName := src.Name

	// Create atomic Transform if absent.
	atomicCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      atomicName,
			Namespace: transformSetNamespace,
			Labels: map[string]string{
				transformSetLabelKind:   transformKindValue,
				"supkube.io/managed-by": "supkube",
			},
			Annotations: map[string]string{
				tsDescriptionAnnotation: src.Annotations[tsDescriptionAnnotation],
				annotMigratedFrom:       transformSetLegacyNamespace + "/" + src.Name,
			},
		},
		Data: map[string]string{
			transformRulesKey: rulesYAML,
		},
	}
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Create(ctx, atomicCM, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create atomic Transform %q: %w", atomicName, err)
		}
		// Already exists — fine, idempotent.
	}

	// Create wrapper TransformSet if absent.
	spec := transform.TransformSetSpec{
		TransformRefs: []string{atomicName},
		Description:   src.Annotations[tsDescriptionAnnotation],
	}
	specYAML, err := sigyaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("serialize TransformSet spec for %q: %w", wrapperName, err)
	}
	wrapperCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wrapperName,
			Namespace: transformSetNamespace,
			Labels: map[string]string{
				transformSetLabelKind:   transformSetKindValue,
				"supkube.io/managed-by": "supkube",
			},
			Annotations: map[string]string{
				tsDescriptionAnnotation: src.Annotations[tsDescriptionAnnotation],
				annotMigratedFrom:       transformSetLegacyNamespace + "/" + src.Name,
			},
		},
		Data: map[string]string{
			transformSetSpecKey: string(specYAML),
		},
	}
	if _, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Create(ctx, wrapperCM, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create wrapper TransformSet %q: %w", wrapperName, err)
		}
	}

	// Annotate the source. Use a freshly fetched copy to avoid stale
	// resourceVersion conflicts when this migration runs alongside the
	// in-place rename (Migration step 2).
	fresh, err := cl.CoreV1().ConfigMaps(transformSetLegacyNamespace).Get(ctx, src.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("re-fetch source %q for annotation: %w", src.Name, err)
	}
	if fresh.Annotations == nil {
		fresh.Annotations = map[string]string{}
	}
	fresh.Annotations[annotMigratedTo] = transformSetNamespace + "/" + wrapperName
	delete(fresh.Annotations, annotMigrationErr) // clear any prior error
	if _, err := cl.CoreV1().ConfigMaps(transformSetLegacyNamespace).Update(ctx, fresh, metav1.UpdateOptions{}); err != nil {
		// Non-fatal: the split itself succeeded. Worst case is we
		// re-do the (now-idempotent) split on next startup.
		log.Printf("[transform-sets] migration: annotate source %q (non-fatal): %v", src.Name, err)
	}
	return nil
}

// annotateMigrationError records a per-CM failure so admins can spot it
// via `kubectl get cm -n velero -o yaml`. Non-fatal: if annotation itself
// fails we just log.
func annotateMigrationError(ctx context.Context, cl kubernetes.Interface, src *corev1.ConfigMap, migrationErr error) {
	fresh, err := cl.CoreV1().ConfigMaps(transformSetLegacyNamespace).Get(ctx, src.Name, metav1.GetOptions{})
	if err != nil {
		log.Printf("[transform-sets] migration: cannot re-fetch %q to annotate error: %v", src.Name, err)
		return
	}
	if fresh.Annotations == nil {
		fresh.Annotations = map[string]string{}
	}
	fresh.Annotations[annotMigrationErr] = migrationErr.Error()
	if _, err := cl.CoreV1().ConfigMaps(transformSetLegacyNamespace).Update(ctx, fresh, metav1.UpdateOptions{}); err != nil {
		log.Printf("[transform-sets] migration: annotate error on %q: %v", src.Name, err)
	}
}

// ─── Seeding ────────────────────────────────────────────────────────────

// seedBuiltinsOnce ensures every built-in atomic Transform exists, then
// ensures the default "common-cluster-relocation" TransformSet exists
// referencing them.
//
// Built-ins are always overwritten (they carry supkube.io/builtin=true,
// users can't modify them via the API — UpdateTransform refuses). This
// lets schema/content changes ship automatically on upgrade.
func seedBuiltinsOnce() error {
	cl, err := transformClientFactory()
	if err != nil {
		return err
	}
	ctx := context.Background()

	// 1) Atomic Transforms.
	for _, tr := range builtinTransforms() {
		cm, err := transformToConfigMap(&tr, true)
		if err != nil {
			log.Printf("[transform-sets] serialize Transform %q: %v", tr.Name, err)
			continue
		}
		if err := upsertConfigMap(ctx, cl, cm); err != nil {
			log.Printf("[transform-sets] seed Transform %q: %v", tr.Name, err)
		}
	}

	// 2) Default TransformSet packaging the built-ins.
	defaultTS := TransformSet{
		Name:        "common-cluster-relocation",
		Description: "Default pack for cross-cluster restores: strips clusterIP, nodePort, loadBalancerIP, and PV bindings so resources rebind cleanly.",
		TransformRefs: []TransformSetRef{
			{Name: "strip-clusterip"},
			{Name: "strip-loadbalancer-ip"},
			{Name: "strip-pv-binding"},
			{Name: "strip-nodeport"},
		},
	}
	cm, err := transformSetToConfigMap(&defaultTS, true)
	if err != nil {
		log.Printf("[transform-sets] serialize default TransformSet: %v", err)
		return nil // already logged
	}
	if err := upsertConfigMap(ctx, cl, cm); err != nil {
		log.Printf("[transform-sets] seed default TransformSet: %v", err)
	}
	return nil
}

// upsertConfigMap creates the CM if absent, updates it (preserving RV)
// if already present.
func upsertConfigMap(ctx context.Context, cl kubernetes.Interface, cm *corev1.ConfigMap) error {
	if _, err := cl.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{}); err == nil {
		log.Printf("[transform-sets] seeded built-in %s/%s", cm.Namespace, cm.Name)
		return nil
	} else if !apierrors.IsAlreadyExists(err) {
		return err
	}
	existing, gerr := cl.CoreV1().ConfigMaps(cm.Namespace).Get(ctx, cm.Name, metav1.GetOptions{})
	if gerr != nil {
		return fmt.Errorf("get for update: %w", gerr)
	}
	cm.ResourceVersion = existing.ResourceVersion
	if _, uerr := cl.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{}); uerr != nil {
		return fmt.Errorf("update: %w", uerr)
	}
	log.Printf("[transform-sets] refreshed built-in %s/%s", cm.Namespace, cm.Name)
	return nil
}

// builtinTransforms returns the canonical atomic Transforms.
//
// v0.8.4 lesson: prefer MergePatch (RFC 7396) over JSON Patch `remove`
// — Velero strips many fields at backup time, and JSON Patch errors on
// removing a non-existent path while Merge Patch silently no-ops.
func builtinTransforms() []Transform {
	return []Transform{
		{
			Name:        "strip-clusterip",
			Description: "Removes .spec.clusterIP and .spec.clusterIPs so the restored Service gets a fresh IP. Merge patch — no-ops if Velero already stripped the field.",
			Rules: []TSRule{
				{
					Conditions: TSCondition{GroupResource: "services"},
					MergePatches: []TSMergePatch{
						{PatchData: `{"spec":{"clusterIP":null,"clusterIPs":null}}`},
					},
				},
			},
		},
		{
			Name:        "strip-loadbalancer-ip",
			Description: "Removes .spec.loadBalancerIP so the cloud LB controller assigns a new one.",
			Rules: []TSRule{
				{
					Conditions: TSCondition{GroupResource: "services"},
					MergePatches: []TSMergePatch{
						{PatchData: `{"spec":{"loadBalancerIP":null}}`},
					},
				},
			},
		},
		{
			Name:        "strip-pv-binding",
			Description: "Removes .spec.volumeName on every PVC so it dynamically binds to a fresh PV at restore.",
			Rules: []TSRule{
				{
					Conditions: TSCondition{GroupResource: "persistentvolumeclaims"},
					MergePatches: []TSMergePatch{
						{PatchData: `{"spec":{"volumeName":null}}`},
					},
				},
			},
		},
		{
			Name:        "strip-nodeport",
			Description: "Removes nodePort from the first Service port so K8s reassigns a free port. (Generic template; auto-fix uses precise per-port strategic patches.)",
			Rules: []TSRule{
				{
					Conditions: TSCondition{GroupResource: "services"},
					Patches: []TSPatch{
						{Operation: "remove", Path: "/spec/ports/0/nodePort"},
					},
				},
			},
		},
	}
}
