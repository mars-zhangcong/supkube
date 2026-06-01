// Package csi auto-configures Kubernetes CSI snapshotting so a SupKube
// install "just works" on the major cloud + edge platforms without the
// user having to hand-write a VolumeSnapshotClass.
//
// # Why this exists (task #84, the customer-pain version)
//
// Velero's CSI snapshot path requires a VolumeSnapshotClass (VSC) that:
//   - matches the CSI driver of the PVC being backed up, and
//   - is tagged with label `velero.io/csi-volumesnapshot-class=true`
//     so Velero picks it as the default for that driver.
//
// Out of the box, K8s does NOT ship VSCs — most distros expect either the
// user or a Helm chart to install them. Result on a fresh SupKube
// install: the user creates a Backup, Velero captures metadata fine, but
// the PV data is silently skipped because no VSC matched. The Backup
// status says "Completed" while the restore would lose every byte of
// stateful data. This was the #1 demo-blocker complaint pre-v0.9.
//
// This controller fixes that by walking the installed CSI drivers at
// startup (and again periodically) and creating a Velero-tagged
// VolumeSnapshotClass for any well-known driver that doesn't already
// have one. Unknown drivers are skipped — we won't guess parameters
// blindly. A status field is exposed via GET /storage/csi-autoconfig
// so the UI can show "X drivers auto-configured, Y need attention".
//
// # What it does NOT do
//
//   - Does NOT touch existing VSCs (idempotent: if any VSC for that
//     driver exists, regardless of label, we leave it alone).
//   - Does NOT enable CSI snapshotting on the driver itself
//     (that's the CSI driver vendor's install path).
//   - Does NOT cover non-CSI provisioners (e.g. NFS, in-tree drivers).
//
// # Known-driver table maintenance
//
// When you add a new driver to `knownDrivers`, double-check the
// `deletionPolicy` and `parameters` against the driver's docs. Wrong
// parameters fail loudly at snapshot time, not at VSC create time.
// The default is `Delete` to avoid the orphan-cloud-snapshot billing
// trap (see D-11 in dashboard). Customers who want `Retain` for
// long-term archive should patch the VSC after the fact — we don't
// override their choice on subsequent reconcile loops.
package csi

