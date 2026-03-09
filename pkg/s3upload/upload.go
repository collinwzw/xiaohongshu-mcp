package s3upload

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"
)

// Uploader handles downloading media from URLs and uploading to S3.
type Uploader struct {
	client *s3.Client
	bucket string
	prefix string // e.g. "xiaohongshu/media/"
}

// NewUploader creates an S3 uploader from environment variables.
// Required: S3_MEDIA_BUCKET (or S3_COOKIES_BUCKET as fallback)
// Optional: S3_MEDIA_PREFIX (default "xiaohongshu/media/")
func NewUploader() (*Uploader, error) {
	bucket := os.Getenv("S3_MEDIA_BUCKET")
	if bucket == "" {
		bucket = os.Getenv("S3_COOKIES_BUCKET") // reuse same bucket
	}
	if bucket == "" {
		return nil, fmt.Errorf("S3_MEDIA_BUCKET or S3_COOKIES_BUCKET must be set")
	}

	prefix := os.Getenv("S3_MEDIA_PREFIX")
	if prefix == "" {
		prefix = "xiaohongshu/media/"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Uploader{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
		prefix: prefix,
	}, nil
}

// UploadResult contains info about a successfully uploaded file.
type UploadResult struct {
	S3Key       string `json:"s3_key"`
	S3URL       string `json:"s3_url"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// DownloadAndUpload fetches a URL and uploads it to S3.
// filename is the desired filename (without path prefix).
func (u *Uploader) DownloadAndUpload(ctx context.Context, sourceURL, filename string) (*UploadResult, error) {
	// Download
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.xiaohongshu.com/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", sourceURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d for %s", resp.StatusCode, sourceURL)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Detect content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}

	// Build S3 key
	s3Key := u.prefix + filename

	// Upload to S3
	_, err = u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload to S3: %w", err)
	}

	s3URL := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", u.bucket, s3Key)

	logrus.Infof("Uploaded media to S3: %s (%d bytes, %s)", s3Key, len(data), contentType)

	return &UploadResult{
		S3Key:       s3Key,
		S3URL:       s3URL,
		ContentType: contentType,
		Size:        int64(len(data)),
	}, nil
}

// GuessFilename extracts a filename from a URL, falling back to a generated name.
func GuessFilename(sourceURL, fallbackPrefix string, index int) string {
	// Try to extract from URL path
	if u := strings.Split(sourceURL, "?"); len(u) > 0 {
		base := path.Base(u[0])
		if base != "" && base != "." && base != "/" {
			return base
		}
	}
	return fmt.Sprintf("%s_%d", fallbackPrefix, index)
}
