// Package v1: BSL (BackupStorageLocation) tarball-size lookup (v0.8.6).
//
// Why this file is separate from backup_meta.go: BSL access has its own
// dependency tree (aws-sdk-go-v2) and its own failure modes (network,
// credentials, bucket auth). Keeping it isolated means a misconfigured
// BSL can't break the dataPath/volumeBytes calculation that runs purely
// off in-cluster state.
//
// What it does
// ────────────
// Given a Backup CR, finds its BSL, opens an S3-compatible client using
// the BSL-referenced credentials Secret, and calls HeadObject on the
// resource tarball at:
//
//   s3://<bucket>/<prefix>/backups/<name>/<name>.tar.gz
//
// Returns Content-Length, or an error string the UI can display.
//
// Scope (v0.8.6)
// ──────────────
//   ✅ provider: aws  (S3 / MinIO / Tencent COS / Aliyun OSS — all S3-API)
//   ❌ provider: gcp  (different SDK; v0.9)
//   ❌ provider: azure (different SDK; v0.9)
//
// Performance
// ───────────
// ListBackups can render 100+ rows. Per-row HeadObject = 100 round-trips.
// Instead we do ONE ListObjectsV2 per BSL with prefix "backups/", build
// an in-memory map keyed by backup name, and use that for the whole list.
// Cached per-BSL with a 60s TTL — see bslSizeCache.
package v1

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsCreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gopkg.in/ini.v1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// tarballSizes is a per-BSL cache of "backup name → tarball byte size".
// One ListObjectsV2 call hydrates the whole BSL; subsequent ListBackups
// requests served from the cache until TTL expires. Recompute happens
// on next miss after expiry — no background refresh thread.
type tarballSizes struct {
	sizes  map[string]int64
	expiry time.Time
	err    string
}

type bslSizeCache struct {
	mu sync.Mutex
	m  map[string]tarballSizes // key = BSL name
}

var bslCache = &bslSizeCache{m: map[string]tarballSizes{}}

const bslCacheTTL = 60 * time.Second

// getBackupTarballSize returns the cached tarball size for backupName.
// On cache miss / expiry it hydrates from BSL. Errors are stickied for
// the TTL so a broken BSL doesn't cause a HeadObject storm.
//
// Returns (bytes, errString). errString is the user-facing reason when
// bytes == 0; empty when the lookup succeeded.
func getBackupTarballSize(ctx context.Context, cl client.Client, backup *velerov1.Backup) (int64, string) {
	if backup == nil {
		return 0, "no backup"
	}
	bslName := backup.Spec.StorageLocation
	if bslName == "" {
		bslName = "default"
	}

	bslCache.mu.Lock()
	entry, ok := bslCache.m[bslName]
	bslCache.mu.Unlock()

	if !ok || time.Now().After(entry.expiry) {
		entry = hydrateBSLSizes(ctx, cl, bslName)
		bslCache.mu.Lock()
		bslCache.m[bslName] = entry
		bslCache.mu.Unlock()
	}
	if entry.err != "" {
		return 0, entry.err
	}
	if v, ok := entry.sizes[backup.Name]; ok {
		return v, ""
	}
	// Backup name not found in BSL — either still uploading or this
	// backup never finished. Don't treat as error; just zero.
	return 0, ""
}

// hydrateBSLSizes opens the BSL and bulk-lists object sizes under
// "backups/" prefix. Returns a cache entry valid for bslCacheTTL.
//
// v0.8.7: provider dispatch — aws/S3 path stays the way it was, and we
// add `provider: azure` via the azblob SDK. The two share NOTHING at
// the API level (different auth, different list call shape), so they
// live in dedicated helpers. The dispatch + caching live here.
func hydrateBSLSizes(ctx context.Context, cl client.Client, bslName string) tarballSizes {
	bsl := &velerov1.BackupStorageLocation{}
	if err := cl.Get(ctx, client.ObjectKey{Name: bslName, Namespace: "velero"}, bsl); err != nil {
		return tarballSizes{expiry: time.Now().Add(bslCacheTTL), err: "BSL not found: " + err.Error()}
	}
	switch strings.ToLower(bsl.Spec.Provider) {
	case "aws":
		return hydrateBSLSizesAWS(ctx, cl, bsl)
	case "azure":
		return hydrateBSLSizesAzure(ctx, cl, bsl)
	}
	return tarballSizes{
		expiry: time.Now().Add(bslCacheTTL),
		err:    fmt.Sprintf("provider %q not supported yet (v0.9 will add gcp)", bsl.Spec.Provider),
	}
}

