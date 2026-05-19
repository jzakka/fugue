package bot

import (
	"context"
	"strings"
	"testing"
)

// TestHarvestPipeline_SSRFWiring is the integration-level smoke test for the
// SSRF block on the cacheImage path. It exercises the real NewHarvestPipeline
// constructor to prove the wired client refuses to dial AWS IMDS, and that
// MockStorage is never asked to Upload a metadata response.
//
// The dialer-level coverage (every blocked range, redirect re-check, timeout
// configuration) lives in apps/api/internal/httpclient/ssrf_test.go. This
// test only verifies that the bot pipeline picks the SSRF-safe client up.
func TestHarvestPipeline_SSRFWiring_CacheImageRejectsIMDS(t *testing.T) {
	mockStorage := NewMockStorage()
	p := NewHarvestPipeline(nil, mockStorage)

	candidateURL := "http://169.254.169.254/latest/meta-data/iam/security-credentials/test-role"
	got, err := p.cacheImage(context.Background(), candidateURL)
	if err == nil {
		t.Fatal("expected error fetching AWS IMDS, got nil")
	}
	if !strings.Contains(err.Error(), "blocked private/reserved IP") {
		t.Errorf("error missing SSRF block marker: %v", err)
	}
	if got != candidateURL {
		t.Errorf("expected fallback to original URL %q, got %q", candidateURL, got)
	}
	if mockStorage.CallCount != 0 {
		t.Errorf("Upload should not be called on blocked fetch, got %d calls", mockStorage.CallCount)
	}
}

func TestHarvestPipeline_SSRFWiring_DownloadAndUploadRejectsPrivateIP(t *testing.T) {
	mockStorage := NewMockStorage()
	p := NewHarvestPipeline(nil, mockStorage)

	_, err := p.downloadAndUpload(context.Background(), "http://10.0.0.1/private.mp4", "video")
	if err == nil {
		t.Fatal("expected error fetching private IPv4, got nil")
	}
	if !strings.Contains(err.Error(), "blocked private/reserved IP") {
		t.Errorf("error missing SSRF block marker: %v", err)
	}
	if mockStorage.CallCount != 0 {
		t.Errorf("Upload should not be called on blocked fetch, got %d calls", mockStorage.CallCount)
	}
}
