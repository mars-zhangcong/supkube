package drflow

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// emitPhaseEvent writes a K8s Event for a phase transition. The
// supkube.io/activity=true label makes it appear on the Activity page
// without any additional API changes (ADR-053 D5).
func emitPhaseEvent(ctx context.Context, k8sCli kubernetes.Interface, r Run, note string) {
	t := metav1.NewMicroTime(time.Now().UTC())
	evtType := corev1.EventTypeNormal
	reason := fmt.Sprintf("DRFlow%s", string(r.Phase))
	if r.Phase == PhaseFailed {
		evtType = corev1.EventTypeWarning
		reason = "DRFlowFailed"
	}

	evt := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "drflow-",
			Namespace:    Namespace,
			Labels: map[string]string{
				labelActivity:  "true", // ← Activity page picks this up automatically
				labelComponent: "drflow",
				labelRunID:     r.ID,
				labelPhase:     string(r.Phase),
			},
		},
		EventTime: t,
		Type:      evtType,
		Reason:    reason,
		Note:      note,
		Regarding: corev1.ObjectReference{
			Kind:      "ConfigMap",
			Namespace: Namespace,
			Name:      cmName(r.ID),
		},
		ReportingController: reportingCtrl,
		ReportingInstance:   reportingInst,
		Action:              "PhaseTransition",
	}
	if _, err := k8sCli.EventsV1().Events(Namespace).Create(ctx, evt, metav1.CreateOptions{}); err != nil {
		log.Printf("[drflow] %s: failed to emit event for phase %s: %v", r.ID, r.Phase, err)
	}
}
