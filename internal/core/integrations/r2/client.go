package r2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/Nidka/BPM/internal/core/config"
	"github.com/Nidka/BPM/internal/core/contracts"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	ErrInvalidObjectKey = errors.New("invalid private object key")
	ErrInvalidMedia     = errors.New("invalid media metadata")
)

type Client struct {
	bucket       string
	presign      *s3.PresignClient
	s3           *s3.Client
	ttl          time.Duration
	maxBytes     int64
	allowedMIMEs map[string]struct{}
}

func New(ctx context.Context, cfg config.R2Config) (*Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize R2 configuration: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(cfg.Endpoint)
		options.UsePathStyle = true
	})

	return &Client{
		bucket:       cfg.Bucket,
		presign:      s3.NewPresignClient(s3Client),
		s3:           s3Client,
		ttl:          cfg.SignedURLTTL,
		maxBytes:     cfg.MaxUploadBytes,
		allowedMIMEs: cfg.AllowedMIMETypes,
	}, nil
}

func (c *Client) PresignUpload(ctx context.Context, request contracts.UploadRequest) (contracts.SignedRequest, error) {
	if err := validateObjectKey(request.ObjectKey); err != nil {
		return contracts.SignedRequest{}, err
	}
	contentType := strings.ToLower(strings.TrimSpace(request.ContentType))
	if _, allowed := c.allowedMIMEs[contentType]; !allowed || request.ContentLength < 1 || request.ContentLength > c.maxBytes {
		return contracts.SignedRequest{}, ErrInvalidMedia
	}
	if len(request.SHA256Hex) != sha256.Size*2 {
		return contracts.SignedRequest{}, ErrInvalidMedia
	}
	if _, err := hex.DecodeString(request.SHA256Hex); err != nil {
		return contracts.SignedRequest{}, ErrInvalidMedia
	}

	result, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(request.ObjectKey),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(request.ContentLength),
		Metadata:      map[string]string{"sha256": strings.ToLower(request.SHA256Hex)},
	}, s3.WithPresignExpires(c.ttl))
	if err != nil {
		return contracts.SignedRequest{}, fmt.Errorf("presign R2 upload: %w", err)
	}
	return contracts.SignedRequest{
		URL:       result.URL,
		Headers:   result.SignedHeader,
		ExpiresAt: time.Now().UTC().Add(c.ttl),
	}, nil
}

func (c *Client) PresignDownload(ctx context.Context, objectKey string) (contracts.SignedRequest, error) {
	if err := validateObjectKey(objectKey); err != nil {
		return contracts.SignedRequest{}, err
	}
	result, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(c.ttl))
	if err != nil {
		return contracts.SignedRequest{}, fmt.Errorf("presign R2 download: %w", err)
	}
	return contracts.SignedRequest{
		URL:       result.URL,
		Headers:   result.SignedHeader,
		ExpiresAt: time.Now().UTC().Add(c.ttl),
	}, nil
}

func (c *Client) Head(ctx context.Context, objectKey string) (contracts.ObjectMetadata, error) {
	if err := validateObjectKey(objectKey); err != nil {
		return contracts.ObjectMetadata{}, err
	}
	result, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return contracts.ObjectMetadata{}, fmt.Errorf("read R2 object metadata: %w", err)
	}
	return contracts.ObjectMetadata{
		ContentType:   aws.ToString(result.ContentType),
		ContentLength: aws.ToInt64(result.ContentLength),
		SHA256Hex:     result.Metadata["sha256"],
	}, nil
}

func validateObjectKey(key string) error {
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") || strings.ContainsRune(key, '\x00') {
		return ErrInvalidObjectKey
	}
	if path.Clean(key) != key || !strings.HasPrefix(key, "organizations/") {
		return ErrInvalidObjectKey
	}
	return nil
}

var _ contracts.ObjectStorage = (*Client)(nil)
