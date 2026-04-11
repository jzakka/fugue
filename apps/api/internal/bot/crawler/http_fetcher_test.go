package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPFetcher_Success(t *testing.T) {
	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body><h1>Test</h1></body></html>"))
	}))
	defer ts.Close()

	fetcher := NewHTTPFetcher(nil)
	result, err := fetcher.Fetch(context.Background(), ts.URL)

	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	defer func() { _ = result.Body.Close() }()

	if result.ContentType != "text/html" {
		t.Errorf("Expected content type text/html, got %s", result.ContentType)
	}
}

func TestHTTPFetcher_NotFound(t *testing.T) {
	// Create test server that returns 404
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	fetcher := NewHTTPFetcher(nil)
	_, err := fetcher.Fetch(context.Background(), ts.URL)

	if err == nil {
		t.Fatal("Expected error for 404, got nil")
	}
}

func TestHTTPFetcher_CustomClient(t *testing.T) {
	customClient := &http.Client{}
	fetcher := NewHTTPFetcher(customClient)

	if fetcher.client != customClient {
		t.Error("Custom client was not used")
	}
}
