package v1

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// CreateStorageLocationWithSecret creates a BSL and its associated credential secret
func CreateStorageLocationWithSecret(c *gin.Context) {
	var req struct {
		Name      string            `json:"name" binding:"required"`
		Provider  string            `json:"provider" binding:"required"`
		Bucket    string            `json:"bucket" binding:"required"`
		Region    string            `json:"region"`
		Endpoint  string            `json:"endpoint"`
		AccessKey string            `json:"accessKey"`
		SecretKey string            `json:"secretKey"`
		Config    map[string]string `json:"config"`
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

	// Create K8S Secret for S3 credentials if provided
	secretName := ""
	if req.AccessKey != "" && req.SecretKey != "" {
		secretName = fmt.Sprintf("supkube-bsl-%s-credentials", req.Name)

		// Build the credentials file content (AWS format)
		credentialData := fmt.Sprintf("[default]\naws_access_key_id=%s\naws_secret_access_key=%s\n", req.AccessKey, req.SecretKey)

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
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"cloud": []byte(credentialData),
			},
		}

		_, err = k8sClient.CoreV1().Secrets("velero").Create(context.Background(), secret, metav1.CreateOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create credentials secret: " + err.Error()})
			return
		}
	}

	// Build config map
	config := req.Config
	if config == nil {
		config = make(map[string]string)
	}
	if req.Region != "" {
		config["region"] = req.Region
	}
	if req.Endpoint != "" {
		config["s3Url"] = req.Endpoint
		config["s3ForcePathStyle"] = "true"
	}

	// Create BSL
	bsl := &velerov1.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: "velero",
		},
		Spec: velerov1.BackupStorageLocationSpec{
			Provider: req.Provider,
			StorageType: velerov1.StorageType{
				ObjectStorage: &velerov1.ObjectStorageLocation{
					Bucket: req.Bucket,
				},
			},
			Config: config,
		},
	}

	// Set credential reference if secret was created
	if secretName != "" {
		bsl.Spec.Credential = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secretName,
			},
			Key: "cloud",
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

// UpdateStorageLocation updates a BSL's mutable fields. The metadata.name is
// immutable (K8s API rejects it anyway); accessKey/secretKey are optional —
// if both provided, the linked credentials Secret is rotated in place.
// On any spec change, we clear status.lastValidationTime so Velero re-validates
// on its next reconcile.
func UpdateStorageLocation(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Provider         string            `json:"provider" binding:"required"`
		Bucket           string            `json:"bucket" binding:"required"`
		Region           string            `json:"region"`
		Endpoint         string            `json:"endpoint"`
		S3ForcePathStyle bool              `json:"s3ForcePathStyle"`
		AccessKey        string            `json:"accessKey"`
		SecretKey        string            `json:"secretKey"`
		Config           map[string]string `json:"config"`
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

	// Rotate credentials secret if a new pair was provided. If the BSL has no
	// linked secret yet, create one with the standard supkube naming.
	if req.AccessKey != "" && req.SecretKey != "" {
		k8sClient, err := k8s.GetClient()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		secretName := fmt.Sprintf("supkube-bsl-%s-credentials", name)
		if bsl.Spec.Credential != nil && bsl.Spec.Credential.Name != "" {
			secretName = bsl.Spec.Credential.Name
		}
		credentialData := fmt.Sprintf("[default]\naws_access_key_id=%s\naws_secret_access_key=%s\n", req.AccessKey, req.SecretKey)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: "velero",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "supkube",
					"supkube.io/bsl-name":          name,
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"cloud": []byte(credentialData)},
		}
		// Upsert: try create, fall back to update if it already exists.
		if _, err := k8sClient.CoreV1().Secrets("velero").Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
			if _, err2 := k8sClient.CoreV1().Secrets("velero").Update(context.Background(), secret, metav1.UpdateOptions{}); err2 != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update credentials secret: " + err2.Error()})
				return
			}
		}
		bsl.Spec.Credential = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  "cloud",
		}
	}

	// Rebuild spec fields. Config merging strategy: replace, not patch.
	config := req.Config
	if config == nil {
		config = make(map[string]string)
	}
	if req.Region != "" {
		config["region"] = req.Region
	}
	if req.Endpoint != "" {
		config["s3Url"] = req.Endpoint
	}
	if req.S3ForcePathStyle {
		config["s3ForcePathStyle"] = "true"
	} else {
		delete(config, "s3ForcePathStyle")
	}

	bsl.Spec.Provider = req.Provider
	bsl.Spec.ObjectStorage = &velerov1.ObjectStorageLocation{Bucket: req.Bucket}
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
