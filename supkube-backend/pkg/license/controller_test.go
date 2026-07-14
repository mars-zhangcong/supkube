package license

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func fakeCli(objs ...runtime.Object) *fake.Clientset { return fake.NewSimpleClientset(objs...) }

func licSecret(y string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
		Data:       map[string][]byte{"license": []byte(y)},
	}
}

func workerNode(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
}
func cpNode(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""}}}
}
func taintedNode(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: corev1.NodeSpec{
		Taints: []corev1.Taint{{Key: "dedicated", Effect: corev1.TaintEffectNoSchedule}},
	}}
}

func runCheck(objs ...runtime.Object) Status {
	c := &controller{k8sCli: fakeCli(objs...)} // recorder/target nil → emitEvents no-ops
	c.check(context.Background())
	return Snapshot()
}

func TestCheck_MissingSecret(t *testing.T) {
	s := runCheck(workerNode("w1"))
	if s.State != StateMissing {
		t.Fatalf("no Secret → state=%s, want missing", s.State)
	}
	if Allowed() {
		t.Error("missing license must block writes")
	}
	if s.NodeCount != 1 {
		t.Errorf("node count still counted while missing: got %d", s.NodeCount)
	}
}

func TestCheck_ValidGolden(t *testing.T) {
	s := runCheck(licSecret(goldenLicense), workerNode("w1"), workerNode("w2"))
	if s.State != StateLicensed {
		t.Fatalf("valid unexpired license → state=%s, want licensed", s.State)
	}
	if !Allowed() {
		t.Error("licensed must allow writes")
	}
	if s.NodeCount != 2 || s.License == nil || s.License.Restrictions.Nodes != 3 {
		t.Errorf("got nodes=%d license=%+v", s.NodeCount, s.License)
	}
	if len(s.Violations) != 0 {
		t.Errorf("2 nodes ≤ 3 max, no violation expected: %v", s.Violations)
	}
}

func TestCheck_TamperedSecret(t *testing.T) {
	tampered := strings.Replace(goldenLicense, "nodes: 3", "nodes: 99", 1)
	s := runCheck(licSecret(tampered), workerNode("w1"))
	if s.State != StateInvalid {
		t.Fatalf("tampered license → state=%s, want invalid", s.State)
	}
	if Allowed() {
		t.Error("invalid license must block writes")
	}
}

func TestCheck_NodeCountExceeded(t *testing.T) {
	// golden allows 3 nodes; give it 4 workers → warn-only violation, still licensed.
	s := runCheck(licSecret(goldenLicense), workerNode("w1"), workerNode("w2"), workerNode("w3"), workerNode("w4"))
	if s.State != StateLicensed {
		t.Errorf("node overage is warn-only, state should stay licensed: %s", s.State)
	}
	if !contains(s.Violations, "NodeCountExceeded") {
		t.Fatalf("4 > 3 must flag NodeCountExceeded, got %v", s.Violations)
	}
}

func TestCheck_NodeExclusion(t *testing.T) {
	// 1 worker (billable) + 1 control-plane + 1 tainted infra (both excluded).
	s := runCheck(licSecret(goldenLicense), workerNode("w1"), cpNode("cp1"), taintedNode("infra1"))
	if s.NodeCount != 1 {
		t.Errorf("billable = %d, want 1 (cp + tainted excluded)", s.NodeCount)
	}
	if s.NodeExcluded != 2 {
		t.Errorf("excluded = %d, want 2", s.NodeExcluded)
	}
}

func TestClassifyExpiry(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		dateEnd time.Time
		want    string
	}{
		{"not expired", now.Add(48 * time.Hour), StateLicensed},
		{"expired within grace (3d)", now.Add(-3 * 24 * time.Hour), StateGrace},
		{"expired at grace edge (7d)", now.Add(-7 * 24 * time.Hour), StateGrace},
		{"expired past grace (8d)", now.Add(-8 * 24 * time.Hour), StateDegraded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyExpiry(&License{DateEnd: tc.dateEnd}, now)
			if got != tc.want {
				t.Errorf("dateEnd %v → %s, want %s", tc.dateEnd, got, tc.want)
			}
		})
	}
}
