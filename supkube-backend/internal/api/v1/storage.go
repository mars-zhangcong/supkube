package v1

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigyaml "sigs.k8s.io/yaml"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// rfc1123SubdomainPattern matches the K8s DNS-1123 subdomain spec, which is
// what metadata.name must satisfy for Secrets, BackupStorageLocations, etc.
// Lowercase, alphanumeric or '-'/'.', must start and end with alphanumeric.
var rfc1123SubdomainPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

func validateK8sName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > 253 {
		return fmt.Errorf("name too long (max 253 characters)")
	}
	if !rfc1123SubdomainPattern.MatchString(name) {
		return fmt.Errorf("invalid name %q: must be lowercase alphanumeric or '-'/'.', and start/end with alphanumeric (e.g. my-s3-storage)", name)
	}
	return nil
}

// BackupResourceInfo represents a resource item in a backup
type BackupResourceInfo struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// GetBackupResources returns the list of resources included in a backup
func GetBackupResources(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// First verify the backup exists
	backup := &velerov1.Backup{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, backup); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Return backup spec info as resource preview
	// In a real implementation, this would query the backup contents from object storage
	// For MVP, we return the backup's included namespaces, resources, and label selector info
	preview := gin.H{
		"backupName":         backup.Name,
		"phase":              string(backup.Status.Phase),
		"includedNamespaces": backup.Spec.IncludedNamespaces,
		"excludedNamespaces": backup.Spec.ExcludedNamespaces,
		"includedResources":  backup.Spec.IncludedResources,
		"excludedResources":  backup.Spec.ExcludedResources,
		"labelSelector":      backup.Spec.LabelSelector,
		"storageLocation":    backup.Spec.StorageLocation,
	}

	if backup.Status.Progress != nil {
		preview["totalItems"] = backup.Status.Progress.TotalItems
		preview["itemsBackedUp"] = backup.Status.Progress.ItemsBackedUp
	}

	c.JSON(http.StatusOK, preview)
}

