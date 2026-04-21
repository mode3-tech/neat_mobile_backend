package s3bucket

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/kurin/blazer/b2"
)

type BackblazeConfig struct {
	KeyID                  string `json:"key_id" binding:"required"`
	AppKey                 string `json:"app_key" binding:"required"`
	DocsBucketName         string `json:"doc_bucket_name" binding:"required"`
	PublicAssetsBucketName string `json:"pfps_bucket_name" binding:"required"`
}

type BackblazeClient struct {
	client             *b2.Client
	bucket             *b2.Bucket
	publicAssetsBucket *b2.Bucket
}

func NewBackblazeClient(ctx context.Context, cfg BackblazeConfig) (*BackblazeClient, error) {
	client, err := b2.NewClient(ctx, cfg.KeyID, cfg.AppKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create B2 client: %w", err)
	}

	bucket, err := client.Bucket(ctx, cfg.DocsBucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket '%s': %w", cfg.DocsBucketName, err)
	}

	publicAssetsBucket, err := client.Bucket(ctx, cfg.PublicAssetsBucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket '%s': %w", cfg.PublicAssetsBucketName, err)
	}

	return &BackblazeClient{
		client:             client,
		bucket:             bucket,
		publicAssetsBucket: publicAssetsBucket,
	}, nil
}

func (b *BackblazeClient) UploadDocument(ctx context.Context, key string, body io.ReadSeeker, contentType string) error {
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return err
	}

	obj := b.bucket.Object(key)
	w := obj.NewWriter(ctx)
	w.ConcurrentUploads = 1
	w.WithAttrs(&b2.Attrs{
		ContentType: contentType,
	})

	if _, err := io.Copy(w, body); err != nil {
		w.Close()
		return fmt.Errorf("failed to upload to B2: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}

	return nil
}

func (b *BackblazeClient) UploadProfilePicture(ctx context.Context, key string, body io.ReadSeeker, contentType string) error {
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return err
	}

	obj := b.publicAssetsBucket.Object(key)
	w := obj.NewWriter(ctx)
	w.ConcurrentUploads = 1
	w.WithAttrs(&b2.Attrs{
		ContentType: contentType,
	})

	if _, err := io.Copy(w, body); err != nil {
		w.Close()
		return fmt.Errorf("failed to upload to B2: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}

	return nil
}

func (b *BackblazeClient) ProfilePictureURL(key string) string {
	return fmt.Sprintf("%s/file/%s/%s", b.publicAssetsBucket.BaseURL(), b.publicAssetsBucket.Name(), key)
}

func (b *BackblazeClient) FileURL(key string) string {
	return fmt.Sprintf("%s/file/%s/%s", b.bucket.BaseURL(), b.bucket.Name(), key)
}

func (b *BackblazeClient) PresignURL(ctx context.Context, filePath string, ttl time.Duration) (string, error) {
	obj := b.bucket.Object(filePath)
	url, err := obj.AuthURL(ctx, ttl, "")
	if err != nil {
		return "", fmt.Errorf("failed to generate auth URL: %w", err)
	}
	return url.String(), nil
}
