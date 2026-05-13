package v1

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
)

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