// restoreArtifactGVRs enumerates the common namespace-scoped resource types
// Velero typically captures. Used by ListBackupArtifacts to render the
// "Spec (N)" list in the Restore drawer.
//
// Why hard-coded vs dynamic discovery: Velero's actual restore set comes from
// the backup tarball on object storage, which would require BSL credentials +
// S3 client + tar/gzip parsing — too heavy for the v0.7.10 drawer MVP. The
// live-cluster preview is accurate enough for the in-place restore flow
// (the user just took the backup, so what's in the cluster ≈ what's in the
// backup). v0.8 will swap this for BackupStore.GetBackupContents().
var restoreArtifactGVRs = []schema.GroupVersionResource{
	{Group: "", Version: "v1", Resource: "configmaps"},
	{Group: "", Version: "v1", Resource: "secrets"},
	{Group: "", Version: "v1", Resource: "services"},
	{Group: "", Version: "v1", Resource: "serviceaccounts"},
	{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	{Group: "apps", Version: "v1", Resource: "deployments"},
	{Group: "apps", Version: "v1", Resource: "statefulsets"},
	{Group: "apps", Version: "v1", Resource: "daemonsets"},
	{Group: "batch", Version: "v1", Resource: "jobs"},
	{Group: "batch", Version: "v1", Resource: "cronjobs"},
	{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
}

// excludedArtifactNames is a small denylist of objects the user almost never
// wants to see in a restore preview because they're auto-managed by K8s
// (cluster-injected, regenerated on namespace create, etc).
var excludedArtifactNames = map[string]map[string]bool{
	"configmaps": {"kube-root-ca.crt": true},
	"secrets":    {}, // auto-generated SA tokens already gone in K8s 1.24+
}

// ListBackupArtifacts returns the resources that would be restored from this
// backup. Each item carries its GVK, namespace, name, and full YAML so the
// drawer can render the expand-to-view-YAML interaction without a second
// round-trip per artifact.
//
// Query strategy:
//  1. Resolve the backup's source namespaces (spec.includedNamespaces).
//  2. For each namespace × each well-known GVR, list with the backup's
//     labelSelector applied (so partial backups don't oversell).
//  3. Strip server-only fields (resourceVersion, uid, managedFields, status)
//     so the YAML is a clean "what gets recreated" view.
func ListBackupArtifacts(c *gin.Context) {
	name := c.Param("name")
	rcl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	backup := &velerov1.Backup{}
	if err := rcl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, backup); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	namespaces := backup.Spec.IncludedNamespaces
	if len(namespaces) == 0 {
		c.JSON(http.StatusOK, gin.H{"items": []any{}, "warning": "backup has no includedNamespaces (whole-cluster backup); artifact preview is only available for namespace-scoped backups in v0.7.10"})
		return
	}

	dcl, err := k8s.GetDynamicClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	listOpts := metav1.ListOptions{}
	if backup.Spec.LabelSelector != nil {
		listOpts.LabelSelector = metav1.FormatLabelSelector(backup.Spec.LabelSelector)
	}

	type artifact struct {
		Group     string `json:"group"`
		Version   string `json:"version"`
		Kind      string `json:"kind"`
		Resource  string `json:"resource"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		YAML      string `json:"yaml"`
	}
	items := make([]artifact, 0, 32)

	// Resource → Kind mapping (Kind isn't on GVR; we hand-fill the common ones
	// so the UI can display "deployments" with the friendlier label.)
	kindOf := map[string]string{
		"configmaps":             "ConfigMap",
		"secrets":                "Secret",
		"services":               "Service",
		"serviceaccounts":        "ServiceAccount",
		"persistentvolumeclaims": "PersistentVolumeClaim",
		"deployments":            "Deployment",
		"statefulsets":           "StatefulSet",
		"daemonsets":             "DaemonSet",
		"jobs":                   "Job",
		"cronjobs":               "CronJob",
		"ingresses":              "Ingress",
		"networkpolicies":        "NetworkPolicy",
		"roles":                  "Role",
		"rolebindings":           "RoleBinding",
	}

	for _, ns := range namespaces {
		for _, gvr := range restoreArtifactGVRs {
			list, err := dcl.Resource(gvr).Namespace(ns).List(context.Background(), listOpts)
			if err != nil {
				if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
					continue // CRD or RBAC absent → skip silently
				}
				continue
			}
			denyForResource := excludedArtifactNames[gvr.Resource]
			for _, obj := range list.Items {
				if denyForResource != nil && denyForResource[obj.GetName()] {
					continue
				}
				// Skip Helm-internal Secrets and SA tokens.
				if gvr.Resource == "secrets" {
					t, _, _ := unstructuredString(obj.Object, "type")
					if t == "helm.sh/release.v1" || t == "kubernetes.io/service-account-token" {
						continue
					}
				}
				cleaned := stripServerFields(obj.Object)
				y, err := sigyaml.Marshal(cleaned)
				if err != nil {
					continue
				}
				items = append(items, artifact{
					Group:     gvr.Group,
					Version:   gvr.Version,
					Kind:      kindOf[gvr.Resource],
					Resource:  gvr.Resource,
					Namespace: ns,
					Name:      obj.GetName(),
					YAML:      string(y),
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// stripServerFields removes the server-populated metadata so the YAML in the
// drawer looks like "what gets recreated", not "what's currently in etcd".
func stripServerFields(in map[string]interface{}) map[string]interface{} {
	// Shallow copy of top-level keys; only metadata + status are mutated.
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	if md, ok := out["metadata"].(map[string]interface{}); ok {
		mdCopy := make(map[string]interface{}, len(md))
		for k, v := range md {
			switch k {
			case "resourceVersion", "uid", "generation", "managedFields",
				"creationTimestamp", "selfLink", "ownerReferences":
				continue
			}
			mdCopy[k] = v
		}
		// Drop kubectl.kubernetes.io/last-applied-configuration — huge and
		// already represented by the rest of the object.
		if ann, ok := mdCopy["annotations"].(map[string]interface{}); ok {
			delete(ann, "kubectl.kubernetes.io/last-applied-configuration")
			delete(ann, "deployment.kubernetes.io/revision")
			if len(ann) == 0 {
				delete(mdCopy, "annotations")
			}
		}
		out["metadata"] = mdCopy
	}
	delete(out, "status")
	return out
}

// unstructuredString reads a string field from an unstructured object map.
// Returned ok=false on missing or wrong-type.
func unstructuredString(obj map[string]interface{}, keys ...string) (string, bool, error) {
	var cur interface{} = obj
	for _, k := range keys {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return "", false, nil
		}
		cur, ok = m[k]
		if !ok {
			return "", false, nil
		}
	}
	s, ok := cur.(string)
	if !ok {
		return "", false, fmt.Errorf("field %s is not a string", strings.Join(keys, "."))
	}
	return s, true, nil
}

// VerifyStorageLocation checks if a backup storage location is accessible
func VerifyStorageLocation(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get the BSL
	bsl := &velerov1.BackupStorageLocation{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, bsl); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Check BSL phase/status
	status := "Unknown"
	if bsl.Status.Phase == velerov1.BackupStorageLocationPhaseAvailable {
		status = "Available"
	} else if bsl.Status.Phase == velerov1.BackupStorageLocationPhaseUnavailable {
		status = "Unavailable"
	}

	c.JSON(http.StatusOK, gin.H{
		"name":     bsl.Name,
		"provider": bsl.Spec.Provider,
		"bucket":   bsl.Spec.ObjectStorage.Bucket,
		"status":   status,
		"phase":    string(bsl.Status.Phase),
	})
}

// CreateStorageLocationWithSecret creates a BSL and its associated
// credential Secret. Two providers supported (v0.8.7):
//
//	provider: "aws"   — S3 / MinIO / Tencent COS / Aliyun OSS / any S3-API
//	provider: "azure" — Azure Blob Storage (storage account key auth)
//
// The request shape uses ONE struct with optional fields for each
// provider; the dispatch happens inside on req.Provider. We deliberately
// don't split into two endpoints — the BSL CRUD lifecycle (validate name,
// create Secret, create BSL, label both for cleanup) is identical, only
// the credential payload format and the BSL.spec.config keys differ.
//
// The created credentials Secret is always named
// `supkube-bsl-<name>-credentials` with key `cloud` — same convention
// Velero's own `velero install --provider azure ...` uses, so the
// resulting BSL works interchangeably with `velero backup` CLI.
func CreateStorageLocationWithSecret(c *gin.Context) {
	var req struct {
		Name     string            `json:"name" binding:"required"`
		Provider string            `json:"provider" binding:"required"` // "aws" | "azure"
		Bucket   string            `json:"bucket" binding:"required"`   // S3 bucket or Azure container
		Config   map[string]string `json:"config"`

		// ── AWS / S3-compatible fields ─────────────────────────────────
		Region    string `json:"region,omitempty"`
		Endpoint  string `json:"endpoint,omitempty"`
		AccessKey string `json:"accessKey,omitempty"`
		SecretKey string `json:"secretKey,omitempty"`

		// ── Azure fields ──────────────────────────────────────────────
		// Required: storageAccount, container (= Bucket field), and
		// either storageAccountKey (simple) OR full AAD service-principal
		// quartet (resourceGroup + subscriptionId + tenantId + clientId
		// + clientSecret). MVP supports the simple path only.
		StorageAccount    string `json:"storageAccount,omitempty"`
		StorageAccountKey string `json:"storageAccountKey,omitempty"`
		// ResourceGroup is OPTIONAL for storage-account-key auth (the
		// path v0.8.7 supports). It's only required when running in
		// AAD/MSI mode where the plugin needs to call ARM API to
		// resolve the storage account's keys. v0.8.7.1: don't reject
		// when missing — first user feedback flagged this as confusing.
		ResourceGroup  string `json:"resourceGroup,omitempty"`
		SubscriptionID string `json:"subscriptionId,omitempty"`

		// v0.8.7.1: optional object storage prefix. Lets Velero share
		// a bucket/container with another backup tool (Kasten K10's
		// k10/ prefix is the common case) by namespacing every Velero
		// object under <bucket>/<prefix>/.... Applied to BSL.spec.
		// objectStorage.prefix.
		Prefix string `json:"prefix,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// BSL name becomes both the BSL metadata.name and the basis of the Secret
	// name (supkube-bsl-<name>-credentials). Both must satisfy RFC 1123 — fail
	// fast with a clear message rather than letting the K8s API reject it.
	if err := validateK8sName(req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	runtimeClient, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build provider-specific credential bytes + BSL config map.
	var credentialData []byte
	config := req.Config
	if config == nil {
		config = map[string]string{}
	}
	switch strings.ToLower(req.Provider) {
	case "aws":
		if req.AccessKey != "" && req.SecretKey != "" {
			credentialData = []byte(fmt.Sprintf("[default]\naws_access_key_id=%s\naws_secret_access_key=%s\n", req.AccessKey, req.SecretKey))
		}
		if req.Region != "" {
			config["region"] = req.Region
		}
		if req.Endpoint != "" {
			config["s3Url"] = req.Endpoint
			config["s3ForcePathStyle"] = "true"
		}
	case "azure":
		// Validate ONLY the truly-required Azure fields. v0.8.7.1
		// loosened this from "require RG" to "require StorageAccount +
		// StorageAccountKey" — RG is only needed for AAD/MSI mode (which
		// we don't support yet), and storage-account-key auth talks
		// directly to the Blob service without RG.
		if req.StorageAccount == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "storageAccount is required for provider=azure"})
			return
		}
		if req.StorageAccountKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "storageAccountKey is required (v0.8.7 supports storage-account-key auth only; AAD service-principal lands in v0.9)"})
			return
		}
		// ResourceGroup intentionally NOT required here — see comment
		// on the request struct. Passing it through to config when set
		// keeps the BSL forward-compatible with VolumeSnapshot use.
		// SubscriptionID also optional for the same reason.

		// Velero's Azure plugin reads credentials as plain key=value pairs
		// from the `cloud` key in the referenced Secret. The exact key
		// names below match velero-plugin-for-microsoft-azure v1.10's
		// expectations.
		var sb strings.Builder
		fmt.Fprintf(&sb, "AZURE_STORAGE_ACCOUNT_ACCESS_KEY=%s\n", req.StorageAccountKey)
		if req.SubscriptionID != "" {
			fmt.Fprintf(&sb, "AZURE_SUBSCRIPTION_ID=%s\n", req.SubscriptionID)
		}
		fmt.Fprintf(&sb, "AZURE_CLOUD_NAME=AzurePublicCloud\n")
		credentialData = []byte(sb.String())

		// Azure plugin reads these from BSL.spec.config (NOT credentials).
		// resourceGroup only emitted if user actually provided one —
		// the plugin's key-auth path doesn't read it, and leaving it
		// unset avoids fake authority for users who never had an RG.
		config["storageAccount"] = req.StorageAccount
		if req.ResourceGroup != "" {
			config["resourceGroup"] = req.ResourceGroup
		}
		if req.SubscriptionID != "" {
			config["subscriptionId"] = req.SubscriptionID
		}
		// Storage account auth mode hint — required when the cluster's
		// node identity DOESN'T have IAM access to the container (which
		// is the common case for cross-cluster DR).
		config["storageAccountKeyEnvVar"] = "AZURE_STORAGE_ACCOUNT_ACCESS_KEY"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported provider %q (v0.8.7 supports: aws, azure)", req.Provider)})
		return
	}

	// Create the credentials Secret (provider-shape already built above).
	secretName := ""
	if len(credentialData) > 0 {
		secretName = fmt.Sprintf("supkube-bsl-%s-credentials", req.Name)
		k8sClient, err := k8s.GetClient()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: "velero",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "supkube",
					"supkube.io/bsl-name":          req.Name,
					"supkube.io/bsl-provider":      strings.ToLower(req.Provider),
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"cloud": credentialData},
		}
		if _, err = k8sClient.CoreV1().Secrets("velero").Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create credentials secret: " + err.Error()})
			return
		}
	}

	// Create the BSL itself.
	objectStorage := &velerov1.ObjectStorageLocation{Bucket: req.Bucket}
	// v0.8.7.1: optional prefix lets Velero co-tenant a bucket with
	// another tool. Common case: the user's container already has a
	// `k10/` directory from Kasten K10 and Velero's BSL validator
	// refuses to share the container at top level.
	if req.Prefix != "" {
		objectStorage.Prefix = strings.Trim(req.Prefix, "/")
	}
	bsl := &velerov1.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: "velero",
		},
		Spec: velerov1.BackupStorageLocationSpec{
			Provider: req.Provider,
			StorageType: velerov1.StorageType{
				ObjectStorage: objectStorage,
			},
			Config: config,
		},
	}
	if secretName != "" {
		bsl.Spec.Credential = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  "cloud",
		}
	}
	if err := runtimeClient.Create(context.Background(), bsl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, bsl)
}

