// Package v1: Azure Blob tarball-size lookup (v0.8.7).
//
// Sister to backup_meta_bsl.go's AWS-flavored path. Same shape:
//
//   hydrateBSLSizesAzure(ctx, cl, bsl) → tarballSizes
//
// What's different from the S3 path
// ─────────────────────────────────
//   1. Auth: Azure plugin's credential Secret stores key=value pairs
//      (not an AWS INI file). We parse those + use shared-key auth.
//   2. SDK: github.com/Azure/azure-sdk-for-go/sdk/storage/azblob — its
//      List walker has a totally different shape than S3's paginator,
//      so we can't reuse the S3 loop.
//   3. Endpoint: Azure containers live under
//      https://<storageAccount>.blob.core.windows.net/<container>/...
//      — no region, no path-style toggle.
//
// What stays the same: the cache (per-BSL 60s TTL via tarballSizes),
// the error-message trimming for UI tooltips, and the prefix scan over
// "backups/" so each backup's tarball is identified by directory layout.
package v1

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	azservice "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// hydrateBSLSizesAzure walks an Azure Blob container under the
// "backups/" prefix and returns a name→size map for every Velero
// backup tarball it finds. Cache-friendly: one List per BSL hydrate.
func hydrateBSLSizesAzure(ctx context.Context, cl client.Client, bsl *velerov1.BackupStorageLocation) tarballSizes {
	if bsl.Status.Phase == velerov1.BackupStorageLocationPhaseUnavailable {
		return tarballSizes{expiry: time.Now().Add(bslCacheTTL), err: "BSL Unavailable — Azure container unreachable from Velero"}
	}
	containerName, prefix := bslBucketAndPrefix(bsl)
	if containerName == "" {
		return tarballSizes{expiry: time.Now().Add(bslCacheTTL), err: "BSL missing objectStorage.bucket (the Azure container name)"}
	}
	storageAccount := bsl.Spec.Config["storageAccount"]
	if storageAccount == "" {
		return tarballSizes{expiry: time.Now().Add(bslCacheTTL), err: "BSL missing config.storageAccount"}
	}

	accountKey, err := loadAzureSharedKey(ctx, cl, bsl)
	if err != nil {
		return tarballSizes{expiry: time.Now().Add(bslCacheTTL), err: "load Azure credentials: " + err.Error()}
	}
	if accountKey == "" {
		// AAD service principal isn't supported in v0.8.7 — surface a
		// clear message instead of silently failing inside the SDK.
		return tarballSizes{
			expiry: time.Now().Add(bslCacheTTL),
			err:    "v0.8.7 supports storage-account-key auth only; AAD service-principal lands in v0.9",
		}
	}

	cred, err := azblob.NewSharedKeyCredential(storageAccount, accountKey)
	if err != nil {
		return tarballSizes{expiry: time.Now().Add(bslCacheTTL), err: "build SharedKeyCredential: " + err.Error()}
	}
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", storageAccount)
	svc, err := azservice.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return tarballSizes{expiry: time.Now().Add(bslCacheTTL), err: "build azblob service client: " + err.Error()}
	}
	containerClient := svc.NewContainerClient(containerName)

	// Velero uploads to "<prefix>/backups/<name>/<name>.tar.gz" — same
	// layout as S3, so the prefix scan logic is identical conceptually.
	listPrefix := path.Join(prefix, "backups")
	if !strings.HasSuffix(listPrefix, "/") {
		listPrefix += "/"
	}

	sizes := map[string]int64{}
	pager := containerClient.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{
		Prefix: &listPrefix,
	})
	for pager.More() {
		pageCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		page, err := pager.NextPage(pageCtx)
		cancel()
		if err != nil {
			short := err.Error()
			if idx := strings.IndexByte(short, '\n'); idx > 0 {
				short = short[:idx]
			}
			if len(short) > 120 {
				short = short[:117] + "..."
			}
			return tarballSizes{expiry: time.Now().Add(bslCacheTTL), err: "Azure list failed: " + short}
		}
		if page.Segment == nil {
			continue
		}
		for _, blob := range page.Segment.BlobItems {
			if blob == nil || blob.Name == nil || blob.Properties == nil || blob.Properties.ContentLength == nil {
				continue
			}
			key := *blob.Name
			if !strings.HasSuffix(key, ".tar.gz") {
				continue
			}
			rel := strings.TrimPrefix(key, listPrefix)
			parts := strings.Split(rel, "/")
			if len(parts) < 2 {
				continue
			}
			name := parts[0]
			if key == path.Join(listPrefix, name, name+".tar.gz") {
				sizes[name] = *blob.Properties.ContentLength
			}
		}
	}
	return tarballSizes{sizes: sizes, expiry: time.Now().Add(bslCacheTTL)}
}

// loadAzureSharedKey fetches the storage account key from the BSL's
// credential Secret. Velero's Azure plugin stores credentials as
// key=value pairs (one per line) in the "cloud" key of the Secret —
// we re-parse them here using a simple split rather than dragging in
// a full INI parser, since the format isn't quite INI (no [section]).
//
// Returns ("", nil) when the Secret exists but contains only AAD
// credentials (service principal), so the caller can surface the
// "not yet supported" message rather than crashing.
func loadAzureSharedKey(ctx context.Context, cl client.Client, bsl *velerov1.BackupStorageLocation) (string, error) {
	cred := bsl.Spec.Credential
	if cred == nil || cred.Name == "" {
		// Fall back to the Velero install convention — same as the AWS
		// path. The default Secret name for an Azure-installed Velero
		// is also `cloud-credentials`.
		secret := &corev1.Secret{}
		if err := cl.Get(ctx, client.ObjectKey{Name: "cloud-credentials", Namespace: "velero"}, secret); err != nil {
			return "", fmt.Errorf("no credential on BSL and cloud-credentials missing: %w", err)
		}
		return parseAzureSecretForKey(secret.Data["cloud"]), nil
	}
	secret := &corev1.Secret{}
	if err := cl.Get(ctx, client.ObjectKey{Name: cred.Name, Namespace: "velero"}, secret); err != nil {
		return "", fmt.Errorf("get secret %s: %w", cred.Name, err)
	}
	raw, ok := secret.Data[cred.Key]
	if !ok {
		return "", fmt.Errorf("secret %s has no key %q", cred.Name, cred.Key)
	}
	return parseAzureSecretForKey(raw), nil
}

// parseAzureSecretForKey scans key=value lines for AZURE_STORAGE_ACCOUNT_ACCESS_KEY.
// Tolerates whitespace around = and case-insensitive key names (some
// older Velero docs use mixed case).
func parseAzureSecretForKey(raw []byte) string {
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(line[:eq]))
		val := strings.TrimSpace(line[eq+1:])
		// Strip optional surrounding quotes that some users paste in.
		val = strings.Trim(val, `"'`)
		if key == "AZURE_STORAGE_ACCOUNT_ACCESS_KEY" {
			return val
		}
	}
	return ""
}
