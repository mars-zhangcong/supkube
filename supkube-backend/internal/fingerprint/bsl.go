// Package fingerprint: BSL adapter — the minimum GET/PUT/Prefix surface
// the writer and validator need against a Velero BackupStorageLocation.
//
// Why a separate, slimmer client (vs. reusing internal/api/v1/backup_meta_bsl.go)
// ─────────────────────────────────────────────────────────────────────────────
// The v1 package's BSL helpers are List-centric (one ListObjectsV2 hydrates
// every size for a whole BSL) and are unexported. Fingerprint only ever needs
// to GET / PUT a single 400-byte JSON object per backup, so building on List
// is overkill and exporting v1's helpers would invite import cycles
// (fingerprint→v1 while server.go wires fingerprint into a controller that
// the v1 router could later call). The duplication is contained: ~80 LOC of
// auth wiring that mirrors v1/backup_meta_bsl.go, plus PutObject which v1
// doesn't have.
//
// Provider coverage: AWS (S3 / MinIO / COS / OSS) + Azure Blob. Matches
// the v0.8.7 envelope of backup_meta_bsl.go and backup_meta_bsl_azure.go.
// GCP lands when v1 grows it.
package fingerprint

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	azservice "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsCreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"gopkg.in/ini.v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// bslClient implements BSLObjectClient against a controller-runtime client.
// One instance per process; safe for concurrent calls (the underlying
// AWS / Azure SDK clients are too).
type bslClient struct {
	cl client.Client
}

// NewBSLClient is the default BSLObjectClient — reads BSL CRs via the
// runtime client and dispatches to the right cloud SDK.
func NewBSLClient(cl client.Client) BSLObjectClient {
	return &bslClient{cl: cl}
}

func (b *bslClient) Prefix(ctx context.Context, bslName string) (string, error) {
	bsl, err := b.loadBSL(ctx, bslName)
	if err != nil {
		return "", err
	}
	if bsl.Spec.ObjectStorage == nil {
		return "", nil
	}
	return strings.TrimSuffix(bsl.Spec.ObjectStorage.Prefix, "/"), nil
}

func (b *bslClient) GetObject(ctx context.Context, bslName, key string) ([]byte, error) {
	bsl, err := b.loadBSL(ctx, bslName)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(bsl.Spec.Provider) {
	case "aws":
		return b.getAWS(ctx, bsl, key)
	case "azure":
		return b.getAzure(ctx, bsl, key)
	}
	return nil, fmt.Errorf("fingerprint: provider %q not supported (aws/azure only in v1)", bsl.Spec.Provider)
}

func (b *bslClient) PutObject(ctx context.Context, bslName, key string, body []byte) error {
	bsl, err := b.loadBSL(ctx, bslName)
	if err != nil {
		return err
	}
	switch strings.ToLower(bsl.Spec.Provider) {
	case "aws":
		return b.putAWS(ctx, bsl, key, body)
	case "azure":
		return b.putAzure(ctx, bsl, key, body)
	}
	return fmt.Errorf("fingerprint: provider %q not supported (aws/azure only in v1)", bsl.Spec.Provider)
}

// ListPrefix walks the BSL under keyPrefix and returns every object key.
// keyPrefix follows the same convention as GetObject/PutObject's key arg
// — the BSL's configured prefix is appended automatically. Pagination
// is internal; callers receive the full slice. Used by the ImportPolicy
// S3 lister (Agent G, sprint 2026-06-01) to discover backups on a BSL
// directly rather than relying on Velero's BackupSyncController.
func (b *bslClient) ListPrefix(ctx context.Context, bslName, keyPrefix string) ([]string, error) {
	bsl, err := b.loadBSL(ctx, bslName)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(bsl.Spec.Provider) {
	case "aws":
		return b.listAWS(ctx, bsl, keyPrefix)
	case "azure":
		return b.listAzure(ctx, bsl, keyPrefix)
	}
	return nil, fmt.Errorf("fingerprint: provider %q not supported (aws/azure only in v1)", bsl.Spec.Provider)
}

func (b *bslClient) loadBSL(ctx context.Context, name string) (*velerov1.BackupStorageLocation, error) {
	bsl := &velerov1.BackupStorageLocation{}
	if err := b.cl.Get(ctx, client.ObjectKey{Name: name, Namespace: "velero"}, bsl); err != nil {
		return nil, fmt.Errorf("get BSL %s: %w", name, err)
	}
	return bsl, nil
}

// ─────────────────────────────────────────────────────────────────────
// AWS (S3 / MinIO / COS / OSS)
// ─────────────────────────────────────────────────────────────────────

func (b *bslClient) getAWS(ctx context.Context, bsl *velerov1.BackupStorageLocation, key string) ([]byte, error) {
	bucket, prefix := bslBucketAndPrefix(bsl)
	if bucket == "" {
		return nil, fmt.Errorf("BSL missing objectStorage.bucket")
	}
	cli, err := b.buildS3Client(ctx, bsl)
	if err != nil {
		return nil, err
	}
	fullKey := path.Join(prefix, key)
	out, err := cli.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(fullKey)})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// listAWS pages through ListObjectsV2 under <bslPrefix>/<keyPrefix>/ and
