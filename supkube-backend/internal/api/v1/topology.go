// Package v1: topology.go — DR Topology aggregation for Dashboard (v0.8.12.5).
//
// One endpoint, one self-contained payload describing the entire backup
// topology: which clusters back up which namespaces to which BSLs, and
// a 3-2-1-1-0 compliance score for the whole picture.
//
// Why one endpoint (vs the SPA stitching it from 4 existing endpoints):
//
//   - Render-on-paint. The Dashboard topology card is above the fold; we
//     want one round trip, not four.
//   - Single point of truth for the 3-2-1-1-0 score. If the SPA computed
//     it from raw data, every page that wants to show the score would
//     re-implement the rules; here it's centralized.
//   - Future multi-cluster (v0.9.0) only changes this file: SPA stays the
//     same. The Cluster Manager will list remote clusters, this endpoint
//     will aggregate them in.
//
// What's NOT in this endpoint:
//   - Per-RP details, per-backup-run timestamps — those belong to the
//     Backups / Restore Points pages.
//   - Recommendation text — Advisor lives on its own page.
//
// RBAC: viewer-or-above (every Dashboard render needs it).
package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

)

// ──────────────────────────────────────────────────────────────────────
// Payload shapes — SPA renders directly from these. Keep flat + JSON-clean.
// ──────────────────────────────────────────────────────────────────────

type TopologyResponse struct {
	Clusters   []TopologyCluster   `json:"clusters"`
	BSLs       []TopologyBSL       `json:"bsls"`
	Flows      []TopologyFlow      `json:"flows"`
	Score      TopologyScore       `json:"score"`
	Summary    TopologySummary     `json:"summary"`
}

