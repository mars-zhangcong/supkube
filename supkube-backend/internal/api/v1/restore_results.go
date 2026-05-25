// Package v1: restore_results.go
//
// Fetch the per-namespace error/warning detail for a Velero Restore by going
// through Velero's DownloadRequest CRD flow. Used by GetRestoreResults to
// turn "Errors (1)" into actionable messages the user can paste into a bug.
package v1

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/supkube/supkube-backend/internal/k8s"
)

// VeleroResultSection mirrors Velero's internal `results.Result` struct. Each
// section partitions messages by where they originated during the restore.
//
//	Velero     — scheduler / engine messages
//	Cluster    — cluster-scoped resource issues
//	Namespaces — keyed by ns, the per-resource warnings/errors
type VeleroResultSection struct {
	Velero     []string            `json:"velero,omitempty"`
	Cluster    []string            `json:"cluster,omitempty"`
	Namespaces map[string][]string `json:"namespaces,omitempty"`
}

// VeleroRestoreResults is the JSON shape Velero uploads to BSL at the path
// `restores/<name>/restore-<name>-results.gz`. Two top-level sections.
type VeleroRestoreResults struct {
	Errors   VeleroResultSection `json:"errors"`
	Warnings VeleroResultSection `json:"warnings"`
}

// fetchRestoreDetailedResults performs the DownloadRequest dance and returns
// the parsed results. The DownloadRequest is best-effort cleaned up on exit
// so we don't accumulate stale CRs over time.
//
// Why insecureSkipVerify on the HTTP client: in-cluster BSLs (MinIO, MAYbe
// self-signed S3 gateways) typically use HTTP or self-signed TLS. The
// presigned URL Velero returns is signed against the BSL endpoint, not a
// public certificate authority. A future hardening pass (v0.8) will resolve
// the BSL's `caCert` and trust it explicitly; for now the URL is short-lived
// (~10 min) and only reachable from within the cluster.
func fetchRestoreDetailedResults(ctx context.Context, restoreName string) (*VeleroRestoreResults, error) {
	cl, err := k8s.GetRuntimeClient()
	if err != nil {
		return nil, fmt.Errorf("k8s client: %w", err)
	}

	// Per-request unique name (DownloadRequest names must be DNS-1123).
	// Truncate restoreName so the prefix + timestamp fits in 253 chars.
	prefix := restoreName
	if len(prefix) > 200 {
		prefix = prefix[:200]
	}
	drName := fmt.Sprintf("%s-supkube-%d", prefix, time.Now().UnixNano())

	dr := &velerov1.DownloadRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      drName,
			Namespace: "velero",
			Labels: map[string]string{
				"supkube.io/managed-by": "supkube",
			},
		},
		Spec: velerov1.DownloadRequestSpec{
			Target: velerov1.DownloadTarget{
				Kind: velerov1.DownloadTargetKindRestoreResults,
				Name: restoreName,
			},
		},
	}
	if err := cl.Create(ctx, dr); err != nil {
		return nil, fmt.Errorf("create DownloadRequest: %w", err)
	}
	// Best-effort cleanup; if we leak one, the velero gc-controller will
	// expire it after ~10 minutes.
	defer func() {
		_ = cl.Delete(context.Background(), &velerov1.DownloadRequest{
			ObjectMeta: metav1.ObjectMeta{Name: drName, Namespace: "velero"},
		})
	}()

	// Poll for downloadURL. Velero usually fills this within 1-2s.
	deadline := time.Now().Add(20 * time.Second)
	var downloadURL string
	for time.Now().Before(deadline) {
		cur := &velerov1.DownloadRequest{}
		if err := cl.Get(ctx, client.ObjectKey{Name: drName, Namespace: "velero"}, cur); err != nil {
			if apierrors.IsNotFound(err) {
				// shouldn't happen this fast — wait & retry
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("get DownloadRequest: %w", err)
		}
		if cur.Status.DownloadURL != "" {
			downloadURL = cur.Status.DownloadURL
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if downloadURL == "" {
		return nil, fmt.Errorf("Velero did not populate DownloadRequest.status.downloadURL within 20s — restore-results.gz may not exist yet")
	}

	// Download + gunzip + decode.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build HTTP request: %w", err)
	}
	httpClient := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			// See comment on this function for the trust rationale.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET downloadURL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("BSL returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		// Some BSLs (or proxies) decompress on the way out, so try raw JSON
		// as a fallback before giving up.
		raw, rerr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if rerr != nil {
			return nil, fmt.Errorf("gunzip: %w (and fallback read failed: %v)", err, rerr)
		}
		var out VeleroRestoreResults
		if jerr := json.Unmarshal(raw, &out); jerr != nil {
			return nil, fmt.Errorf("gunzip failed (%v) and body is not raw JSON (%v)", err, jerr)
		}
		return &out, nil
	}
	defer gr.Close()

	var out VeleroRestoreResults
	if err := json.NewDecoder(gr).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode results JSON: %w", err)
	}
	return &out, nil
}