// returns every key found (full key, with the BSL prefix included — so
// callers can pass them straight back into GetObject by stripping the
// BSL prefix if desired). Pagination guards against the 1000-object
// default page size; we keep going until HasMorePages returns false.
func (b *bslClient) listAWS(ctx context.Context, bsl *velerov1.BackupStorageLocation, keyPrefix string) ([]string, error) {
	bucket, prefix := bslBucketAndPrefix(bsl)
	if bucket == "" {
		return nil, fmt.Errorf("BSL missing objectStorage.bucket")
	}
	cli, err := b.buildS3Client(ctx, bsl)
	if err != nil {
		return nil, err
	}
	fullPrefix := path.Join(prefix, keyPrefix)
	if !strings.HasSuffix(fullPrefix, "/") {
		fullPrefix += "/"
	}
	out := []string{}
	paginator := s3.NewListObjectsV2Paginator(cli, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(fullPrefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list S3 prefix %s: %w", fullPrefix, err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			out = append(out, *obj.Key)
		}
	}
	return out, nil
}

func (b *bslClient) putAWS(ctx context.Context, bsl *velerov1.BackupStorageLocation, key string, body []byte) error {
	bucket, prefix := bslBucketAndPrefix(bsl)
	if bucket == "" {
		return fmt.Errorf("BSL missing objectStorage.bucket")
	}
	cli, err := b.buildS3Client(ctx, bsl)
	if err != nil {
		return err
	}
	fullKey := path.Join(prefix, key)
	_, err = cli.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(fullKey),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
	})
	return err
}

// buildS3Client mirrors internal/api/v1/backup_meta_bsl.go::buildS3Client.
// Kept in-package to avoid a circular import. If we ever lift BSL helpers
// into internal/bsl/, BOTH this and the v1 copy can collapse into it.
func (b *bslClient) buildS3Client(ctx context.Context, bsl *velerov1.BackupStorageLocation) (*s3.Client, error) {
	cred := bsl.Spec.Credential
	region := bsl.Spec.Config["region"]
	if region == "" {
		region = "us-east-1"
	}

	loadStatic := func(ak, sk, st string) (*s3.Client, error) {
		cfg, err := awsConfig.LoadDefaultConfig(ctx,
			awsConfig.WithRegion(region),
			awsConfig.WithCredentialsProvider(awsCreds.NewStaticCredentialsProvider(ak, sk, st)),
		)
		if err != nil {
			return nil, err
		}
		return s3FromConfig(cfg, bsl), nil
	}

	if cred == nil || cred.Name == "" {
		// Fall back to velero's `cloud-credentials`; if that's missing
		// too, try the SDK default chain (IRSA / IMDS).
		fallback := &corev1.Secret{}
		if err := b.cl.Get(ctx, client.ObjectKey{Name: "cloud-credentials", Namespace: "velero"}, fallback); err == nil {
			if raw, ok := fallback.Data["cloud"]; ok {
				if ak, sk, st, perr := parseAWSCredsINI(raw); perr == nil {
					return loadStatic(ak, sk, st)
				}
			}
		}
		cfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region))
		if err != nil {
			return nil, err
		}
		return s3FromConfig(cfg, bsl), nil
	}

	secret := &corev1.Secret{}
	if err := b.cl.Get(ctx, client.ObjectKey{Name: cred.Name, Namespace: "velero"}, secret); err != nil {
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
	return loadStatic(ak, sk, st)
}

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

func bslBucketAndPrefix(bsl *velerov1.BackupStorageLocation) (string, string) {
	if bsl.Spec.ObjectStorage == nil {
		return "", ""
	}
	return bsl.Spec.ObjectStorage.Bucket, strings.TrimSuffix(bsl.Spec.ObjectStorage.Prefix, "/")
}

func parseAWSCredsINI(raw []byte) (ak, sk, st string, err error) {
	f, err := ini.Load(raw)
	if err != nil {
		return "", "", "", err
	}
	var sec *ini.Section
	for _, s := range f.Sections() {
		if s.Name() == ini.DefaultSection {
			continue
		}
		sec = s
		break
	}
	if sec == nil {
		sec = f.Section(ini.DefaultSection)
	}
	ak = sec.Key("aws_access_key_id").String()
	sk = sec.Key("aws_secret_access_key").String()
	st = sec.Key("aws_session_token").String()
	if ak == "" || sk == "" {
		return "", "", "", fmt.Errorf("missing aws_access_key_id / aws_secret_access_key")
	}
	return ak, sk, st, nil
}

// ─────────────────────────────────────────────────────────────────────
// Azure Blob
// ─────────────────────────────────────────────────────────────────────

