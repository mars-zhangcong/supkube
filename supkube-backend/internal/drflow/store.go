package drflow

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// createRun persists a new Run to a ConfigMap and returns the stored run.
func createRun(ctx context.Context, k8sCli kubernetes.Interface, r Run) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName(r.ID),
			Namespace: Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "supkube",
				labelComponent:                 "drflow",
				labelRunID:                     r.ID,
				labelPhase:                     string(r.Phase),
			},
		},
		Data: map[string]string{
			"run": string(data),
		},
	}
	_, err = k8sCli.CoreV1().ConfigMaps(Namespace).Create(ctx, cm, metav1.CreateOptions{})
	return err
}

// updateRun writes updated run state back to the ConfigMap.
func updateRun(ctx context.Context, k8sCli kubernetes.Interface, r Run) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	cm, err := k8sCli.CoreV1().ConfigMaps(Namespace).Get(ctx, cmName(r.ID), metav1.GetOptions{})
	if err != nil {
		return err
	}
	cm.Data["run"] = string(data)
	cm.Labels[labelPhase] = string(r.Phase)
	_, err = k8sCli.CoreV1().ConfigMaps(Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

// GetRun fetches a single run by ID.
func GetRun(ctx context.Context, k8sCli kubernetes.Interface, id string) (*Run, error) {
	cm, err := k8sCli.CoreV1().ConfigMaps(Namespace).Get(ctx, cmName(id), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseRunCM(cm)
}

// ListRuns returns all DRFlow runs, sorted newest-first.
func ListRuns(ctx context.Context, k8sCli kubernetes.Interface) ([]*Run, error) {
	list, err := k8sCli.CoreV1().ConfigMaps(Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelComponent + "=drflow",
	})
	if err != nil {
		return nil, err
	}
	runs := make([]*Run, 0, len(list.Items))
	for i := range list.Items {
		r, err := parseRunCM(&list.Items[i])
		if err != nil {
			continue
		}
		runs = append(runs, r)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	return runs, nil
}

func parseRunCM(cm *corev1.ConfigMap) (*Run, error) {
	raw, ok := cm.Data["run"]
	if !ok {
		return nil, nil
	}
	var r Run
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// generateRunID produces a time-ordered run ID: "drflow-<yyyymmddhhmmss>".
func generateRunID() string {
	return "drflow-" + strings.ReplaceAll(
		time.Now().UTC().Format("20060102150405"), "", "")
}
