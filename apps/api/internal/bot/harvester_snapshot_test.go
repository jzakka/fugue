// harvester_snapshot_test.go covers tasks 4.7 and 4.8 of
// harvester-snapshot-first-fetch: end-to-end Harvester flow with a
// CompositeFetcher injected.

package bot

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// mapFetcher dispatches Fetch requests by URL and records per-URL call
// counts. Used to inject deterministic bodies and assert that specific
// URLs did (or did not) hit the fetch path.
type mapFetcher struct {
	bodies map[string][]byte
	errs   map[string]error
	calls  map[string]int
}

func newMapFetcher() *mapFetcher {
	return &mapFetcher{
		bodies: map[string][]byte{},
		errs:   map[string]error{},
		calls:  map[string]int{},
	}
}

func (f *mapFetcher) Fetch(rawURL string) ([]byte, error) {
	f.calls[rawURL]++
	if err, ok := f.errs[rawURL]; ok && err != nil {
		return nil, err
	}
	if body, ok := f.bodies[rawURL]; ok {
		return body, nil
	}
	return nil, fmt.Errorf("mapFetcher: no entry for %q", rawURL)
}

func (f *mapFetcher) totalCalls() int {
	n := 0
	for _, c := range f.calls {
		n += c
	}
	return n
}

// Task 4.7: a single-node double failure increments HarvestErrorCount by
// exactly 1 and does NOT abort the run — subsequent nodes are still
// processed.
func TestHarvester_DoubleFailureIncrementsHarvestErrorCountAndContinues(t *testing.T) {
	ok := pinnableHTML("OK")

	rootURL := "https://site.test/root"
	failURL := "https://site.test/broken"
	tailURL := "https://site.test/tail"

	// Composite: object-storage layer misses everything, HTTP layer
	// succeeds except for failURL where it also errors — simulating the
	// double-failure case.
	snap := newMapFetcher()
	for _, u := range []string{rootURL, failURL, tailURL} {
		snap.errs[u] = errors.New("no snapshot")
	}
	httpF := newMapFetcher()
	httpF.bodies[rootURL] = []byte(ok)
	httpF.bodies[tailURL] = []byte(ok)
	httpF.errs[failURL] = errors.New("502 bad gateway")

	composite := NewCompositeFetcher(snap, httpF)

	siteRepo := NewMockSiteRepository()
	graphRepo := NewMockGraphRepository()
	site := seedGraph(t, siteRepo, graphRepo, rootURL, []string{failURL, tailURL})

	pipeline := &togglePipeline{
		onProcess: func(_ db.BotGraphNode, _ PinDocument) (bool, uuid.UUID, error) {
			return true, uuid.New(), nil
		},
	}
	h := buildHarvester(graphRepo, siteRepo, pipeline, nil).WithFetcher(composite)

	stats, err := h.Run(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if stats.NodesProcessed != 3 {
		t.Errorf("NodesProcessed = %d, want 3 (failure must not abort)", stats.NodesProcessed)
	}
	if stats.HarvestErrorCount != 1 {
		t.Errorf("HarvestErrorCount = %d, want 1 (exactly one double failure)", stats.HarvestErrorCount)
	}
	if stats.Failed != 1 {
		t.Errorf("Failed = %d, want 1", stats.Failed)
	}
	// Other nodes still got persisted.
	if stats.PinsCreated != 2 {
		t.Errorf("PinsCreated = %d, want 2 (root + tail)", stats.PinsCreated)
	}
	// Sentinel: failURL was tried by BOTH layers (snapshot first, then HTTP).
	if snap.calls[failURL] != 1 {
		t.Errorf("snapshot tried %d times for failURL, want 1", snap.calls[failURL])
	}
	if httpF.calls[failURL] != 1 {
		t.Errorf("http tried %d times for failURL, want 1", httpF.calls[failURL])
	}
}

// Task 4.8: on the snapshot-hit happy path, HTTP is never called. Proves
// the spec's "no external traffic on normal runs" guarantee.
func TestHarvester_SnapshotHitSkipsHTTPTrafficEntirely(t *testing.T) {
	body := []byte(pinnableHTML("OK"))
	rootURL := "https://site.test/root"
	childURL := "https://site.test/child"

	snap := newMapFetcher()
	snap.bodies[rootURL] = body
	snap.bodies[childURL] = body
	httpF := newMapFetcher() // no entries → any call returns "no entry" error

	composite := NewCompositeFetcher(snap, httpF)

	siteRepo := NewMockSiteRepository()
	graphRepo := NewMockGraphRepository()
	site := seedGraph(t, siteRepo, graphRepo, rootURL, []string{childURL})

	pipeline := &togglePipeline{
		onProcess: func(_ db.BotGraphNode, _ PinDocument) (bool, uuid.UUID, error) {
			return true, uuid.New(), nil
		},
	}
	h := buildHarvester(graphRepo, siteRepo, pipeline, nil).WithFetcher(composite)

	stats, err := h.Run(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if stats.NodesProcessed != 2 {
		t.Errorf("NodesProcessed = %d, want 2", stats.NodesProcessed)
	}
	if stats.HarvestErrorCount != 0 {
		t.Errorf("HarvestErrorCount = %d, want 0", stats.HarvestErrorCount)
	}
	if httpF.totalCalls() != 0 {
		t.Errorf("HTTP fetcher calls = %d, want 0 (snapshot-hit path must not touch HTTP)", httpF.totalCalls())
	}
	if snap.calls[rootURL] != 1 || snap.calls[childURL] != 1 {
		t.Errorf("expected each URL fetched once from snapshot; got %v", snap.calls)
	}
}
