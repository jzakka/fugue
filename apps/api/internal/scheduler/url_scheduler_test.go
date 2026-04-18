package scheduler

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// ---------------------------------------------------------------------------
// Unit tests: 6.8 — unknown errorKind is rejected. These exercise the pure
// validateErrorKind helper so they do not depend on PGURLScheduler fields
// (no nil-DB pointer aliasing that could silently regress if the impl order
// of "validate enum then touch DB" ever flips).
// ---------------------------------------------------------------------------

func TestValidateErrorKind_AcceptsEnum(t *testing.T) {
	for _, k := range []ErrorKind{ErrorHTTP4xx, ErrorHTTP5xx, ErrorNetwork, ErrorTimeout} {
		if err := validateErrorKind(k); err != nil {
			t.Errorf("validateErrorKind(%q) returned %v, want nil", k, err)
		}
	}
}

func TestValidateErrorKind_RejectsOthers(t *testing.T) {
	for _, k := range []ErrorKind{"", "unknown", "HTTP_4XX", "503", "4xx", "fetch_failed"} {
		err := validateErrorKind(k)
		if err == nil || !errors.Is(err, ErrUnknownErrorKind) {
			t.Errorf("validateErrorKind(%q) = %v, want ErrUnknownErrorKind", k, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Integration tests: 5.2, 5.3, 6.3, 6.4, 6.5, 6.7.
// Gated on TEST_DATABASE_URL; they run only when an operator provides a DSN.
// This mirrors the lightweight test strategy used elsewhere in this module —
// the repo does not ship a testcontainer harness.
// ---------------------------------------------------------------------------

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping scheduler integration test")
	}
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}
	// Registered FIRST so it runs LAST (t.Cleanup is LIFO). Subsequent
	// t.Cleanup registrations in the test body (e.g. purgeByHost) execute
	// before this Close, so DELETE statements still see an open connection.
	// This fixes the ordering bug where `defer sqlDB.Close()` in each test
	// would close the DB before test-scoped cleanups ran.
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

// seedPioneerRow inserts a single pioneer_frontier row with the given error
// count and returns the normalized URL so the caller can address it via the
// scheduler API. The row gets a unique hash per test by including the test
// name in the URL.
func seedPioneerRow(t *testing.T, sqlDB *sql.DB, count int32) string {
	t.Helper()
	url := "https://example.test/" + t.Name() + "/" + uuid.NewString()
	h := sha256.Sum256([]byte(url))
	_, err := sqlDB.Exec(`
		INSERT INTO pioneer_frontier (normalized_url, url, url_hash, host, depth, score, next_fetch_at, fetch_error_count)
		VALUES ($1, $1, $2, 'example.test', 0, 0.0, now(), $3)
		ON CONFLICT (url_hash) DO UPDATE SET fetch_error_count = EXCLUDED.fetch_error_count
	`, url, h[:], count)
	if err != nil {
		t.Fatalf("seed pioneer_frontier: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec("DELETE FROM pioneer_frontier WHERE url_hash = $1", h[:])
	})
	return url
}

func seedHarvesterRow(t *testing.T, sqlDB *sql.DB, count int32) string {
	t.Helper()
	url := "https://example.test/harvest/" + t.Name() + "/" + uuid.NewString()
	h := sha256.Sum256([]byte(url))
	_, err := sqlDB.Exec(`
		INSERT INTO harvester_frontier (normalized_url, url, url_hash, host, score, next_harvest_at, harvest_error_count)
		VALUES ($1, $1, $2, 'example.test', 0.0, now(), $3)
		ON CONFLICT (url_hash) DO UPDATE SET harvest_error_count = EXCLUDED.harvest_error_count
	`, url, h[:], count)
	if err != nil {
		t.Fatalf("seed harvester_frontier: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec("DELETE FROM harvester_frontier WHERE url_hash = $1", h[:])
	})
	return url
}

// readPioneer returns (fetch_error_count, next_fetch_at) for the given URL.
func readPioneer(t *testing.T, sqlDB *sql.DB, url string) (int32, time.Time) {
	t.Helper()
	h := sha256.Sum256([]byte(url))
	var count int32
	var next time.Time
	err := sqlDB.QueryRow(`SELECT fetch_error_count, next_fetch_at FROM pioneer_frontier WHERE url_hash = $1`, h[:]).Scan(&count, &next)
	if err != nil {
		t.Fatalf("read pioneer: %v", err)
	}
	return count, next
}

// readPioneerLastUpdated returns last_updated_at separately; used to verify the
// spec requirement that every failure report refreshes that column.
func readPioneerLastUpdated(t *testing.T, sqlDB *sql.DB, url string) time.Time {
	t.Helper()
	h := sha256.Sum256([]byte(url))
	var ts time.Time
	if err := sqlDB.QueryRow(`SELECT last_updated_at FROM pioneer_frontier WHERE url_hash = $1`, h[:]).Scan(&ts); err != nil {
		t.Fatalf("read pioneer last_updated_at: %v", err)
	}
	return ts
}

func readHarvester(t *testing.T, sqlDB *sql.DB, url string) (int32, time.Time) {
	t.Helper()
	h := sha256.Sum256([]byte(url))
	var count int32
	var next time.Time
	err := sqlDB.QueryRow(`SELECT harvest_error_count, next_harvest_at FROM harvester_frontier WHERE url_hash = $1`, h[:]).Scan(&count, &next)
	if err != nil {
		t.Fatalf("read harvester: %v", err)
	}
	return count, next
}

func readHarvesterLastUpdated(t *testing.T, sqlDB *sql.DB, url string) time.Time {
	t.Helper()
	h := sha256.Sum256([]byte(url))
	var ts time.Time
	if err := sqlDB.QueryRow(`SELECT last_updated_at FROM harvester_frontier WHERE url_hash = $1`, h[:]).Scan(&ts); err != nil {
		t.Fatalf("read harvester last_updated_at: %v", err)
	}
	return ts
}

// 6.3 + spec.md:48-50: 4xx immediately sets count to 5 AND does not modify
// next_fetch_at. 5.2: dead row excluded from the claimable partial index.
func TestIntegration_RecordFetchError_4xxImmediateDead(t *testing.T) {
	sqlDB := openTestDB(t)
	s := NewPGURLScheduler(sqlDB)

	url := seedPioneerRow(t, sqlDB, 0)
	beforeCount, beforeNext := readPioneer(t, sqlDB, url)
	if beforeCount != 0 {
		t.Fatalf("precondition: count=0, got %d", beforeCount)
	}

	if err := s.RecordFetchError(url, ErrorHTTP4xx); err != nil {
		t.Fatalf("RecordFetchError: %v", err)
	}

	afterCount, afterNext := readPioneer(t, sqlDB, url)
	if afterCount != 5 {
		t.Fatalf("4xx dead: expected count=5, got %d", afterCount)
	}
	// spec.md:48-50: next_fetch_at is NOT updated by the 4xx path.
	if !afterNext.Equal(beforeNext) {
		t.Errorf("4xx path mutated next_fetch_at: before=%s after=%s", beforeNext, afterNext)
	}

	// Claim-side verification via the partial index condition.
	h := sha256.Sum256([]byte(url))
	var claimable int
	err := sqlDB.QueryRow(`SELECT count(*) FROM pioneer_frontier WHERE url_hash = $1 AND fetch_error_count < 5`, h[:]).Scan(&claimable)
	if err != nil {
		t.Fatalf("claimable probe: %v", err)
	}
	if claimable != 0 {
		t.Fatalf("dead row still satisfies claim condition: %d", claimable)
	}
}

// 6.4: five 5xx failures produce counts 1..5 with delays 30/60/120/240/480s
// (±10%). After the fifth, the row is excluded from the claimable index.
func TestIntegration_RecordFetchError_FiveConsecutiveBackoffs(t *testing.T) {
	sqlDB := openTestDB(t)
	s := NewPGURLScheduler(sqlDB)

	url := seedPioneerRow(t, sqlDB, 0)
	expectedDelays := []time.Duration{
		30 * time.Second, 60 * time.Second, 120 * time.Second,
		240 * time.Second, 480 * time.Second,
	}
	for i, d := range expectedDelays {
		callStart := time.Now()
		if err := s.RecordFetchError(url, ErrorHTTP5xx); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
		count, next := readPioneer(t, sqlDB, url)
		wantCount := int32(i + 1)
		if count != wantCount {
			t.Errorf("call %d: count = %d, want %d", i+1, count, wantCount)
		}
		gap := next.Sub(callStart)
		// The ±10% envelope accounts for scheduler jitter. Test runtime between
		// callStart and the actual clock.Now() inside recordError adds at most
		// a few milliseconds on top, which is dwarfed by 10% of d.
		low := time.Duration(float64(d) * 0.9)
		high := time.Duration(float64(d) * 1.1)
		if gap < low || gap > high {
			t.Errorf("call %d: next_fetch_at gap = %s, want [%s, %s]", i+1, gap, low, high)
		}
	}
	// 5.2: dead row excluded.
	h := sha256.Sum256([]byte(url))
	var claimable int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM pioneer_frontier WHERE url_hash = $1 AND fetch_error_count < 5`, h[:]).Scan(&claimable); err != nil {
		t.Fatalf("claimable probe: %v", err)
	}
	if claimable != 0 {
		t.Fatalf("after 5 failures, row still claimable")
	}
}

// 6.5: network and timeout apply the same formula as http_5xx.
// Seeds cover counts 0, 1, and 3 so the next increment lands on 1, 2, and 4,
// verifying that the same 30s * 2^(n-1) formula applies at non-trivial counts
// (task 6.5 "http_5xx와 동일한 공식" is a function-of-count assertion, not just
// a single-point check).
func TestIntegration_RecordFetchError_NetworkAndTimeoutFormula(t *testing.T) {
	sqlDB := openTestDB(t)
	s := NewPGURLScheduler(sqlDB)

	type step struct {
		seedCount int32
		wantCount int32
		wantDelay time.Duration
	}
	steps := []step{
		{seedCount: 0, wantCount: 1, wantDelay: 30 * time.Second},
		{seedCount: 1, wantCount: 2, wantDelay: 60 * time.Second},
		{seedCount: 3, wantCount: 4, wantDelay: 240 * time.Second},
	}
	kinds := []ErrorKind{ErrorNetwork, ErrorTimeout}
	for _, kind := range kinds {
		kind := kind
		for _, st := range steps {
			st := st
			t.Run(string(kind)+"_n"+strconv.Itoa(int(st.wantCount)), func(t *testing.T) {
				url := seedPioneerRow(t, sqlDB, st.seedCount)
				callStart := time.Now()
				if err := s.RecordFetchError(url, kind); err != nil {
					t.Fatalf("RecordFetchError(%s): %v", kind, err)
				}
				count, next := readPioneer(t, sqlDB, url)
				if count != st.wantCount {
					t.Errorf("count = %d, want %d", count, st.wantCount)
				}
				gap := next.Sub(callStart)
				low := time.Duration(float64(st.wantDelay) * 0.9)
				high := time.Duration(float64(st.wantDelay) * 1.1)
				if gap < low || gap > high {
					t.Errorf("gap = %s, want %s ±10%% [%s, %s]", gap, st.wantDelay, low, high)
				}
			})
		}
	}
}

// 6.7: harvest side mirrors fetch side for 4xx and 5xx.
func TestIntegration_RecordHarvestError_4xxAndBackoff(t *testing.T) {
	sqlDB := openTestDB(t)
	s := NewPGURLScheduler(sqlDB)

	t.Run("4xx_immediate_dead", func(t *testing.T) {
		url := seedHarvesterRow(t, sqlDB, 0)
		_, beforeNext := readHarvester(t, sqlDB, url)

		if err := s.RecordHarvestError(url, ErrorHTTP4xx); err != nil {
			t.Fatalf("RecordHarvestError: %v", err)
		}
		count, afterNext := readHarvester(t, sqlDB, url)
		if count != 5 {
			t.Errorf("count = %d, want 5", count)
		}
		// spec.md:52-54: harvester 4xx path also preserves next_harvest_at.
		if !afterNext.Equal(beforeNext) {
			t.Errorf("4xx path mutated next_harvest_at: before=%s after=%s", beforeNext, afterNext)
		}
	})

	t.Run("5xx_backoff", func(t *testing.T) {
		url := seedHarvesterRow(t, sqlDB, 1)
		callStart := time.Now()
		if err := s.RecordHarvestError(url, ErrorHTTP5xx); err != nil {
			t.Fatalf("RecordHarvestError: %v", err)
		}
		count, next := readHarvester(t, sqlDB, url)
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
		gap := next.Sub(callStart)
		if gap < 54*time.Second || gap > 66*time.Second {
			t.Errorf("gap = %s, want 60s ±10%%", gap)
		}
	})
}

// 5.3 / claim-api Scenario "알 수 없는 key": warn log, no row created, no panic.
func TestIntegration_RecordFetchError_UnknownKeyWarnsNoRow(t *testing.T) {
	sqlDB := openTestDB(t)
	s := NewPGURLScheduler(sqlDB)

	url := "https://example.test/unknown-key/" + uuid.NewString()
	h := sha256.Sum256([]byte(url))

	// Precondition: row absent.
	var before int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM pioneer_frontier WHERE url_hash = $1`, h[:]).Scan(&before); err != nil {
		t.Fatalf("pre count: %v", err)
	}
	if before != 0 {
		t.Fatalf("test setup: row already present")
	}

	if err := s.RecordFetchError(url, ErrorHTTP5xx); err != nil {
		t.Fatalf("RecordFetchError should not return error for unknown key, got: %v", err)
	}

	var after int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM pioneer_frontier WHERE url_hash = $1`, h[:]).Scan(&after); err != nil {
		t.Fatalf("post count: %v", err)
	}
	if after != 0 {
		t.Fatalf("no row should be created for unknown key, got %d", after)
	}
}

// 6.8: unknown errorKind does not mutate the row (integration half).
// Task 6.8 requires "양쪽 모두" — fetch and harvest — so both sides get their
// own integration test covering the row-invariance guarantee, not only the
// shared validateErrorKind unit coverage.
func TestIntegration_RecordFetchError_UnknownKindLeavesRowIntact(t *testing.T) {
	sqlDB := openTestDB(t)
	s := NewPGURLScheduler(sqlDB)

	url := seedPioneerRow(t, sqlDB, 2)
	beforeCount, beforeNext := readPioneer(t, sqlDB, url)

	err := s.RecordFetchError(url, "bogus")
	if err == nil || !errors.Is(err, ErrUnknownErrorKind) {
		t.Fatalf("expected ErrUnknownErrorKind, got %v", err)
	}
	afterCount, afterNext := readPioneer(t, sqlDB, url)
	if afterCount != beforeCount {
		t.Errorf("count changed: %d -> %d", beforeCount, afterCount)
	}
	if !afterNext.Equal(beforeNext) {
		t.Errorf("next_fetch_at changed: %s -> %s", beforeNext, afterNext)
	}
}

// spec.md Requirement "실패 보고는 last_updated_at을 현재 시각으로 갱신한다":
// every non-error path (4xx dead + non-4xx backoff, both pioneer and harvester)
// must advance last_updated_at past the pre-call wall clock. Row invariance
// otherwise (unknown errorKind) is covered by the _UnknownKindLeavesRowIntact
// tests above and is not re-asserted here.
func TestIntegration_RecordError_UpdatesLastUpdatedAt(t *testing.T) {
	sqlDB := openTestDB(t)
	s := NewPGURLScheduler(sqlDB)

	type recordFn func(url string) error
	type readFn func(url string) time.Time
	cases := []struct {
		name    string
		seed    func() string
		record  recordFn
		read    readFn
		errKind ErrorKind
	}{
		{
			name:    "pioneer_4xx",
			seed:    func() string { return seedPioneerRow(t, sqlDB, 0) },
			record:  func(url string) error { return s.RecordFetchError(url, ErrorHTTP4xx) },
			read:    func(url string) time.Time { return readPioneerLastUpdated(t, sqlDB, url) },
			errKind: ErrorHTTP4xx,
		},
		{
			name:    "pioneer_5xx",
			seed:    func() string { return seedPioneerRow(t, sqlDB, 0) },
			record:  func(url string) error { return s.RecordFetchError(url, ErrorHTTP5xx) },
			read:    func(url string) time.Time { return readPioneerLastUpdated(t, sqlDB, url) },
			errKind: ErrorHTTP5xx,
		},
		{
			name:    "harvester_4xx",
			seed:    func() string { return seedHarvesterRow(t, sqlDB, 0) },
			record:  func(url string) error { return s.RecordHarvestError(url, ErrorHTTP4xx) },
			read:    func(url string) time.Time { return readHarvesterLastUpdated(t, sqlDB, url) },
			errKind: ErrorHTTP4xx,
		},
		{
			name:    "harvester_5xx",
			seed:    func() string { return seedHarvesterRow(t, sqlDB, 0) },
			record:  func(url string) error { return s.RecordHarvestError(url, ErrorHTTP5xx) },
			read:    func(url string) time.Time { return readHarvesterLastUpdated(t, sqlDB, url) },
			errKind: ErrorHTTP5xx,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			url := tc.seed()
			before := tc.read(url)
			// Sleep a tick so a fast clock can't produce before == after
			// purely from same-microsecond execution; the spec only requires
			// "current time" monotonicity, not nanosecond granularity.
			time.Sleep(10 * time.Millisecond)
			if err := tc.record(url); err != nil {
				t.Fatalf("%s: %v", tc.errKind, err)
			}
			after := tc.read(url)
			if !after.After(before) {
				t.Errorf("%s: last_updated_at not advanced (before=%s after=%s)", tc.errKind, before, after)
			}
		})
	}
}

// Edge-case documented on url_scheduler.go:186-195: a non-4xx report against
// an already-dead row (count=5) keeps count idempotent via LEAST(6,5)=5 but
// does overwrite next_fetch_at with a fresh candidate. This test pins that
// behavior so a future refactor that "fixes" it (e.g., adds a guard) must
// also update the comment / spec reasoning. The dead row stays excluded from
// the claim partial index regardless, so no downstream consumer observes the
// rewritten timestamp.
func TestIntegration_RecordFetchError_DeadRowNon4xxIsIdempotentOnCount(t *testing.T) {
	sqlDB := openTestDB(t)
	s := NewPGURLScheduler(sqlDB)

	url := seedPioneerRow(t, sqlDB, 5)
	if err := s.RecordFetchError(url, ErrorHTTP5xx); err != nil {
		t.Fatalf("RecordFetchError: %v", err)
	}
	count, _ := readPioneer(t, sqlDB, url)
	if count != 5 {
		t.Errorf("count = %d, want 5 (LEAST(6,5) idempotent)", count)
	}

	// The dead row must remain excluded from the claim partial index.
	h := sha256.Sum256([]byte(url))
	var claimable int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM pioneer_frontier WHERE url_hash = $1 AND fetch_error_count < 5`, h[:]).Scan(&claimable); err != nil {
		t.Fatalf("claimable probe: %v", err)
	}
	if claimable != 0 {
		t.Fatalf("dead row became claimable: %d", claimable)
	}
}

func TestIntegration_RecordHarvestError_UnknownKindLeavesRowIntact(t *testing.T) {
	sqlDB := openTestDB(t)
	s := NewPGURLScheduler(sqlDB)

	url := seedHarvesterRow(t, sqlDB, 2)
	beforeCount, beforeNext := readHarvester(t, sqlDB, url)

	err := s.RecordHarvestError(url, "bogus")
	if err == nil || !errors.Is(err, ErrUnknownErrorKind) {
		t.Fatalf("expected ErrUnknownErrorKind, got %v", err)
	}
	afterCount, afterNext := readHarvester(t, sqlDB, url)
	if afterCount != beforeCount {
		t.Errorf("count changed: %d -> %d", beforeCount, afterCount)
	}
	if !afterNext.Equal(beforeNext) {
		t.Errorf("next_harvest_at changed: %s -> %s", beforeNext, afterNext)
	}
}