// hydrateBSLSizesAWS is the original v0.8.6 S3/MinIO/COS path, lifted
// out of hydrateBSLSizes so we can fork by provider cleanly.
func hydrateBSLSizesAWS(ctx context.Context, cl client.Client, bsl *velerov1.BackupStorageLocation) tarballSizes {
	// BSL phase check: if Velero marks the BSL unavailable, the bucket
	// is unreachable from the Velero pod's perspective — we'd just be
	// queueing the same failure. Surface fast.
	if bsl.Status.Phase == velerov1.BackupStorageLocationPhaseUnavailable {
		return tarballSizes{
			expiry: time.Now().Add(bslCacheTTL),
			err:    "BSL Unavailable — bucket unreachable from Velero",
		}
	}

	bucket, prefix := bslBucketAndPrefix(bsl)
	if bucket == "" {
		return tarballSizes{expiry: time.Now().Add(bslCacheTTL), err: "BSL missing objectStorage.bucket"}
	}

	s3Client, err := buildS3Client(ctx, cl, bsl)
	if err != nil {
		return tarballSizes{expiry: time.Now().Add(bslCacheTTL), err: "build S3 client: " + err.Error()}
	}

	// Walk the bucket under "<prefix>/backups/" — that's where Velero
	// uploads every backup's artifact set. We sum the sizes of every
	// object whose key matches "backups/<name>/<name>.tar.gz". A single
	// ListObjectsV2 with the prefix returns all backups in one round-trip
	// (unless there are >1000 — handled by the paginator).
	listPrefix := path.Join(prefix, "backups")
	if !strings.HasSuffix(listPrefix, "/") {
		listPrefix += "/"
	}

	sizes := map[string]int64{}
	paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(listPrefix),
	})
	for paginator.HasMorePages() {
		// Bound each page call to avoid hangs on misbehaving providers.
		pageCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		page, err := paginator.NextPage(pageCtx)
		cancel()
		if err != nil {
			// Trim the SDK's verbose multi-line "operation error X: failed
			// to Y: ..." to the first line — that's what fits in a UI
			// tooltip. The full chain stays in backend logs for debugging.
			short := err.Error()
			if idx := strings.IndexByte(short, '\n'); idx > 0 {
				short = short[:idx]
			}
			if len(short) > 120 {
				short = short[:117] + "..."
			}
			return tarballSizes{
				expiry: time.Now().Add(bslCacheTTL),
				err:    "S3 list failed: " + short,
			}
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			// Match keys like "<prefix>/backups/<name>/<name>.tar.gz".
			// Velero's naming is strict so a suffix check is enough.
			if !strings.HasSuffix(*obj.Key, ".tar.gz") {
				continue
			}
			parts := strings.Split(strings.TrimPrefix(*obj.Key, listPrefix), "/")
			if len(parts) < 2 {
				continue
			}
			name := parts[0]
			// SDK v2 surfaces Size as int64 (not *int64) — zero values
			// are legitimate empty objects, but Velero tarballs are
			// never zero, so we don't bother filtering.
			if *obj.Key == path.Join(listPrefix, name, name+".tar.gz") {
				sizes[name] = obj.Size
			}
		}
	}
	return tarballSizes{sizes: sizes, expiry: time.Now().Add(bslCacheTTL)}
}

// bslBucketAndPrefix extracts the bucket name and optional key prefix
// from BSL.spec.objectStorage. Velero allows "<bucket>/<sub-path>" via
// the prefix field — keep them apart here.
func bslBucketAndPrefix(bsl *velerov1.BackupStorageLocation) (string, string) {
	if bsl.Spec.ObjectStorage == nil {
		return "", ""
	}
	return bsl.Spec.ObjectStorage.Bucket, strings.TrimSuffix(bsl.Spec.ObjectStorage.Prefix, "/")
}