import (
	"context"
	"log"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// veleroDefaultLabel is the label Velero v1.10+ uses to discover which
// VSC to use for a given CSI driver. Without this label Velero falls
// back to "the only VSC for this driver" (works if there's one) or
// errors out (if there are zero or multiple).
const veleroDefaultLabel = "velero.io/csi-volumesnapshot-class"

// VolumeSnapshotClass GVR — we use unstructured to avoid pulling the
// external-snapshotter Go module just for one Create. The schema is
// stable v1 since k8s 1.20.
var vscGVR = schema.GroupVersionResource{
	Group:    "snapshot.storage.k8s.io",
	Version:  "v1",
	Resource: "volumesnapshotclasses",
}

// KnownDriver describes a CSI driver SupKube can auto-configure a VSC
// for. Add new entries here when you've actually tested the resulting
// VSC produces a working snapshot — don't add speculatively, a broken
// VSC silently corrupts backups.
type KnownDriver struct {
	// CSIDriver.Name as it appears in `kubectl get csidriver` (and
	// equivalently in StorageClass.provisioner).
	DriverName string
	// Human label used in /storage/csi-autoconfig response.
	Label string
	// Parameters passed verbatim into VSC.parameters. Often empty,
	// but some drivers (Azure Disk for incremental snapshots) need
	// specific keys.
	Parameters map[string]string
}

// knownDrivers is the curated list of CSI drivers SupKube knows how
// to configure. Order is for human readability only.
var knownDrivers = []KnownDriver{
	{
		DriverName: "disk.csi.azure.com",
		Label:      "Azure Disk CSI",
		// incremental=true is Azure's cost-saver: each snapshot stores
		// only the delta against the previous one. Doc:
		// https://learn.microsoft.com/azure/virtual-machines/disks-incremental-snapshots
		Parameters: map[string]string{"incremental": "true"},
	},
	{
		DriverName: "ebs.csi.aws.com",
		Label:      "AWS EBS CSI",
		// EBS snapshots are always incremental at the AWS layer; no
		// parameter needed. Doc: https://docs.aws.amazon.com/ebs/latest/userguide/ebs-creating-snapshot.html
		Parameters: nil,
	},
	{
		DriverName: "pd.csi.storage.gke.io",
		Label:      "GCP Persistent Disk CSI",
		Parameters: nil,
	},
	{
		DriverName: "diskplugin.csi.alibabacloud.com",
		Label:      "Alibaba Cloud Disk CSI",
		Parameters: nil,
	},
	{
		DriverName: "com.tencent.cloud.csi.cbs",
		Label:      "Tencent Cloud CBS CSI",
		Parameters: nil,
	},
	{
		DriverName: "hostpath.csi.k8s.io",
		Label:      "CSI Hostpath (dev/test)",
		Parameters: nil,
	},
}

// Status is what the controller exposes for the UI / debugging.
type Status struct {
	LastReconcileAt  time.Time   `json:"lastReconcileAt"`
	ReconcileError   string      `json:"reconcileError,omitempty"`
	InstalledDrivers []string    `json:"installedDrivers"`
	ManagedVSCs      []VSCStatus `json:"managedVSCs"`
	UnknownDrivers   []string    `json:"unknownDrivers"`
}

// VSCStatus is one entry in Status.ManagedVSCs.
type VSCStatus struct {
	DriverName string `json:"driverName"`
	Label      string `json:"label"`
	VSCName    string `json:"vscName"`
	Created    bool   `json:"created"`           // true if THIS reconcile created it
	Skipped    string `json:"skipped,omitempty"` // reason if we didn't touch it
}

var (
	statusMu     sync.RWMutex
	latestStatus Status
)

// GetStatus returns the latest reconcile status (thread-safe).
// The Status handler in api/v1 reads this directly.
func GetStatus() Status {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return latestStatus
}

func setStatus(s Status) {
	statusMu.Lock()
	latestStatus = s
	statusMu.Unlock()
}

// Run starts the CSI auto-config reconciler. It runs once immediately
// then every interval. Cancel ctx to stop.
//
// Failure to create a VSC is logged + recorded in Status but does NOT
// stop the loop — partial success (e.g. AWS works, Azure fails) is
// better than zero success on a multi-driver cluster.
func Run(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface, interval time.Duration) {
	log.Printf("[csi-autoconfig] starting (interval=%s, %d known drivers)", interval, len(knownDrivers))
	// First pass immediately so a freshly-installed SupKube reaches the
	// "VSCs ready" state in seconds, not minutes.
	reconcile(ctx, k8sCli, dynCli)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[csi-autoconfig] stopped: %v", ctx.Err())
			return
		case <-ticker.C:
			reconcile(ctx, k8sCli, dynCli)
		}
	}
}