// GetStorageLocation returns a single BSL with derived UI fields. The linked
// credentials Secret is referenced (name only); secret data itself is never
// returned over the API.
func GetStorageLocation(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	bsl := &velerov1.BackupStorageLocation{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, bsl); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bsl)
}

// rotateBSLSecret writes new credential bytes into the BSL's linked Secret
// (creating one with the canonical supkube-bsl-<name>-credentials name if
// none exists yet). On success, sets bsl.Spec.Credential so the BSL
// reconciles to the (possibly new) Secret on the next Update.
//
// Mutation pattern is Upsert: try Create, fall back to Update — saves a
// pre-flight Get round-trip on the common rotate-existing path.
func rotateBSLSecret(name string, bsl *velerov1.BackupStorageLocation, credentialData []byte) error {
	k8sClient, err := k8s.GetClient()
	if err != nil {
		return err
	}
	secretName := fmt.Sprintf("supkube-bsl-%s-credentials", name)
	if bsl.Spec.Credential != nil && bsl.Spec.Credential.Name != "" {
		secretName = bsl.Spec.Credential.Name
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "velero",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "supkube",
				"supkube.io/bsl-name":          name,
				"supkube.io/bsl-provider":      strings.ToLower(bsl.Spec.Provider),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"cloud": credentialData},
	}
	if _, err := k8sClient.CoreV1().Secrets("velero").Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		if _, err2 := k8sClient.CoreV1().Secrets("velero").Update(context.Background(), secret, metav1.UpdateOptions{}); err2 != nil {
			return fmt.Errorf("failed to update credentials secret: %w", err2)
		}
	}
	bsl.Spec.Credential = &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
		Key:                  "cloud",
	}
	return nil
}

