package snapshot

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type recordingStore struct {
	mu     sync.Mutex
	putErr error
	calls  []struct {
		url  string
		body []byte
	}
}

func (r *recordingStore) Put(_ context.Context, url string, body []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, struct {
		url  string
		body []byte
	}{url, append([]byte(nil), body...)})
	return r.putErr
}

type capturingLogger struct {
	mu    sync.Mutex
	calls []struct {
		url string
		key string
		err error
	}
}

func (c *capturingLogger) UploadFailed(_ context.Context, url, key string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, struct {
		url string
		key string
		err error
	}{url, key, err})
}

func TestSaver_Success_RecordsMetricsAndNoLog(t *testing.T) {
	t.Parallel()
	store := &recordingStore{}
	m := NewMetricsRecorder()
	log := &capturingLogger{}

	s := NewSaver(store, m, log)
	if err := s.SaveRawContent(context.Background(), "https://ex.com/p", []byte("body")); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if m.Success() != 1 || m.Failure() != 0 {
		t.Fatalf("metrics: success=%d failure=%d", m.Success(), m.Failure())
	}
	if m.Calls() != 1 {
		t.Fatalf("expected 1 duration observation, got %d", m.Calls())
	}
	if len(log.calls) != 0 {
		t.Fatalf("expected no upload-failed logs, got %d", len(log.calls))
	}
}

func TestSaver_EmptyBody_Skipped(t *testing.T) {
	t.Parallel()
	store := &recordingStore{}
	m := NewMetricsRecorder()
	s := NewSaver(store, m, nil)
	if err := s.SaveRawContent(context.Background(), "https://ex.com/p", nil); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(store.calls) != 0 {
		t.Fatal("empty body should not trigger store.Put")
	}
	if m.Success()+m.Failure() != 0 {
		t.Fatal("no upload metric should fire for empty body")
	}
}

func TestSaver_UploadError_FailOpenAndMetricsAndLog(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("storage boom")
	store := &recordingStore{putErr: sentinel}
	m := NewMetricsRecorder()
	log := &capturingLogger{}
	s := NewSaver(store, m, log)

	err := s.SaveRawContent(context.Background(), "https://ex.com/p", []byte("body"))
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if m.Success() != 0 || m.Failure() != 1 {
		t.Fatalf("metrics: success=%d failure=%d", m.Success(), m.Failure())
	}
	if len(log.calls) != 1 {
		t.Fatalf("expected 1 failure log, got %d", len(log.calls))
	}
	if log.calls[0].url != "https://ex.com/p" {
		t.Fatalf("log url: %q", log.calls[0].url)
	}
	if log.calls[0].key == "" {
		t.Fatal("log key should be populated")
	}
}

func TestSaver_NoopStore_NoError(t *testing.T) {
	t.Parallel()
	m := NewMetricsRecorder()
	s := NewSaver(NoopStore{}, m, nil)
	if err := s.SaveRawContent(context.Background(), "https://ex.com/p", []byte("body")); err != nil {
		t.Fatal(err)
	}
	if m.Success() != 1 {
		t.Fatalf("noop store should still record success, got %d", m.Success())
	}
}

// deterministic now for duration measurement.
func TestSaver_ObservesDuration(t *testing.T) {
	t.Parallel()
	store := &recordingStore{}
	m := NewMetricsRecorder()
	s := NewSaver(store, m, nil)

	calls := 0
	s.now = func() time.Time {
		calls++
		base := time.Unix(0, 0)
		if calls == 1 {
			return base
		}
		return base.Add(7 * time.Millisecond)
	}
	if err := s.SaveRawContent(context.Background(), "u", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if m.Calls() != 1 {
		t.Fatalf("expected 1 duration sample, got %d", m.Calls())
	}
}
