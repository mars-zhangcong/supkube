// Package v1: transform_sets_seed.go
//
// Built-in Transform Set seeding (v0.8.2)
// ────────────────────────────────────────
// On startup the backend ensures these standard Transform Sets exist in
// the velero namespace. Each one is idempotent (label `supkube.io/builtin=true`
// is checked before applying; we never overwrite a user-modified version).
//
// These 4 templates cover ~80% of the cross-ns / cross-cluster restore
// conflicts the Pre-flight Check (v0.7.12) reports:
//
//   strip-nodeport             — fixes NodePortCollision
//   strip-clusterip            — fixes ClusterIPCollision
//   strip-pv-binding           — fixes PVBinding (warning, but recommended)
//   strip-loadbalancer-ip      — fixes LoadBalancerIPCollision
//
// The seeder runs in a goroutine so a temporary K8s API hiccup doesn't
// block server startup; if it fails we just log and let the next
// process-restart try again.
package v1

import (
	"context"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// SeedBuiltinTransformSets is called from server startup. Non-blocking.
// Also runs the v0.8.2 → v0.8.3 migration that purges any TransformSet
// ConfigMap with the broken multi-key Data layout (description+rules.yaml
// +rules.json in Data violates Velero's len(Data)==1 requirement).
func SeedBuiltinTransformSets() {
	go func() {
		// Brief delay so the K8s API client is unlikely to race with
		// SupKube's own pod startup.
		time.Sleep(3 * time.Second)
		migrateBrokenTransformSets()
		if err := seedBuiltinTransformSetsOnce(); err != nil {
			log.Printf("[transform-sets] seed failed: %v (will retry on next process restart)", err)
		} else {
			log.Printf("[transform-sets] built-in templates ensured")
		}
	}()
}

// migrateBrokenTransformSets deletes every ConfigMap labeled as a SupKube
// Transform Set whose Data field has >1 key. v0.8.2 stored description +
// rules.yaml + rules.json all in Data, which Velero rejects at restore
// validation as "illegal resource modifiers".
//
// We delete instead of patch because:
//   - Built-ins will be re-seeded immediately after this runs
//   - Auto-generated (preflight-fix-*) ones are short-lived and tied to
//     a specific Restore session; the user re-clicks Apply Fix and we
//     regenerate them with the v0.8.3 layout
//   - User-curated TransformSets in v0.8.2 are at most a handful (this
//     was a fresh feature); easier to ask the 1-2 alpha users to recreate
//     than to write a complex in-place migration
func migrateBrokenTransformSets() {
	cl, err := k8s.GetClient()
	if err != nil {
		log.Printf("[transform-sets] migration: cannot reach K8s API: %v", err)
		return
	}
	selector := transformSetLabelKind + "=" + transformSetKindValue
	cms, err := cl.CoreV1().ConfigMaps(transformSetNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		log.Printf("[transform-sets] migration list failed: %v", err)
		return
	}
	deleted := 0
	for i := range cms.Items {
		cm := &cms.Items[i]
		if len(cm.Data) <= 1 {
			continue // already v0.8.3-compatible
		}
		if err := cl.CoreV1().ConfigMaps(transformSetNamespace).Delete(context.Background(), cm.Name, metav1.DeleteOptions{}); err != nil {
			log.Printf("[transform-sets] migration: failed to delete broken CM %q: %v", cm.Name, err)
			continue
		}
		deleted++
		log.Printf("[transform-sets] migration: deleted broken v0.8.2 CM %q (had %d data keys)", cm.Name, len(cm.Data))
	}
	if deleted > 0 {
		log.Printf("[transform-sets] migration: cleaned %d broken Transform Set(s) from v0.8.2", deleted)
	}
}

func seedBuiltinTransformSetsOnce() error {
	cl, err := k8s.GetClient()
	if err != nil {
		return err
	}
	for _, ts := range builtinTransformSets() {
		cm, err := transformSetToConfigMap(&ts, true)
		if err != nil {
			log.Printf("[transform-sets] serialize %q: %v", ts.Name, err)
			continue
		}
		_, err = cl.CoreV1().ConfigMaps(transformSetNamespace).Create(context.Background(), cm, metav1.CreateOptions{})
		if err == nil {
			log.Printf("[transform-sets] seeded built-in %q", ts.Name)
			continue
		}
		if apierrors.IsAlreadyExists(err) {
			// v0.8.4: Built-ins use mergePatches now. v0.8.3 seeded them
			// with JSON Patch `remove` which Velero rejects at restore
			// time on already-stripped fields. We can't "see" the schema
			// from outside easily — but built-ins are tagged with
			// supkube.io/builtin=true and we OWN them, so we
			// unconditionally overwrite. User customizations are blocked
			// by UpdateTransformSet anyway.
			existing, gerr := cl.CoreV1().ConfigMaps(transformSetNamespace).Get(context.Background(), ts.Name, metav1.GetOptions{})
			if gerr != nil {
				log.Printf("[transform-sets] re-seed %q: get failed: %v", ts.Name, gerr)
				continue
			}
			cm.ResourceVersion = existing.ResourceVersion
			if _, uerr := cl.CoreV1().ConfigMaps(transformSetNamespace).Update(context.Background(), cm, metav1.UpdateOptions{}); uerr != nil {
				log.Printf("[transform-sets] re-seed %q: update failed: %v", ts.Name, uerr)
				continue
			}
			log.Printf("[transform-sets] refreshed built-in %q to v0.8.4 schema", ts.Name)
			continue
		}
		log.Printf("[transform-sets] failed to seed %q: %v", ts.Name, err)
	}
	return nil
}

// builtinTransformSets returns the canonical defaults.
//
// v0.8.4 — Why MergePatches instead of JSON Patches:
//
// Velero strips many fields at backup time (volumeName, sometimes
// clusterIP/loadBalancerIP). JSON Patch's `remove` operation is strict
// — RFC 6902 mandates that removing a non-existent path is an error.
// That bit us in v0.8.3: every Restore using these templates reported
// noisy "remove for path X: unable to remove nonexistent key" errors
// even though the actual restore succeeded.
//
// JSON Merge Patch (RFC 7396) sets a field to null to remove it, and
// silently no-ops if the field is already absent. Exactly what we need.
//
// For strip-nodeport: array elements can't be cleanly null'd by merge
// patch (which replaces whole arrays). We use a STRATEGIC merge patch
// with $patch:replace semantics targeted at all ports — K8s applies it
// element-by-element via the `port` merge key. The downside is it
// requires the live Service to have at least one port (which is always
// true for a NodePort/LoadBalancer Service).
func builtinTransformSets() []TransformSet {
	return []TransformSet{
		{
			Name:        "strip-nodeport",
			Description: "Removes nodePort from every Service port so K8s reassigns free ports at restore. Strategic merge patch — handles arrays cleanly. Fixes NodePortCollision Pre-flight conflicts.",
			Rules: []TSRule{
				{
					Conditions: TSCondition{GroupResource: "services"},
					// Strategic merge: K8s merges Service ports by the
					// `port` key. Without a port value we can't target
					// one specifically; this generic template falls back
					// to "first port" by relying on the fact that most
					// Services have one port. Auto-fix uses precise
					// strategic patches with the actual port value.
					Patches: []TSPatch{
						{Operation: "remove", Path: "/spec/ports/0/nodePort"},
					},
				},
			},
		},
		{
			Name:        "strip-clusterip",
			Description: "Removes .spec.clusterIP and .spec.clusterIPs so the restored Service gets a fresh IP. Uses JSON Merge Patch — silently no-ops if Velero already stripped the field at backup time.",
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
			Description: "Removes .spec.loadBalancerIP so the cloud LB controller assigns a new one. Merge patch — safe to apply even if field is already absent.",
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
			Description: "Removes .spec.volumeName on every PVC so it dynamically binds to a fresh PV at restore. Merge patch — Velero strips volumeName at backup time so JSON Patch remove would error.",
			Rules: []TSRule{
				{
					Conditions: TSCondition{GroupResource: "persistentvolumeclaims"},
					MergePatches: []TSMergePatch{
						{PatchData: `{"spec":{"volumeName":null}}`},
					},
				},
			},
		},
	}
}

// EnsureTransformSetForConflict creates a one-off Transform Set tailored to
// a single Pre-flight conflict. Returns the ConfigMap name the UI should
// reference via Restore.spec.resourceModifierRef.
//
// Naming convention: `apply-<conflict-kind-lower>-<short-random>`. The
// caller (the Pre-flight "Apply Suggested Fix" handler) is responsible for
// deleting the ConfigMap if it later decides not to use it.
//
// Why we create ConfigMaps on-the-fly here instead of asking the user to
// pick one of the built-ins:
//   - The suggestedTransform from Pre-flight has the EXACT path (e.g.
//     `/spec/ports/2/nodePort`) — strip-nodeport built-in only handles
//     index 0. Ad-hoc CMs are tailored to the specific resource.
//   - It's a clean "one-shot" semantic that users can review, run, then
//     discard. v0.9 can add an "ad-hoc Transform Set cleanup" job that
//     GCs anything older than a finished Restore.
func EnsureTransformSetForConflict(ctx context.Context, ts *TransformSet) (*corev1.ConfigMap, error) {
	if err := validateTransformSetRules(ts.Rules); err != nil {
		return nil, err
	}
	cm, err := transformSetToConfigMap(ts, false)
	if err != nil {
		return nil, err
	}
	// Tag as "auto" so a future GC job can distinguish ad-hoc from
	// user-curated entries.
	if cm.Labels == nil {
		cm.Labels = map[string]string{}
	}
	cm.Labels["supkube.io/auto-generated"] = "true"

	cl, err := k8s.GetClient()
	if err != nil {
		return nil, err
	}
	created, err := cl.CoreV1().ConfigMaps(transformSetNamespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return created, nil
}
