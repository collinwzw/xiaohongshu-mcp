package cookies

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// s3Cookie implements the Cookier interface using AWS S3 for persistence.
// This allows ECS Fargate containers (stateless) to persist cookies across restarts.
type s3Cookie struct {
	bucket string
	key    string
	client *s3.Client
}

// NewS3Cookie creates a new S3-backed cookie store.
// bucket: S3 bucket name (e.g. "oversky-prod-uploads")
// key: S3 object key (e.g. "xiaohongshu/cookies.json")
func NewS3Cookie(bucket, key string) (Cookier, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load AWS config")
	}

	client := s3.NewFromConfig(cfg)

	logrus.Infof("S3 cookie store initialized: bucket=%s key=%s", bucket, key)

	return &s3Cookie{
		bucket: bucket,
		key:    key,
		client: client,
	}, nil
}

// LoadCookies reads cookie data from S3.
func (c *s3Cookie) LoadCookies() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	output, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(c.key),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to read cookies from S3")
	}
	defer output.Body.Close()

	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read S3 response body")
	}

	logrus.Infof("Loaded cookies from S3: %s/%s (%d bytes)", c.bucket, c.key, len(data))
	return data, nil
}

// SaveCookies writes cookie data to S3.
func (c *s3Cookie) SaveCookies(data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(c.key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return errors.Wrap(err, "failed to save cookies to S3")
	}

	logrus.Infof("Saved cookies to S3: %s/%s (%d bytes)", c.bucket, c.key, len(data))
	return nil
}

// DeleteCookies removes the cookie object from S3.
func (c *s3Cookie) DeleteCookies() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(c.key),
	})
	if err != nil {
		return errors.Wrap(err, "failed to delete cookies from S3")
	}

	logrus.Infof("Deleted cookies from S3: %s/%s", c.bucket, c.key)
	return nil
}

// NewCookier creates the appropriate Cookier implementation based on environment.
// If S3_COOKIES_BUCKET is set, uses S3 backend (for ECS Fargate).
// Otherwise falls back to local file (for local development).
func NewCookier() Cookier {
	bucket := os.Getenv("S3_COOKIES_BUCKET")
	key := os.Getenv("S3_COOKIES_KEY")
	if key == "" {
		key = "xiaohongshu/cookies.json"
	}

	if bucket != "" {
		cookier, err := NewS3Cookie(bucket, key)
		if err != nil {
			logrus.Errorf("Failed to initialize S3 cookie store, falling back to local: %v", err)
			return NewLoadCookie(GetCookiesFilePath())
		}
		return cookier
	}

	return NewLoadCookie(GetCookiesFilePath())
}
