package drflow

import (
	"context"
	"fmt"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// triggerRestoringDB creates a KB Restore CR in DrillNS to restore the database
// from the named backup into a new KB Cluster (ADR-052 D5, method A).
//
// Safety invariants (ADR-053 D5):
//   - All objects are created in DrillNS only.
//   - Every created object is labeled with labelRunID=r.ID so cleanup can
//     identify and guard against deleting non-drflow resources.
//   - Idempotent: if the Restore CR already exists, this is a no-op.
//   - DryRun=true skips the Create entirely for gate-only testing.
func triggerRestoringDB(ctx context.Context, dynCli dynamic.Interface, r Run) error {
	if r.DryRun {
		log.Printf("[drflow] %s: dry_run — skipping RestoringDB trigger", r.ID)
		return nil
	}

	restoreName := "drflow-restore-" + r.ID[:8]
	backupNS := r.BackupNamespace
	if backupNS == "" {
		backupNS = r.DrillNS
	}

	// Idempotent: check if Restore CR already exists
	_, err := dynCli.Resource(kbRestoreGVR).Namespace(r.DrillNS).Get(ctx, restoreName, metav1.GetOptions{})
	if err == nil {
		log.Printf("[drflow] %s: Restore %s/%s already exists, skipping create", r.ID, r.DrillNS, restoreName)
		return nil
	}

	restore := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "dataprotection.kubeblocks.io/v1alpha1",
		"kind":       "Restore",
		"metadata": map[string]interface{}{
			"name":      restoreName,
			"namespace": r.DrillNS,
			"labels": map[string]interface{}{
				labelRunID:     r.ID,
				labelComponent: "drflow",
				labelActivity:  "true",
			},
		},
		"spec": map[string]interface{}{
			"backup": map[string]interface{}{
				"name":      r.BackupName,
				"namespace": backupNS,
			},
			"prepareDataConfig": map[string]interface{}{
				"volumeClaimRestorePolicy": "Parallel",
				"restoreVolumeClaimsTemplate": map[string]interface{}{
					"storageClassName": "managed-csi",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"storage": "20Gi",
						},
					},
				},
			},
		},
	}}

	_, err = dynCli.Resource(kbRestoreGVR).Namespace(r.DrillNS).Create(ctx, restore, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create KB Restore %s/%s: %w", r.DrillNS, restoreName, err)
	}
	log.Printf("[drflow] %s: created KB Restore %s/%s from backup %s/%s", r.ID, r.DrillNS, restoreName, backupNS, r.BackupName)
	return nil
}

// triggerRestoringApp deploys Adminer in DrillNS, pointing at the restored KB Cluster
// (ADR-052 D3/D4 route A: App reads credentials from KB-generated Secret).
//
// Safety invariants (ADR-053 D5):
//   - Deployment and Service are created in DrillNS only.
//   - Labeled with labelRunID=r.ID for cleanup guarding.
//   - Idempotent: if the Deployment already exists, this is a no-op.
//   - DryRun=true skips the Create entirely for gate-only testing.
func triggerRestoringApp(ctx context.Context, k8sCli kubernetes.Interface, r Run) error {
	if r.DryRun {
		log.Printf("[drflow] %s: dry_run — skipping RestoringApp trigger", r.ID)
		return nil
	}

	appName := "adminer-" + r.ID[:8]

	// Idempotent: check if Deployment already exists
	_, err := k8sCli.AppsV1().Deployments(r.DrillNS).Get(ctx, appName, metav1.GetOptions{})
	if err == nil {
		log.Printf("[drflow] %s: Deployment %s/%s already exists, skipping create", r.ID, r.DrillNS, appName)
		return nil
	}

	// ADR-052 D3: AKS-internal svc DNS, no Tailscale needed for intra-cluster traffic
	dbHost := r.KBCluster + "-postgresql." + r.DrillNS + ".svc.cluster.local"

	labels := map[string]string{
		"app":      "adminer",
		labelRunID: r.ID,
	}
	one := int32(1)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: r.DrillNS,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "adminer",
						Image: "adminer:4.8.1",
						Ports: []corev1.ContainerPort{{ContainerPort: 8080}},
						Env: []corev1.EnvVar{
							{Name: "ADMINER_DEFAULT_SERVER", Value: dbHost},
							// ADR-052 D4 Route A: credentials from KB-generated Secret
							{
								Name: "ADMINER_DEFAULT_USERNAME",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: r.DBSecret},
										Key:                  "username",
									},
								},
							},
							{
								Name: "ADMINER_DEFAULT_PASSWORD",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: r.DBSecret},
										Key:                  "password",
									},
								},
							},
							{Name: "ADMINER_DEFAULT_DB", Value: "demo"},
						},
					}},
				},
			},
		},
	}
	if _, err := k8sCli.AppsV1().Deployments(r.DrillNS).Create(ctx, dep, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create Adminer Deployment %s/%s: %w", r.DrillNS, appName, err)
	}

	// ClusterIP Service so the App is accessible within the cluster
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: r.DrillNS,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    []corev1.ServicePort{{Port: 8080}},
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
	if _, err := k8sCli.CoreV1().Services(r.DrillNS).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
		// Service creation failure is non-fatal — App pod will still come up for gate testing
		log.Printf("[drflow] %s: warning: create Adminer Service %s/%s: %v", r.ID, r.DrillNS, appName, err)
	}

	log.Printf("[drflow] %s: deployed Adminer %s/%s → DB %s", r.ID, r.DrillNS, appName, dbHost)
	return nil
}