// UpdateStorageLocation updates a BSL's mutable fields (v0.8.7.2):
//
//	metadata.name           — IMMUTABLE (K8s API rejects)
//	spec.provider           — IMMUTABLE (different SDK / config keys)
//	spec.objectStorage.bucket   — EDITABLE — credentials bind to the
//	                              ACCOUNT (IAM identity for AWS, storage
//	                              account for Azure), NOT the bucket.
//	                              Kasten K10 allows this and we mirror.
//	spec.objectStorage.prefix   — editable
//	spec.config.storageAccount  — EDITABLE — but caller must also send
//	                              a new key on the same request, since
//	                              keys ARE tied to a specific SA. We
//	                              refuse the update if the SA changes
//	                              and no new key is supplied.
//	spec.config.* (others)      — editable, MERGED not replaced
//	credentials Secret          — editable (key rotation)
//
// Why merge config not replace: a UI sending only "prefix" should not
// wipe storageAccount/resourceGroup. Pre-v0.8.7.1 it did exactly that.
//
// v0.8.7.2 changelog vs .1: container/bucket is editable again (the
// .1 lockdown was over-conservative; Kasten allows the same edits and
// nothing's actually bound at the bucket level).
func UpdateStorageLocation(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		// Provider-agnostic identity / location (v0.8.7.2 editable).
		// Bucket maps to BSL.spec.objectStorage.bucket — for AWS this
		// is the S3 bucket, for Azure the Blob container. Sending an
		// empty value or omitting leaves the existing value untouched.
		Bucket string `json:"bucket,omitempty"`
		Prefix string `json:"prefix,omitempty"`

		// AWS / S3-compat
		Region           string `json:"region,omitempty"`
		Endpoint         string `json:"endpoint,omitempty"`
		S3ForcePathStyle *bool  `json:"s3ForcePathStyle,omitempty"`
		AccessKey        string `json:"accessKey,omitempty"`
		SecretKey        string `json:"secretKey,omitempty"`

		// Azure
		StorageAccount    string `json:"storageAccount,omitempty"`
		StorageAccountKey string `json:"storageAccountKey,omitempty"`
		ResourceGroup     string `json:"resourceGroup,omitempty"`
		SubscriptionID    string `json:"subscriptionId,omitempty"`

		// Free-form extra config keys (advanced; rarely set from UI).
		Config map[string]string `json:"config,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	runtimeClient, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	bsl := &velerov1.BackupStorageLocation{}
	if err := runtimeClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, bsl); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Read the BSL's actual provider — we trust the live CR, never the
	// request, because letting the client switch provider on an
	// existing BSL is meaningless (the bucket + credentials are
	// provider-specific). The Edit dialog also disables the provider
	// field so it never sends a divergent value.
	provider := strings.ToLower(bsl.Spec.Provider)

	// Start with the EXISTING config and mutate — never wholesale replace.
	if bsl.Spec.Config == nil {
		bsl.Spec.Config = map[string]string{}
	}
	config := bsl.Spec.Config

	// Apply caller-supplied extra keys first (allows admins to tweak
	// obscure Velero settings via UI without us listing every one).
	for k, v := range req.Config {
		config[k] = v
	}

	// Provider-specific config edits + credentials rotation.
	switch provider {
	case "aws":
		if req.Region != "" {
			config["region"] = req.Region
		}
		if req.Endpoint != "" {
			config["s3Url"] = req.Endpoint
		}
		if req.S3ForcePathStyle != nil {
			if *req.S3ForcePathStyle {
				config["s3ForcePathStyle"] = "true"
			} else {
				delete(config, "s3ForcePathStyle")
			}
		}
		if req.AccessKey != "" && req.SecretKey != "" {
			credentialData := fmt.Sprintf("[default]\naws_access_key_id=%s\naws_secret_access_key=%s\n", req.AccessKey, req.SecretKey)
			if err := rotateBSLSecret(name, bsl, []byte(credentialData)); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	case "azure":
		// Storage Account is editable in v0.8.7.2 — but switching it
		// requires a new key on the same request, since keys are
		// account-scoped. If the user changes the SA without sending
		// a key we 400 (a silent success would leave the BSL pointed
		// at the new SA but still using the old SA's key, failing
		// every blob operation).
		newSA := req.StorageAccount
		currentSA := config["storageAccount"]
		if newSA != "" && newSA != currentSA && req.StorageAccountKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "changing storage account also requires a new Storage Account Key (keys are account-scoped)",
			})
			return
		}
		if newSA != "" {
			config["storageAccount"] = newSA
		}
		if req.ResourceGroup != "" {
			config["resourceGroup"] = req.ResourceGroup
		}
		if req.SubscriptionID != "" {
			config["subscriptionId"] = req.SubscriptionID
		}
		if req.StorageAccountKey != "" {
			// Rebuild the cred file. After the SA edit above,
			// config["storageAccount"] is the new value (if changed).
			var sb strings.Builder
			fmt.Fprintf(&sb, "AZURE_STORAGE_ACCOUNT_ACCESS_KEY=%s\n", req.StorageAccountKey)
			if sub := config["subscriptionId"]; sub != "" {
				fmt.Fprintf(&sb, "AZURE_SUBSCRIPTION_ID=%s\n", sub)
			}
			fmt.Fprintf(&sb, "AZURE_CLOUD_NAME=AzurePublicCloud\n")
			if err := rotateBSLSecret(name, bsl, []byte(sb.String())); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}

	// Provider-agnostic: object storage bucket + prefix.
	if bsl.Spec.ObjectStorage == nil {
		bsl.Spec.ObjectStorage = &velerov1.ObjectStorageLocation{}
	}
	// Bucket: editable. Empty/omitted = keep existing. Sending empty
	// string explicitly to wipe a bucket is nonsensical (BSL would be
	// invalid anyway), so we treat "" as "no change".
	if req.Bucket != "" {
		bsl.Spec.ObjectStorage.Bucket = req.Bucket
	}
	// Prefix: editable. Empty string IS meaningful (= remove prefix).
	bsl.Spec.ObjectStorage.Prefix = strings.Trim(req.Prefix, "/")
	bsl.Spec.Config = config

	if err := runtimeClient.Update(context.Background(), bsl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Force re-validation on next reconcile by clearing lastValidationTime.
	// Best-effort: if status subresource patch fails, the BSL itself is still
	// updated and Velero will revalidate within its default interval.
	bsl.Status.LastValidationTime = nil
	_ = runtimeClient.Status().Update(context.Background(), bsl)

	c.JSON(http.StatusOK, bsl)
}

// DeleteStorageLocation deletes a BSL and cascades the linked credentials
// secret if it was created by supkube. Secrets without the supkube managed-by
// label are left alone so we don't clobber user-managed credentials shared
// across multiple BSLs.
func DeleteStorageLocation(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	bsl := &velerov1.BackupStorageLocation{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, bsl); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Cascade delete the credentials secret only if we own it.
	if bsl.Spec.Credential != nil && bsl.Spec.Credential.Name != "" {
		k8sClient, err := k8s.GetClient()
		if err == nil {
			secret, getErr := k8sClient.CoreV1().Secrets("velero").Get(context.Background(), bsl.Spec.Credential.Name, metav1.GetOptions{})
			if getErr == nil && secret.Labels["app.kubernetes.io/managed-by"] == "supkube" {
				_ = k8sClient.CoreV1().Secrets("velero").Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
			}
		}
	}

	if err := cl.Delete(context.Background(), bsl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "storage location deleted"})
}

// --- VolumeSnapshotLocations (v0.6) -------------------------------------
//
// VSL points Velero at how to take CSI/native volume snapshots. For
// hostpath-csi the spec is minimal (just the provider name = csi); for
// cloud providers it carries region, profile, etc. v0.6 MVP exposes
// list/get/create/delete; UI builds a similar Kasten-style table.

// ListVolumeSnapshotLocations returns all Velero VolumeSnapshotLocations.
func ListVolumeSnapshotLocations(c *gin.Context) {
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	list := &velerov1.VolumeSnapshotLocationList{}
	if err := cl.List(context.Background(), list, client.InNamespace("velero")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list.Items, "total": len(list.Items)})
}

// GetVolumeSnapshotLocation returns a single VSL.
func GetVolumeSnapshotLocation(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	vsl := &velerov1.VolumeSnapshotLocation{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, vsl); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, vsl)
}

// CreateVolumeSnapshotLocation creates a Velero VolumeSnapshotLocation.
// Provider naming follows Velero convention: "csi" for any CSI driver
// (Velero v1.14+ resolves the actual driver via VolumeSnapshotClass);
// cloud-native providers (aws/gcp/azure) take provider-specific config.
func CreateVolumeSnapshotLocation(c *gin.Context) {
	var req struct {
		Name     string            `json:"name" binding:"required"`
		Provider string            `json:"provider" binding:"required"`
		Config   map[string]string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateK8sName(req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	vsl := &velerov1.VolumeSnapshotLocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: "velero",
		},
		Spec: velerov1.VolumeSnapshotLocationSpec{
			Provider: req.Provider,
			Config:   req.Config,
		},
	}
	if err := cl.Create(context.Background(), vsl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, vsl)
}

// DeleteVolumeSnapshotLocation deletes a Velero VolumeSnapshotLocation.
// No cascade — VSLs have no associated secrets in our model. Velero
// will return an error if the VSL is still referenced by any Backup.
func DeleteVolumeSnapshotLocation(c *gin.Context) {
	name := c.Param("name")
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	vsl := &velerov1.VolumeSnapshotLocation{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: name, Namespace: "velero"}, vsl); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err := cl.Delete(context.Background(), vsl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "volume snapshot location deleted"})
}