// reconcile is one pass. Exported nowhere — call Run instead.
func reconcile(ctx context.Context, k8sCli kubernetes.Interface, dynCli dynamic.Interface) {
	st := Status{LastReconcileAt: time.Now().UTC()}

	// Step 1: enumerate installed CSI drivers on this cluster.
	csiList, err := k8sCli.StorageV1().CSIDrivers().List(ctx, metav1.ListOptions{})
	if err != nil {
		st.ReconcileError = "list CSIDrivers failed: " + err.Error()
		setStatus(st)
		log.Printf("[csi-autoconfig] %s", st.ReconcileError)
		return
	}
	installed := map[string]bool{}
	for _, d := range csiList.Items {
		installed[d.Name] = true
		st.InstalledDrivers = append(st.InstalledDrivers, d.Name)
	}

	// Step 2: enumerate existing VSCs so we can be idempotent.
	// VSC CRD may not be installed (cluster without external-snapshotter)
	// — that's a hard pre-req for CSI snapshotting, log and bail
	// gracefully without flagging as error.
	vscList, err := dynCli.Resource(vscGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || meta_IsNoMatchError(err) {
			st.ReconcileError = "VolumeSnapshotClass CRD not installed on this cluster — install external-snapshotter before SupKube can do CSI snapshots"
			setStatus(st)
			log.Printf("[csi-autoconfig] %s", st.ReconcileError)
			return
		}
		st.ReconcileError = "list VolumeSnapshotClasses failed: " + err.Error()
		setStatus(st)
		log.Printf("[csi-autoconfig] %s", st.ReconcileError)
		return
	}
	// Build set: driver → list of existing VSC names.
	existingByDriver := map[string][]string{}
	for _, item := range vscList.Items {
		driver, found, _ := unstructured.NestedString(item.Object, "driver")
		if !found {
			continue
		}
		existingByDriver[driver] = append(existingByDriver[driver], item.GetName())
	}

	// Step 3: walk known drivers; for each that's installed, ensure VSC.
	knownSet := map[string]bool{}
	for _, kd := range knownDrivers {
		knownSet[kd.DriverName] = true
		if !installed[kd.DriverName] {
			// Driver isn't on this cluster — skip.
			continue
		}
		vscStatus := VSCStatus{DriverName: kd.DriverName, Label: kd.Label}
		if existing, has := existingByDriver[kd.DriverName]; has && len(existing) > 0 {
			// Be conservative: existing VSC means an operator already
			// configured this. Don't override their choice. We do log
			// it so they can SEE we noticed.
			vscStatus.VSCName = existing[0]
			vscStatus.Skipped = "VSC already exists; not overridden"
			st.ManagedVSCs = append(st.ManagedVSCs, vscStatus)
			continue
		}
		// Create the VSC.
		vscName := "supkube-" + sanitizeDriverName(kd.DriverName)
		obj := buildVSC(vscName, kd)
		_, cerr := dynCli.Resource(vscGVR).Create(ctx, obj, metav1.CreateOptions{})
		if cerr != nil {
			if apierrors.IsAlreadyExists(cerr) {
				vscStatus.VSCName = vscName
				vscStatus.Skipped = "VSC raced into existence; left as-is"
			} else {
				vscStatus.Skipped = "create failed: " + cerr.Error()
				log.Printf("[csi-autoconfig] create VSC for %s failed: %v", kd.DriverName, cerr)
			}
		} else {
			vscStatus.VSCName = vscName
			vscStatus.Created = true
			log.Printf("[csi-autoconfig] created VSC %s for driver %s", vscName, kd.DriverName)
		}
		st.ManagedVSCs = append(st.ManagedVSCs, vscStatus)
	}

	// Step 4: record unknown drivers (installed but not in our table).
	// UI surfaces these as "consider opening an issue / PR to add support".
	for d := range installed {
		if !knownSet[d] {
			st.UnknownDrivers = append(st.UnknownDrivers, d)
		}
	}

	setStatus(st)
}

// buildVSC returns an unstructured VolumeSnapshotClass ready to Create.
// Cluster-scoped so no namespace field is set.
func buildVSC(name string, kd KnownDriver) *unstructured.Unstructured {
	params := map[string]interface{}{}
	for k, v := range kd.Parameters {
		params[k] = v
	}
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "snapshot.storage.k8s.io/v1",
			"kind":       "VolumeSnapshotClass",
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					veleroDefaultLabel:             "true",
					"app.kubernetes.io/managed-by": "supkube",
					"supkube.io/csi-autoconfig":    "true",
				},
				"annotations": map[string]interface{}{
					"supkube.io/note": "Auto-created by SupKube to make Velero CSI snapshots work out of the box. Safe to delete if you replace with your own VSC.",
				},
			},
			// Delete (not Retain) is intentional: the D-11 cleanup rule
			// in 测试用例.md §0.4 documents how Retain produces orphan
			// cloud snapshots → silent billing. Customers who need long-
			// term archive should patch this field manually and we'll
			// honor it on subsequent reconciles.
			"driver":         kd.DriverName,
			"deletionPolicy": "Delete",
		},
	}
	if len(params) > 0 {
		obj.Object["parameters"] = params
	}
	return obj
}

// sanitizeDriverName turns "disk.csi.azure.com" into "disk-csi-azure-com"
// — valid as a K8s resource name (DNS-1123 lowercase, no dots).
func sanitizeDriverName(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c+32)
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}

// meta_IsNoMatchError is a tiny shim: we want to detect "CRD doesn't
// exist on this cluster" without importing meta just for this. The
// dynamic client wraps the discovery NoMatch error in different ways
// depending on apimachinery version, so string-match the message as a
// belt-and-suspenders fallback to apierrors.IsNotFound.
func meta_IsNoMatchError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "no matches for kind") ||
		contains(msg, "the server could not find the requested resource")
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
