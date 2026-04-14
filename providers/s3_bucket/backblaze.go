package s3bucket

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type BackblazeConfig struct {
	KeyID      string `json:"key_id" binding:"required"`
	AppKey     string `json:"app_key" binding:"required"`
	BucketName string `json:"bucket_name" binding:"required"`
	Endpoint   string `json:"endpoint" binding:"required"`
}

type BackblazeClient struct {
	s3      *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func NewBackblazeClient(cfg BackblazeConfig) (*BackblazeClient, error) {
	creds := credentials.NewStaticCredentialsProvider(cfg.KeyID, cfg.AppKey, "")

	awsCfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithCredentialsProvider(creds),
		config.WithRegion("auto"),
	)

	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})

	return &BackblazeClient{
		s3:      client,
		presign: s3.NewPresignClient(client),
		bucket:  cfg.BucketName,
	}, nil
}

func (b *BackblazeClient) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	_, err = b.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(b.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(int64(len(data))),
	})

	return err
}

func (b *BackblazeClient) PresignURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := b.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))

	if err != nil {
		return "", err
	}

	return req.URL, nil
}
