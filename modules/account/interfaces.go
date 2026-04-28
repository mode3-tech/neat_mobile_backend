package account

import (
	"context"
	"io"
	"time"
)

type UploadService interface {
	UploadDocument(ctx context.Context, key string, body io.ReadSeeker, contentType string) error
	UploadProfilePicture(ctx context.Context, key string, body io.ReadSeeker, contentType string) error
	PresignURL(ctx context.Context, filePath string, ttl time.Duration) (string, error)
	FileURL(key string) string
	ProfilePictureURL(key string) string
}