// buildS3Client constructs an aws-sdk-go-v2 S3 client from a BSL.
//
// Three things drive the config:
//   1. credentials Secret (BSL.spec.credential.{name,key}) — typically
//      "[default]\naws_access_key_id=...\naws_secret_access_key=...";
//      we parse it as INI rather than relying on the shared-creds env
//   2. region (BSL.spec.config["region"])
//   3. endpoint override (BSL.spec.config["s3Url"]) for MinIO / COS / OSS
//   4. path-style addressing (BSL.spec.config["s3ForcePathStyle"]="true")
func buildS3Client(ctx context.Context, cl client.Client, bsl *velerov1.BackupStorageLocation) (*s3.Client, error) {
	cred := bsl.Spec.Credential
	if cred == nil || cred.Name == "" {
		// BSL didn't pin credentials → Velero's convention is to fall
		// back to the cluster-wide `cloud-credentials` Secret in the
		// velero namespace (key=cloud). That's what `velero install`
		// creates by default. We mirror this so the default BSL "just
		// works" without the user explicitly wiring credentials on it.
		//
		// IF that Secret is also missing we let the AWS SDK try its
		// default chain (IRSA / pod identity / env vars) — the SDK
		// will error out with "no EC2 IMDS role" if it can't find
		// anything, which is the truthful message in that case.
		fallback := &corev1.Secret{}
		if err := cl.Get(ctx, client.ObjectKey{Name: "cloud-credentials", Namespace: "velero"}, fallback); err == nil {
			if raw, ok := fallback.Data["cloud"]; ok {
				if ak, sk, st, perr := parseAWSCredsINI(raw); perr == nil {
					region := bsl.Spec.Config["region"]
					if region == "" {
						region = "us-east-1"
					}
					cfg, err := awsConfig.LoadDefaultConfig(ctx,
						awsConfig.WithRegion(region),
						awsConfig.WithCredentialsProvider(awsCreds.NewStaticCredentialsProvider(ak, sk, st)),
					)
					if err != nil {
						return nil, err
					}
					return s3FromConfig(cfg, bsl), nil
				}
			}
		}
		// Last resort: SDK default chain. Will likely fail in clusters
		// without IRSA but at least surfaces a sensible error message.
		cfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(bsl.Spec.Config["region"]))
		if err != nil {
			return nil, err
		}
		return s3FromConfig(cfg, bsl), nil
	}

	// Pull the Secret. Velero stores it in the velero namespace.
	secret := &corev1.Secret{}
	if err := cl.Get(ctx, client.ObjectKey{Name: cred.Name, Namespace: "velero"}, secret); err != nil {
		return nil, fmt.Errorf("get secret %s: %w", cred.Name, err)
	}
	raw, ok := secret.Data[cred.Key]
	if !ok {
		return nil, fmt.Errorf("secret %s has no key %q", cred.Name, cred.Key)
	}

	ak, sk, st, err := parseAWSCredsINI(raw)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	region := bsl.Spec.Config["region"]
	if region == "" {
		// Some providers don't care about region (path-style MinIO);
		// the AWS SDK insists on SOMETHING. "us-east-1" is the safe
		// default that S3-compatible providers ignore.
		region = "us-east-1"
	}

	cfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithRegion(region),
		awsConfig.WithCredentialsProvider(awsCreds.NewStaticCredentialsProvider(ak, sk, st)),
	)
	if err != nil {
		return nil, err
	}
	return s3FromConfig(cfg, bsl), nil
}

// s3FromConfig produces an S3 client with BSL-specific endpoint /
// path-style overrides applied. Kept separate so the credential and
// no-credential paths share the override logic.
func s3FromConfig(cfg aws.Config, bsl *velerov1.BackupStorageLocation) *s3.Client {
	endpoint := bsl.Spec.Config["s3Url"]
	forcePath := strings.EqualFold(bsl.Spec.Config["s3ForcePathStyle"], "true")
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = forcePath
	})
}

// parseAWSCredsINI extracts AccessKey / SecretKey / SessionToken from
// the standard "[default]" credentials file format Velero uses. Returns
// ("", "", "", error) on malformed input.
func parseAWSCredsINI(raw []byte) (ak, sk, st string, err error) {
	f, err := ini.Load(raw)
	if err != nil {
		return "", "", "", err
	}
	// Velero conventionally uses [default]; tolerate whatever's there
	// by taking the first non-DEFAULT section.
	var sec *ini.Section
	for _, s := range f.Sections() {
		if s.Name() == ini.DefaultSection {
			continue
		}
		sec = s
		break
	}
	if sec == nil {
		// File was just key=value with no section header. Use DEFAULT.
		sec = f.Section(ini.DefaultSection)
	}
	ak = sec.Key("aws_access_key_id").String()
	sk = sec.Key("aws_secret_access_key").String()
	st = sec.Key("aws_session_token").String()
	if ak == "" || sk == "" {
		return "", "", "", fmt.Errorf("missing aws_access_key_id / aws_secret_access_key in credentials")
	}
	return ak, sk, st, nil
}
