package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type MediaType string

const (
	MediaImage MediaType = "image"
	MediaAudio MediaType = "audio"
	MediaVideo MediaType = "video"
)

var allowedMIME = map[string]MediaType{
	"image/jpeg": MediaImage,
	"image/png":  MediaImage,
	"image/gif":  MediaImage,
	"image/webp": MediaImage,
	"audio/mpeg": MediaAudio,
	"audio/wav":  MediaAudio,
	"audio/ogg":  MediaAudio,
	"audio/flac": MediaAudio,
	"video/mp4":  MediaVideo,
	"video/webm": MediaVideo,
}

// mimeAliases maps common client-side variants to the canonical MIME used by
// http.DetectContentType. Keys must be lowercase.
var mimeAliases = map[string]string{
	"image/jpg":    "image/jpeg",
	"image/pjpeg":  "image/jpeg",
	"audio/x-wav":  "audio/wav",
	"audio/wave":   "audio/wav",
	"audio/mp3":    "audio/mpeg",
	"audio/x-flac": "audio/flac",
}

// normalizeMIME lowercases and resolves known client-side aliases so that
// declared and sniffed content types can be compared on equal footing.
func normalizeMIME(mime string) string {
	lower := strings.ToLower(strings.TrimSpace(mime))
	if canonical, ok := mimeAliases[lower]; ok {
		return canonical
	}
	return lower
}

// MaxBytes per media type.
var maxBytes = map[MediaType]int64{
	MediaImage: 10 << 20,  // 10 MB
	MediaAudio: 50 << 20,  // 50 MB
	MediaVideo: 100 << 20, // 100 MB
}

type Client struct {
	s3     *s3.Client
	bucket string
	pubURL string // public base URL for accessing objects
}

type Config struct {
	Endpoint  string // e.g. "http://localhost:9000"
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	PublicURL string // e.g. "http://localhost:9000/fugue-media"
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	resolver := aws.EndpointResolverWithOptionsFunc( //nolint:staticcheck // TODO: migrate to service-specific endpoint resolver
		func(service, region string, options ...interface{}) (aws.Endpoint, error) { //nolint:staticcheck
			if cfg.Endpoint != "" {
				return aws.Endpoint{ //nolint:staticcheck
					URL:               cfg.Endpoint,
					HostnameImmutable: true,
				}, nil
			}
			return aws.Endpoint{}, &aws.EndpointNotFoundError{} //nolint:staticcheck
		},
	)

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
		config.WithEndpointResolverWithOptions(resolver), //nolint:staticcheck
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true // required for MinIO
	})

	return &Client{
		s3:     s3Client,
		bucket: cfg.Bucket,
		pubURL: strings.TrimRight(cfg.PublicURL, "/"),
	}, nil
}

// S3Client exposes the underlying AWS SDK client for callers that need
// to drive object storage directly (for example, the bot/snapshot
// package's S3-backed SnapshotStore). Returning the SDK type rather
// than re-wrapping it avoids spinning up duplicate clients/credentials.
func (c *Client) S3Client() *s3.Client { return c.s3 }

// Bucket returns the configured bucket name.
func (c *Client) Bucket() string { return c.bucket }

// UploadResult holds info about a successful upload.
type UploadResult struct {
	Key       string    // S3 object key
	URL       string    // public URL
	MediaType MediaType // image, audio, video
}

// Upload validates and stores a media file. It reads the first 512 bytes
// to detect the real content type, validates against the allowlist, checks
// size, then streams to S3.
func (c *Client) Upload(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (*UploadResult, error) {
	// Detect real MIME from first 512 bytes
	buf := make([]byte, 512)
	n, err := io.ReadAtLeast(body, buf, 1)
	if err != nil {
		return nil, fmt.Errorf("storage: read header: %w", err)
	}
	detected := http.DetectContentType(buf[:n])

	// spec: pin `MIME 타입 위조 방지는 storage 레이어에서 declared와 sniff의 불일치 거부로 enforce된다`
	// 클라이언트가 표기한 Content-Type(declared)이 실제 파일 sniff 결과와 다르면 외부 저장소
	// 쓰기 전에 거부한다. declared가 비어 있거나 generic octet-stream이면 비교를 skip.
	if contentType != "" && contentType != "application/octet-stream" &&
		normalizeMIME(contentType) != normalizeMIME(detected) {
		return nil, fmt.Errorf("storage: unsupported file type: content type mismatch (declared=%q sniffed=%q)", contentType, detected)
	}

	// Normalize: use detected type, but if it's generic octet-stream
	// fall back to the declared content-type. Resolve client-side aliases
	// (e.g. http.DetectContentType returns "audio/wave" for WAV files,
	// but the allowlist canonical form is "audio/wav") so the allowlist
	// lookup operates on canonical MIME strings.
	mime := normalizeMIME(detected)
	if mime == "application/octet-stream" && contentType != "" {
		mime = normalizeMIME(contentType)
	}

	mt, ok := allowedMIME[mime]
	if !ok {
		return nil, fmt.Errorf("storage: unsupported file type: %s", mime)
	}

	limit := maxBytes[mt]
	if size > limit {
		return nil, fmt.Errorf("storage: file too large: %d bytes (max %d for %s)", size, limit, mt)
	}

	// Build the S3 key: <mediatype>/<uuid>.<ext>
	ext := extensionForMIME(mime)
	key := fmt.Sprintf("%s/%s%s", mt, uuid.New().String(), ext)

	// Read remaining body and combine with header bytes into a seekable reader.
	// bytes.Reader implements io.ReadSeeker, which AWS SDK needs for checksum calculation.
	rest, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("storage: read body: %w", err)
	}
	full := append(buf[:n], rest...)

	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(full),
		ContentType:   aws.String(mime),
		ContentLength: aws.Int64(int64(len(full))),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: s3 put: %w", err)
	}

	return &UploadResult{
		Key:       key,
		URL:       c.pubURL + "/" + key,
		MediaType: mt,
	}, nil
}

// Delete removes an object by key. Deleting a nonexistent key succeeds
// (S3 DeleteObject semantics), so the call is idempotent.
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("storage: s3 delete: %w", err)
	}
	return nil
}

func extensionForMIME(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav":
		return ".wav"
	case "audio/ogg":
		return ".ogg"
	case "audio/flac":
		return ".flac"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	default:
		return path.Ext(mime)
	}
}
