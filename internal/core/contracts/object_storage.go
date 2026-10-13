package contracts

import (
	"context"
	"time"
)

type UploadRequest struct {
	ObjectKey     string
	ContentType   string
	ContentLength int64
	SHA256Hex     string
}

type SignedRequest struct {
	URL       string
	Headers   map[string][]string
	ExpiresAt time.Time
}

type ObjectMetadata struct {
	ContentType   string
	ContentLength int64
	SHA256Hex     string
}

// ObjectStorage is the outbound port for private evidence files.
type ObjectStorage interface {
	PresignUpload(ctx context.Context, request UploadRequest) (SignedRequest, error)
	PresignDownload(ctx context.Context, objectKey string) (SignedRequest, error)
	Head(ctx context.Context, objectKey string) (ObjectMetadata, error)
}
