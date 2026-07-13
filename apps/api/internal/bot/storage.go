package bot

import (
	"context"
	"io"
	"strings"

	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

// Storage abstracts media file upload and image-cache cleanup for the bot
// pipeline.
type Storage interface {
	// Upload stores the payload under filename, which is used verbatim as
	// the object key (and therefore appears in the public URL). Callers own
	// key construction, including namespace prefixes like images/ or bot/.
	Upload(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (url string, err error)
	// DeleteByURL deletes the image-cache object the URL points to. URLs
	// outside our storage's public URL space, and keys outside the image
	// cache namespace, are never deleted and are reported as success — the
	// adapter is the single enforcement point keeping cleanup scoped to
	// cache objects (spec: 미참조가 된 이미지 캐시 객체는 처리 경로에서 정리된다).
	DeleteByURL(ctx context.Context, url string) error
}

// StorageAdapter wraps storage.Client to satisfy the bot Storage interface.
type StorageAdapter struct {
	client *storage.Client
}

func NewStorageAdapter(client *storage.Client) *StorageAdapter {
	return &StorageAdapter{client: client}
}

// Upload stores the payload under the caller-provided key. The bot pipeline
// constructs namespaced keys (images/ cache, bot/ media), so the key must be
// respected — storage.Client.Upload would discard it and generate its own.
func (a *StorageAdapter) Upload(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
	result, err := a.client.UploadWithKey(ctx, filename, contentType, size, body)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (a *StorageAdapter) DeleteByURL(ctx context.Context, url string) error {
	key, ok := a.client.KeyFromURL(url)
	if !ok {
		return nil
	}
	// Prefix must include the "/" separator: the bare constant would also
	// match sibling namespaces like "imagesfoo/...".
	if !strings.HasPrefix(key, imageCacheKeyPrefix+"/") {
		return nil
	}
	return a.client.Delete(ctx, key)
}

// MockStorage is a mock implementation of Storage for testing.
type MockStorage struct {
	UploadFunc      func(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error)
	CallCount       int
	DeleteByURLFunc func(ctx context.Context, url string) error
	DeletedURLs     []string
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		UploadFunc: func(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
			return "https://cdn.example.com/media/" + filename, nil
		},
		DeleteByURLFunc: func(ctx context.Context, url string) error {
			return nil
		},
	}
}

func (m *MockStorage) Upload(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
	m.CallCount++
	return m.UploadFunc(ctx, filename, contentType, size, body)
}

func (m *MockStorage) DeleteByURL(ctx context.Context, url string) error {
	m.DeletedURLs = append(m.DeletedURLs, url)
	return m.DeleteByURLFunc(ctx, url)
}
