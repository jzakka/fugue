package storage

import (
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

	// Normalize: use detected type, but if it's generic octet-stream
	// fall back to the declared content-type.
	mime := detected
	if mime == "application/octet-stream" && contentType != "" {
		mime = contentType
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

	// Reassemble reader: buffered header + remaining body
	combined := io.MultiReader(strings.NewReader(string(buf[:n])), body)

	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          combined,
		ContentType:   aws.String(mime),
		ContentLength: aws.Int64(size),
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