type TopologyCluster struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`           // "primary" | "secondary"
	IsCurrent      bool     `json:"isCurrent"`
	NamespaceNames []string `json:"namespaceNames"` // ns covered by at least one policy
	PolicyCount    int      `json:"policyCount"`
	// v0.8.12.5 fixes B: dashboard cluster card density
	K8sVersion string `json:"k8sVersion,omitempty"`
	NodeCount  int    `json:"nodeCount,omitempty"`
}

type TopologyBSL struct {
	Name              string `json:"name"`
	Kind              string `json:"kind"` // "local" | "cloud"
	Provider          string `json:"provider"`
	Bucket            string `json:"bucket,omitempty"`
	Phase             string `json:"phase"` // Available / Unavailable / ...
	RPCount           int    `json:"rpCount"`
	BackedupNs        int    `json:"backedupNs"`        // distinct ns count writing here
	ObjectLockEnabled bool   `json:"objectLockEnabled"`
	ObjectLockMode    string `json:"objectLockMode,omitempty"`
	// v0.8.12.5 fix C: capacity bar. Local BSL exposes the PVC's bound
	// capacity. Cloud BSLs leave these zero (we'd need each provider's
	// SDK to query bucket usage, scope creep for v1).
	CapacityBytes int64 `json:"capacityBytes,omitempty"`
	UsedBytes     int64 `json:"usedBytes,omitempty"`
	// v0.8.12.5 fix E: last backup landed here. RFC3339; empty = never.
	LastBackupAt string `json:"lastBackupAt,omitempty"`
}

type TopologyFlow struct {
	// FromCluster + FromNs identify the source. BSL identifies the
	// destination. Each row in the SVG becomes one line (or one ribbon).
	FromCluster   string   `json:"fromCluster"`
	FromNamespace string   `json:"fromNamespace"`
	ToBSL         string   `json:"toBSL"`
	PolicyNames   []string `json:"policyNames"`
	BackupCount   int      `json:"backupCount"` // total RPs on this exact path
}

// TopologyScore is the 3-2-1-1-0 evaluation. Each rule is one bool plus
// a human-readable hint that the SPA shows in a tooltip when red.
type TopologyScore struct {
	ThreeCopies   bool   `json:"threeCopies"`
	ThreeNote     string `json:"threeNote,omitempty"`
	TwoMedia      bool   `json:"twoMedia"`
	TwoNote       string `json:"twoNote,omitempty"`
	OneOffsite    bool   `json:"oneOffsite"`
	OneNote       string `json:"oneNote,omitempty"`
	OneImmutable  bool   `json:"oneImmutable"`
	ImmutableNote string `json:"immutableNote,omitempty"`
	ZeroErrors    bool   `json:"zeroErrors"`
	ZeroNote      string `json:"zeroNote,omitempty"`
}

type TopologySummary struct {
	ClusterCount    int `json:"clusterCount"`
	PolicyCount     int `json:"policyCount"`
	RPCount         int `json:"rpCount"`
	NamespaceCount  int `json:"namespaceCount"`
	LocalBSLCount   int `json:"localBSLCount"`
	CloudBSLCount   int `json:"cloudBSLCount"`
}

// ──────────────────────────────────────────────────────────────────────
// Handler
// ──────────────────────────────────────────────────────────────────────

// GetTopology is GET /api/v1/dashboard/topology.
//
// Internally:
//   1. List BSLs (Velero CRDs in the velero namespace)
//   2. List Schedules (Velero CRDs) and Backups (for per-flow RP count)
//   3. Single-cluster v1: synthesise one Cluster from cluster identity
//      env vars / fallback to "this-cluster". v0.9.0 will replace this
//      with the supkube.io/Cluster CR aggregator.
//   4. Walk Schedules to populate Flows + per-ns coverage
//   5. Walk Backups to populate per-flow + per-BSL counts
//   6. Evaluate the 3-2-1-1-0 score from aggregated state
func GetTopology(c *gin.Context) {
	// v0.9.0.1 fix #3: header-routed clients so the Dashboard topology
	// reflects the cluster the Mode Switcher currently points at.
	cl, err := getRequestRuntimeClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "client: " + err.Error()})
		return
	}
	k8sCli, err := getRequestKubernetesClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "k8s: " + err.Error()})
		return
	}
	dynCli, err := getRequestDynamicClient(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "dynamic: " + err.Error()})
		return
	}
	ctx := context.Background()
	veleroNS := "velero"
	supkubeNS := "supkube"

	resp := TopologyResponse{
		Clusters: []TopologyCluster{},
		BSLs:     []TopologyBSL{},
		Flows:    []TopologyFlow{},
	}

	// ───── BSLs ────────────────────────────────────────────────────
	bslList := &velerov1.BackupStorageLocationList{}
	if err := cl.List(ctx, bslList, client.InNamespace(veleroNS)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list bsl: " + err.Error()})
		return
	}
	for i := range bslList.Items {
		b := &bslList.Items[i]
		t := TopologyBSL{
			Name:     b.Name,
			Provider: b.Spec.Provider,
			Phase:    string(b.Status.Phase),
		}
		if b.Spec.ObjectStorage != nil {
			t.Bucket = b.Spec.ObjectStorage.Bucket
		}
		// Local vs cloud: derived from our own labels (set by Helm chart
		// for supkube-managed BSL, blank for user-added ones).
		if b.Labels["supkube.io/bsl-role"] == "local" {
			t.Kind = "local"
		} else {
			t.Kind = "cloud"
		}
		if b.Annotations != nil {
			t.ObjectLockEnabled = b.Annotations["supkube.io/object-lock"] == "true"
			t.ObjectLockMode = b.Annotations["supkube.io/object-lock-mode"]
		}
		resp.BSLs = append(resp.BSLs, t)
	}

	// ───── Schedules → Flows + Namespaces ──────────────────────────
	scheduleList := &velerov1.ScheduleList{}
	if err := cl.List(ctx, scheduleList, client.InNamespace(veleroNS)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list schedules: " + err.Error()})
		return
	}

	// flow key = cluster|ns|bsl, value = mutable aggregator
	type flowKey struct{ Cluster, NS, BSL string }
	flows := map[flowKey]*TopologyFlow{}
	nsSet := map[string]struct{}{}
	policyNames := map[string]struct{}{}

	clusterID := "this-cluster"
	for i := range scheduleList.Items {
		s := &scheduleList.Items[i]
		policyName := s.Labels["supkube.io/policy-name"]
		if policyName == "" {
			policyName = s.Name
		}
		policyNames[policyName] = struct{}{}

		// Where will Backups from this Schedule land?
		bslName := s.Spec.Template.StorageLocation
		if bslName == "" {
			// Schedule didn't pin a BSL → Velero uses default. Find it.
			for _, b := range bslList.Items {
				if b.Spec.Default {
					bslName = b.Name
					break
				}
			}
		}
		if bslName == "" {
			// No default BSL either — orphan schedule, skip from topology
			// (still shows up in Policies page; just absent here).
			continue
		}

		// Which namespaces? Empty IncludedNamespaces means "all", which
		// we represent as the literal "*" for the SPA to render specially.
		nss := s.Spec.Template.IncludedNamespaces
		if len(nss) == 0 {
			nss = []string{"*"}
		}
		for _, ns := range nss {
			nsSet[ns] = struct{}{}
			k := flowKey{clusterID, ns, bslName}
			if _, ok := flows[k]; !ok {
				flows[k] = &TopologyFlow{
					FromCluster:   clusterID,
					FromNamespace: ns,
					ToBSL:         bslName,
				}
			}
			f := flows[k]
			// Dedupe policy names per flow (a single policy can spawn 2
			// Schedules — snapshot half + export half — both pointing
			// here; we don't want to double-count it).
			seen := false
			for _, p := range f.PolicyNames {
				if p == policyName {
					seen = true
					break
				}
			}
			if !seen {
				f.PolicyNames = append(f.PolicyNames, policyName)
			}
		}
	}

	// ───── Backups → RP counts + lastBackupAt on flows + BSLs ────
	backupList := &velerov1.BackupList{}
	if err := cl.List(ctx, backupList, client.InNamespace(veleroNS)); err == nil {
		// Per-BSL: RP count, ns set, latest CreationTime
		bslRPCount := map[string]int{}
		bslNs := map[string]map[string]struct{}{}
		bslLastBackup := map[string]time.Time{}
		for i := range backupList.Items {
			b := &backupList.Items[i]
			bslName := b.Spec.StorageLocation
			if bslName == "" {
				continue
			}
			bslRPCount[bslName]++
			if bslNs[bslName] == nil {
				bslNs[bslName] = map[string]struct{}{}
			}
			// Track most recent CreationTimestamp per BSL — drives the
			// "last backup at X min ago" chip in the UI (fix E).
			if ts := b.CreationTimestamp.Time; ts.After(bslLastBackup[bslName]) {
				bslLastBackup[bslName] = ts
			}
			for _, ns := range b.Spec.IncludedNamespaces {
				bslNs[bslName][ns] = struct{}{}
				k := flowKey{clusterID, ns, bslName}
				if f, ok := flows[k]; ok {
					f.BackupCount++
				}
			}
			if len(b.Spec.IncludedNamespaces) == 0 {
				k := flowKey{clusterID, "*", bslName}
				if f, ok := flows[k]; ok {
					f.BackupCount++
				}
			}
		}
		for i := range resp.BSLs {
			n := resp.BSLs[i].Name
			resp.BSLs[i].RPCount = bslRPCount[n]
			resp.BSLs[i].BackedupNs = len(bslNs[n])
			if t, ok := bslLastBackup[n]; ok && !t.IsZero() {
				resp.BSLs[i].LastBackupAt = t.Format(time.RFC3339)
			}
		}
	}

	// ───── Local BSL capacity (fix C) ─────────────────────────────
	// Only supkube-managed local MinIO has known capacity; we read the
	// PVC the Helm chart created. Cloud BSLs would need each provider
	// SDK to query bucket usage — scope creep for v0.8.12.5.
	if pvc, err := k8sCli.CoreV1().PersistentVolumeClaims(supkubeNS).Get(ctx, "supkube-local-store-data", metav1.GetOptions{}); err == nil {
		var cap int64
		if c, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			cap = c.Value()
		} else if r, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			cap = r.Value()
		}
		for i := range resp.BSLs {
			if resp.BSLs[i].Kind == "local" {
				resp.BSLs[i].CapacityBytes = cap
				// UsedBytes would need MinIO admin API (mc admin info);
				// LBS3 (Object Lock UI) is the right sprint to add that.
				// Leave 0 for now — frontend hides the bar when 0.
			}
		}
	}

	// Materialize flows map → slice
	for _, f := range flows {
		resp.Flows = append(resp.Flows, *f)
	}

	// ───── Clusters (single-cluster v1) ───────────────────────────
	// v0.9.0 MC will replace this with iteration over clusters.supkube.io.
	// For now: one entry representing "this cluster", enriched with
	// k8s server version (Discovery API — free) + node count.
	thisCluster := TopologyCluster{
		ID:             clusterID,
		Name:           detectClusterName(ctx, dynCli),
		Type:           "primary",
		IsCurrent:      true,
		PolicyCount:    len(policyNames),
		NamespaceNames: sortedKeys(nsSet),
	}
	if ver, err := k8sCli.Discovery().ServerVersion(); err == nil && ver != nil {
		// Display GitVersion (e.g. "v1.32.0"); strip leading "v" for
		// visual parity with how kubectl version shows it in UIs.
		thisCluster.K8sVersion = ver.GitVersion
	}
	if nodes, err := k8sCli.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		thisCluster.NodeCount = len(nodes.Items)
	}
	resp.Clusters = []TopologyCluster{thisCluster}

	// ───── Summary + Score ─────────────────────────────────────────
	resp.Summary = TopologySummary{
		ClusterCount:   len(resp.Clusters),
		PolicyCount:    len(policyNames),
		NamespaceCount: len(nsSet),
	}
	for _, b := range resp.BSLs {
		resp.Summary.RPCount += b.RPCount
		if b.Kind == "local" {
			resp.Summary.LocalBSLCount++
		} else {
			resp.Summary.CloudBSLCount++
		}
	}
	resp.Score = computeTopologyScore(resp.BSLs, resp.Flows, backupList)

	c.JSON(http.StatusOK, resp)
}

// computeTopologyScore evaluates the 3-2-1-1-0 rule against current state.
//
//	3 copies      = at least 1 namespace has flows to ≥2 distinct BSLs
//	                (the live PVC counts as the 3rd copy implicitly)
//	2 media       = at least 1 local BSL AND at least 1 cloud BSL exist
//	1 off-site    = at least 1 cloud BSL is Available (off-site = not in-cluster)
//	1 immutable   = at least 1 BSL has Object Lock enabled
//	0 errors      = no backup is in PartiallyFailed / Failed phase in last 7 days
//
// Rules are intentionally optimistic — a green check means "you satisfy
// this in at least one path", not "every path satisfies this". The Advisor
// (v0.9) will surface per-path drilldown; the Dashboard score is the
// summary signal.
func computeTopologyScore(bsls []TopologyBSL, flows []TopologyFlow, backups *velerov1.BackupList) TopologyScore {
	s := TopologyScore{}

	// 3 copies: any ns with ≥2 BSL destinations?
	nsBSL := map[string]map[string]struct{}{}
	for _, f := range flows {
		if nsBSL[f.FromNamespace] == nil {
			nsBSL[f.FromNamespace] = map[string]struct{}{}
		}
		nsBSL[f.FromNamespace][f.ToBSL] = struct{}{}
	}
	for _, m := range nsBSL {
		if len(m) >= 2 {
			s.ThreeCopies = true
			break
		}
	}
	if !s.ThreeCopies {
		s.ThreeNote = "No namespace is backed up to ≥2 destinations. Add a Local + Cloud policy for at least one namespace to satisfy 3-copy rule."
	}

	// 2 media: have both local + cloud?
	hasLocal, hasCloud := false, false
	for _, b := range bsls {
		if b.Kind == "local" {
			hasLocal = true
		} else {
			hasCloud = true
		}
	}
	s.TwoMedia = hasLocal && hasCloud
	if !s.TwoMedia {
		switch {
		case !hasLocal:
			s.TwoNote = "Enable Local Backup Store (Settings → Storage Locations) to add the second medium."
		case !hasCloud:
			s.TwoNote = "Add at least one Cloud Storage Profile (Storage Locations → Add) to add off-site medium."
		}
	}

	// 1 off-site: any cloud BSL Available?
	for _, b := range bsls {
		if b.Kind == "cloud" && b.Phase == "Available" {
			s.OneOffsite = true
			break
		}
	}
	if !s.OneOffsite {
		s.OneNote = "No cloud BSL is Available. Verify your Storage Profile credentials and reachability."
	}

	// 1 immutable: any BSL with Object Lock?
	for _, b := range bsls {
		if b.ObjectLockEnabled {
			s.OneImmutable = true
			break
		}
	}
	if !s.OneImmutable {
		s.ImmutableNote = "No BSL has Object Lock enabled. Local Backup Store enables it by default; for Cloud BSLs configure Object Lock at the bucket level (S3, Azure Immutable Blob)."
	}

	// 0 errors: any Failed / PartiallyFailed in the **last 7 days**?
	//
	// v0.8.12.6 fix 1: was "any historical failure" — meant a single
	// 6-month-old failure permanently flagged the dashboard 4/5. Now
	// we use a rolling 7-day window matching standard SRE habit (and
	// matching Velero's default schedule TTL of 30d retention still
	// catches most failures; 7d is the operationally-meaningful window).
	if backups != nil {
		cutoff := time.Now().Add(-7 * 24 * time.Hour)
		bad := 0
		for i := range backups.Items {
			b := &backups.Items[i]
			if b.CreationTimestamp.Time.Before(cutoff) {
				continue
			}
			p := b.Status.Phase
			if p == velerov1.BackupPhaseFailed || p == velerov1.BackupPhasePartiallyFailed {
				bad++
			}
		}
		if bad == 0 {
			s.ZeroErrors = true
		} else {
			s.ZeroNote = "There are failed/partial backups in the last 7 days. Review Activity page; fix root cause."
		}
	}

	return s
}

// detectClusterName uses the cluster's `kube-system/kube-system` namespace
// UID as a stable cluster identity; falls back to a literal if anything
// fails. The UI displays this string verbatim, so prefer a human label
// when one is configured (future: read from supkube-settings ConfigMap).
func detectClusterName(ctx context.Context, dyn interface{}) string {
	// v0.8.12.5 v1: hardcode. v0.9.0 Cluster Manager will replace this
	// with the user-provided cluster display name. We avoid reading
	// kube-system UID here because that's a 32-char hex string, ugly
	// in the UI.
	return "this-cluster"
}

// sortedKeys returns map keys deterministically sorted. We use a tiny
// implementation rather than pulling slices.Sort because the Go version
// in CI is 1.25 and we want zero new transitive deps for this file.
func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple insertion sort — N is tiny (typical < 30 ns).
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

// _gvr_velero_bsl kept as a documentation anchor — we don't currently
// need dynamic GVR access for this file (typed client suffices), but
// having the literal here makes the future "topology spans clusters
// over kubeconfig" change easy to grep for.
var _gvr_velero_bsl = schema.GroupVersionResource{
	Group:    "velero.io",
	Version:  "v1",
	Resource: "backupstoragelocations",
}

// Compile-time assertion: keep apierrors import referenced so the
// file still compiles when we add 404-tolerant Get calls in v0.9.
var _ = apierrors.IsNotFound
