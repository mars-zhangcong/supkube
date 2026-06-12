package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/supkube/supkube-backend/internal/k8s"
	"github.com/supkube/supkube-backend/internal/velerons"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VeleroResultSection struct {
	Title   string   `json:"title"`
	Items   []string `json:"items,omitempty"`
	Warning string   `json:"warning,omitempty"`
}

type VeleroRestoreResults struct {
	Sections []VeleroResultSection `json:"sections,omitempty"`
	Raw      string                `json:"raw,omitempty"`
}

func fetchBackupDetailedResults(ctx context.Context, backupName string) (*VeleroRestoreResults, error) {
	cli, err := k8s.GetDynamicClient()
	if err != nil {
		return nil, err
	}
	gvr := velerov1.SchemeGroupVersion.WithResource("backups")
	obj, err := cli.Resource(gvr).Namespace(velerons.Namespace()).Get(ctx, backupName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return parseVeleroResults(obj.Object)
}

func fetchRestoreDetailedResults(ctx context.Context, restoreName string) (*VeleroRestoreResults, error) {
	cli, err := k8s.GetDynamicClient()
	if err != nil {
		return nil, err
	}
	gvr := velerov1.SchemeGroupVersion.WithResource("restores")
	obj, err := cli.Resource(gvr).Namespace(velerons.Namespace()).Get(ctx, restoreName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return parseVeleroResults(obj.Object)
}

func parseVeleroResults(obj map[string]interface{}) (*VeleroRestoreResults, error) {
	res := &VeleroRestoreResults{}

	status, _ := obj["status"].(map[string]interface{})
	if status == nil {
		return res, nil
	}

	if raw, ok := status["warnings"]; ok {
		appendSectionFromUnknown(res, "Warnings", raw)
	}
	if raw, ok := status["errors"]; ok {
		appendSectionFromUnknown(res, "Errors", raw)
	}
	if raw, ok := status["validationErrors"]; ok {
		appendSectionFromUnknown(res, "Validation Errors", raw)
	}
	if raw, ok := status["warningsSummary"]; ok {
		appendSectionFromUnknown(res, "Warnings Summary", raw)
	}
	if raw, ok := status["errorsSummary"]; ok {
		appendSectionFromUnknown(res, "Errors Summary", raw)
	}
	if raw, ok := status["results"]; ok {
		appendSectionFromUnknown(res, "Results", raw)
	}

	if len(res.Sections) == 0 {
		if b, err := json.Marshal(status); err == nil {
			res.Raw = string(b)
		}
	}

	return res, nil
}

func appendSectionFromUnknown(res *VeleroRestoreResults, title string, raw interface{}) {
	items := flattenUnknown(raw)
	if len(items) == 0 {
		return
	}
	res.Sections = append(res.Sections, VeleroResultSection{
		Title: title,
		Items: items,
	})
}

func flattenUnknown(v interface{}) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return []string{t}
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, flattenUnknown(item)...)
		}
		return out
	case map[string]interface{}:
		out := make([]string, 0, len(t))
		for k, val := range t {
			flattened := flattenUnknown(val)
			if len(flattened) == 0 {
				continue
			}
			for _, s := range flattened {
				if strings.TrimSpace(k) != "" {
					out = append(out, fmt.Sprintf("%s: %s", k, s))
				} else {
					out = append(out, s)
				}
			}
		}
		return out
	default:
		return []string{fmt.Sprint(t)}
	}
}
