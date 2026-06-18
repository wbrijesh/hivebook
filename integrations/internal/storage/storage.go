// Package storage writes raw artifact bytes to object storage — RustFS locally,
// S3 in production, via the AWS SDK so the swap is configuration only (ADR-0024).
// Layout: one bucket per (connector, region), object key <tenant-id>/<key>;
// buckets are created lazily on first write.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// Config is the object-storage connection configuration.
type Config struct {
	Endpoint  string // RustFS/S3 endpoint URL
	Region    string // SDK signing region (e.g. "us-east-1")
	AccessKey string
	SecretKey string
	Env       string // bucket-name prefix segment, e.g. "local" / "prod"
}

// ConfigFromEnv reads the HIVEBOOK_S3_* environment.
func ConfigFromEnv() Config {
	return Config{
		Endpoint:  os.Getenv("HIVEBOOK_S3_ENDPOINT"),
		Region:    os.Getenv("HIVEBOOK_S3_REGION"),
		AccessKey: os.Getenv("HIVEBOOK_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("HIVEBOOK_S3_SECRET_KEY"),
		Env:       os.Getenv("HIVEBOOK_S3_BUCKET_ENV"),
	}
}

// maxObjectBytes caps how much we read back when serving content for viewing.
const maxObjectBytes = 16 << 20 // 16 MiB

// Store puts raw bytes into object storage.
type Store struct {
	client *s3.Client
	env    string
	known  sync.Map // bucket name -> struct{}, buckets we've ensured exist
}

// New builds a Store. Path-style addressing keeps it compatible with RustFS/MinIO.
func New(ctx context.Context, cfg Config) (*Store, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = true
	})
	return &Store{client: client, env: cfg.Env}, nil
}

// Bucket returns the bucket name for a (connector, region). S3 bucket names are
// global, so the env segment keeps them unique across accounts. The name is
// sanitized to the S3 rules (lowercase, no underscores) — connector ids like
// "github_pat" would otherwise produce an invalid bucket name.
func (s *Store) Bucket(connector, region string) string {
	return bucketSafe(fmt.Sprintf("hivebook-%s-%s-%s", s.env, connector, region))
}

// bucketSafe maps a name to the S3 bucket-naming rules: lowercase, and only
// letters/digits/hyphens (underscores → hyphens). Other stray characters are
// hyphenated too, so any future connector id stays a valid bucket name.
func bucketSafe(name string) string {
	name = strings.ToLower(name)
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		default:
			return '-'
		}
	}, name)
}

// Put writes body under <tenant-id>/<key> in the (connector, region) bucket,
// creating the bucket on first use. Returns the full object key (bucket/object).
func (s *Store) Put(ctx context.Context, connector, region, tenantID, key, contentType string, body []byte) (string, error) {
	bucket := s.Bucket(connector, region)
	if err := s.ensureBucket(ctx, bucket); err != nil {
		return "", err
	}
	objectKey := tenantID + "/" + key
	in := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(body),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	if _, err := s.client.PutObject(ctx, in); err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}
	return bucket + "/" + objectKey, nil
}

// Get fetches an object's bytes and content type by its stored key
// ("bucket/object", as returned by Put). Used to serve artifact content through
// our own API (never a direct S3 URL).
func (s *Store) Get(ctx context.Context, storedKey string) ([]byte, string, error) {
	bucket, object, ok := strings.Cut(storedKey, "/")
	if !ok {
		return nil, "", fmt.Errorf("malformed object key %q", storedKey)
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		return nil, "", fmt.Errorf("get object: %w", err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(io.LimitReader(out.Body, maxObjectBytes))
	if err != nil {
		return nil, "", err
	}
	return data, aws.ToString(out.ContentType), nil
}

// Delete removes an object by its stored key ("bucket/object", as returned by Put).
// Used by the erasure outbox to hard-delete purged artifacts' bytes (ADR-0039). An
// already-gone object (NoSuchKey / NoSuchBucket / a 404) is treated as success: the
// outbox's job is "the bytes are gone", and a missing object satisfies that. Only a
// real failure (throttle, transient error) returns non-nil, keeping the manifest row
// purge_pending for the next pass — the outbox never drops the pointer to live bytes.
func (s *Store) Delete(ctx context.Context, storedKey string) error {
	bucket, object, ok := strings.Cut(storedKey, "/")
	if !ok {
		return fmt.Errorf("malformed object key %q", storedKey)
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		if isNotFound(err) {
			return nil // already gone — 204/404 both count as deleted (ADR-0039)
		}
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

// isNotFound reports whether an S3 error means the object/bucket is already gone — a
// NoSuchKey/NoSuchBucket modelled error or any 404 API response.
func isNotFound(err error) bool {
	var noKey *types.NoSuchKey
	var noBucket *types.NoSuchBucket
	if errors.As(err, &noKey) || errors.As(err, &noBucket) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NoSuchBucket", "NotFound":
			return true
		}
	}
	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) {
		return respErr.HTTPStatusCode() == http.StatusNotFound
	}
	return false
}

// ensureBucket creates the bucket if absent. Lazy and idempotent; concurrent
// creates resolve to "already owned".
func (s *Store) ensureBucket(ctx context.Context, bucket string) error {
	if _, ok := s.known.Load(bucket); ok {
		return nil
	}
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		s.known.Store(bucket, struct{}{})
		return nil
	}
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		var owned *types.BucketAlreadyOwnedByYou
		var exists *types.BucketAlreadyExists
		if !errors.As(err, &owned) && !errors.As(err, &exists) {
			return fmt.Errorf("create bucket %s: %w", bucket, err)
		}
	}
	s.known.Store(bucket, struct{}{})
	return nil
}