func (b *bslClient) getAzure(ctx context.Context, bsl *velerov1.BackupStorageLocation, key string) ([]byte, error) {
	containerName, prefix := bslBucketAndPrefix(bsl)
	if containerName == "" {
		return nil, fmt.Errorf("BSL missing objectStorage.bucket (Azure container)")
	}
	storageAccount := bsl.Spec.Config["storageAccount"]
	if storageAccount == "" {
		return nil, fmt.Errorf("BSL missing config.storageAccount")
	}
	accountKey, err := b.loadAzureSharedKey(ctx, bsl)
	if err != nil {
		return nil, err
	}
	if accountKey == "" {
		return nil, fmt.Errorf("v1 fingerprint requires storage-account-key auth; AAD SP lands later")
	}
	cred, err := azblob.NewSharedKeyCredential(storageAccount, accountKey)
	if err != nil {
		return nil, err
	}
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", storageAccount)
	svc, err := azservice.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return nil, err
	}
	fullKey := path.Join(prefix, key)
	blobCli := svc.NewContainerClient(containerName).NewBlobClient(fullKey)
	resp, err := blobCli.DownloadStream(ctx, &blob.DownloadStreamOptions{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (b *bslClient) putAzure(ctx context.Context, bsl *velerov1.BackupStorageLocation, key string, body []byte) error {
	containerName, prefix := bslBucketAndPrefix(bsl)
	if containerName == "" {
		return fmt.Errorf("BSL missing objectStorage.bucket (Azure container)")
	}
	storageAccount := bsl.Spec.Config["storageAccount"]
	if storageAccount == "" {
		return fmt.Errorf("BSL missing config.storageAccount")
	}
	accountKey, err := b.loadAzureSharedKey(ctx, bsl)
	if err != nil {
		return err
	}
	if accountKey == "" {
		return fmt.Errorf("v1 fingerprint requires storage-account-key auth")
	}
	cred, err := azblob.NewSharedKeyCredential(storageAccount, accountKey)
	if err != nil {
		return err
	}
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", storageAccount)
	svc, err := azservice.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return err
	}
	fullKey := path.Join(prefix, key)
	_, err = svc.NewContainerClient(containerName).NewBlockBlobClient(fullKey).UploadBuffer(ctx, body, nil)
	return err
}

// listAzure mirrors listAWS for Azure Blob — pages the container under
// <bslPrefix>/<keyPrefix>/ via NewListBlobsFlatPager. Used by the
// ImportPolicy S3 lister; we keep the name listAzure for symmetry even
// though the public method is ListPrefix.
func (b *bslClient) listAzure(ctx context.Context, bsl *velerov1.BackupStorageLocation, keyPrefix string) ([]string, error) {
	containerName, prefix := bslBucketAndPrefix(bsl)
	if containerName == "" {
		return nil, fmt.Errorf("BSL missing objectStorage.bucket (Azure container)")
	}
	storageAccount := bsl.Spec.Config["storageAccount"]
	if storageAccount == "" {
		return nil, fmt.Errorf("BSL missing config.storageAccount")
	}
	accountKey, err := b.loadAzureSharedKey(ctx, bsl)
	if err != nil {
		return nil, err
	}
	if accountKey == "" {
		return nil, fmt.Errorf("v1 fingerprint requires storage-account-key auth")
	}
	cred, err := azblob.NewSharedKeyCredential(storageAccount, accountKey)
	if err != nil {
		return nil, err
	}
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", storageAccount)
	svc, err := azservice.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return nil, err
	}
	fullPrefix := path.Join(prefix, keyPrefix)
	if !strings.HasSuffix(fullPrefix, "/") {
		fullPrefix += "/"
	}
	out := []string{}
	pager := svc.NewContainerClient(containerName).NewListBlobsFlatPager(&container.ListBlobsFlatOptions{
		Prefix: &fullPrefix,
	})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list Azure prefix %s: %w", fullPrefix, err)
		}
		if page.Segment == nil {
			continue
		}
		for _, blobItem := range page.Segment.BlobItems {
			if blobItem == nil || blobItem.Name == nil {
				continue
			}
			out = append(out, *blobItem.Name)
		}
	}
	return out, nil
}

func (b *bslClient) loadAzureSharedKey(ctx context.Context, bsl *velerov1.BackupStorageLocation) (string, error) {
	cred := bsl.Spec.Credential
	if cred == nil || cred.Name == "" {
		secret := &corev1.Secret{}
		if err := b.cl.Get(ctx, client.ObjectKey{Name: "cloud-credentials", Namespace: "velero"}, secret); err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("no BSL credential and cloud-credentials Secret not found")
			}
			return "", err
		}
		return parseAzureKey(secret.Data["cloud"]), nil
	}
	secret := &corev1.Secret{}
	if err := b.cl.Get(ctx, client.ObjectKey{Name: cred.Name, Namespace: "velero"}, secret); err != nil {
		return "", fmt.Errorf("get secret %s: %w", cred.Name, err)
	}
	raw, ok := secret.Data[cred.Key]
	if !ok {
		return "", fmt.Errorf("secret %s has no key %q", cred.Name, cred.Key)
	}
	return parseAzureKey(raw), nil
}

func parseAzureKey(raw []byte) string {
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		k := strings.ToUpper(strings.TrimSpace(line[:eq]))
		v := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		if k == "AZURE_STORAGE_ACCOUNT_ACCESS_KEY" {
			return v
		}
	}
	return ""
}
