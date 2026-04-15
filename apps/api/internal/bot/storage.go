package bot

import (
	"context"
	"io"

	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

// Storage abstracts media file upload for the bot pipeline.
type Storage interface {
	Upload(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (url string, err error)
}

// StorageAdapter wraps storage.Client to satisfy the bot Storage interface.
type StorageAdapter struct {
	client *storage.Client
}

func NewStorageAdapter(client *storage.Client) *StorageAdapter {
	return &StorageAdapter{client: client}
}

func (a *StorageAdapter) Upload(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
	result, err := a.client.Upload(ctx, filename, contentType, size, body)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// MockStorage is a mock implementation of Storage for testing.
type MockStorage struct {
	UploadFunc func(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error)
	CallCount  int
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		UploadFunc: func(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
			return "https://cdn.example.com/media/" + filename, nil
		},
	}
}

func (m *MockStorage) Upload(ctx context.Context, filename string, contentType string, size int64, body io.Reader) (string, error) {
	m.CallCount++
	return m.UploadFunc(ctx, filename, contentType, size, body)
}
