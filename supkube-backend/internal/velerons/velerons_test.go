package velerons

import "testing"

// resolve is what actually reads the env; Namespace() caches resolve() once
// at package load (correct for prod, where k8s sets the env before start),
// so we test resolve() directly to exercise the override path.
func TestResolve(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"unset falls back to default", "", Default},
		{"explicit collapsed single-ns", "supkube", "supkube"},
		{"explicit conventional velero ns", "velero", "velero"},
		{"whitespace is trimmed", "  supkube  ", "supkube"},
		{"blank env falls back", "   ", Default},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VELERO_NAMESPACE", tc.env)
			if got := resolve(); got != tc.want {
				t.Fatalf("resolve() with VELERO_NAMESPACE=%q = %q, want %q", tc.env, got, tc.want)
			}
		})
	}
}

func TestDefaultIsVelero(t *testing.T) {
	if Default != "velero" {
		t.Fatalf("Default = %q, want velero (backward-compat for dev/standalone)", Default)
	}
}
