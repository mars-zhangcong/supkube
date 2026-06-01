package v1

import (
	"context"
	"fmt"
	"testing"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// conflictInjectingClient wraps a client.Client and returns an optimistic-lock
// Conflict on the first N Update calls, then delegates to the real client.
// This deterministically reproduces the race that PatchSchedule hit in prod:
// a controller reconcile bumps resourceVersion between our Get and Update.
type conflictInjectingClient struct {
	client.Client
	conflictsLeft int
	updateCalls   int
}

func (c *conflictInjectingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.updateCalls++
	if c.conflictsLeft > 0 {
		c.conflictsLeft--
		// apierrors.IsConflict(...) must be true for retry.RetryOnConflict to retry.
		return apierrors.NewConflict(
			schema.GroupResource{Group: "velero.io", Resource: "schedules"},
			obj.GetName(),
			fmt.Errorf("the object has been modified; please apply your changes to the latest version and try again"),
		)
	}
	return c.Client.Update(ctx, obj, opts...)
}

// TestPatchPolicyWithRetry_RecoversFromConflict is the deterministic regression
// guard for the pause/resume 500 bug: patchPolicyWithRetry must transparently
// recover from a 409 Conflict by re-fetching and re-applying, NOT surface it.
// (Before the fix, the single un-retried Update returned 409 → handler 500;
// the e2e test caught it only intermittently.)
func TestPatchPolicyWithRetry_RecoversFromConflict(t *testing.T) {
	const name = "policy-under-test"

	scheme := runtime.NewScheme()
	if err := velerov1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	sched := &velerov1.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "velero",
			Labels: map[string]string{
				labelPolicyName: name,
				labelPolicyRole: roleSnapshot,
			},
		},
		Spec: velerov1.ScheduleSpec{Schedule: "0 2 * * *", Paused: false},
	}

	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sched).Build()
	cl := &conflictInjectingClient{Client: base, conflictsLeft: 1} // exactly one conflict

	// apply mirrors PatchSchedule's intent: pause the schedule.
	apply := func(s *velerov1.Schedule, _ string) error {
		s.Spec.Paused = true
		return nil
	}

	out, err := patchPolicyWithRetry(context.Background(), cl, name, apply)
	if err != nil {
		t.Fatalf("expected conflict to be retried and succeed, got error: %v", err)
	}

	// Proof the retry actually happened: Update was called >1 time.
	if cl.updateCalls < 2 {
		t.Fatalf("expected >=2 Update calls (1 conflict + 1 success), got %d — retry did not engage", cl.updateCalls)
	}

	// Returned aggregate reflects the mutation.
	if out == nil || out.SnapshotSchedule == nil || !out.SnapshotSchedule.Spec.Paused {
		t.Fatalf("returned policy not paused as expected: %+v", out)
	}

	// And it was actually persisted to the store.
	got := &velerov1.Schedule{}
	if err := base.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, got); err != nil {
		t.Fatalf("re-Get after patch: %v", err)
	}
	if !got.Spec.Paused {
		t.Fatalf("schedule was not persisted as paused; store still has Paused=%v", got.Spec.Paused)
	}
}

// TestPatchPolicyWithRetry_NonConflictErrorNotRetried guards the other side:
// a non-conflict error (e.g. a validation failure from apply) must propagate
// immediately, not get retried or masked.
func TestPatchPolicyWithRetry_NonConflictErrorNotRetried(t *testing.T) {
	const name = "policy-validation"

	scheme := runtime.NewScheme()
	if err := velerov1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	sched := &velerov1.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "velero",
			Labels:    map[string]string{labelPolicyName: name, labelPolicyRole: roleSnapshot},
		},
		Spec: velerov1.ScheduleSpec{Schedule: "0 2 * * *"},
	}
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sched).Build()

	sentinel := fmt.Errorf("invalid input")
	calls := 0
	apply := func(s *velerov1.Schedule, _ string) error {
		calls++
		return sentinel
	}

	_, err := patchPolicyWithRetry(context.Background(), base, name, apply)
	if err == nil {
		t.Fatal("expected the validation error to propagate, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected apply called exactly once (no retry on non-conflict), got %d", calls)
	}
}
