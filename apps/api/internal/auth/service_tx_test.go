package auth

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// These tests pin the atomicity contract added to createNewCreator: the two
// INSERTs (creators + auth_accounts) MUST be wrapped in a transaction so a
// failure on the second INSERT (transient PG error or the auth_accounts
// UNIQUE(provider, provider_id) race when two simultaneous OAuth callbacks
// for the same no-email identity both pass the L44 GetAuthAccountByProvider
// sql.ErrNoRows check) rolls back the creators row instead of leaving an
// orphan creator that has no auth_account, no recovery path for the user,
// and pollutes the creators table.
//
// Mirrors the sibling pattern findOrCreateWithEmail (service.go L69-142)
// already uses for the email-bearing path. Gated on TEST_DATABASE_URL like
// the scheduler integration tests — the repo does not ship a testcontainer
// harness.

func openAuthTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping auth integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestIntegration_CreateNewCreator_RollbackOnAuthAccountFailure simulates the
// transient-failure scenario: addAuthAccount fails because the target
// (provider, provider_id) is already claimed by another creator. Before the
// fix, the new creators row would persist as an orphan; after the fix, the
// transaction rolls back and the creators table count is unchanged.
func TestIntegration_CreateNewCreator_RollbackOnAuthAccountFailure(t *testing.T) {
	db := openAuthTestDB(t)
	svc := &Service{db: db}

	providerID := "test-orphan-" + uuid.NewString()

	// Pre-seed a creator + auth_account that owns (provider='google',
	// provider_id=providerID) so the next addAuthAccount call collides on
	// the UNIQUE(provider, provider_id) constraint.
	var existingCreatorID uuid.UUID
	if err := db.QueryRow(
		`INSERT INTO creators (nickname) VALUES ($1) RETURNING id`,
		"orphan-test-existing",
	).Scan(&existingCreatorID); err != nil {
		t.Fatalf("seed creator: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM creators WHERE id = $1`, existingCreatorID)
	})

	if _, err := db.Exec(
		`INSERT INTO auth_accounts (creator_id, provider, provider_id) VALUES ($1, 'google', $2)`,
		existingCreatorID, providerID,
	); err != nil {
		t.Fatalf("seed auth_account: %v", err)
	}

	var before int
	if err := db.QueryRow(`SELECT COUNT(*) FROM creators`).Scan(&before); err != nil {
		t.Fatalf("count creators before: %v", err)
	}

	profile := &UserProfile{
		ProviderID: providerID,
		Nickname:   "orphan-test-new",
		AvatarURL:  "",
	}
	// Direct call to the unexported function — same-package test bypasses the
	// FindOrCreateCreator step-1 lookup so we exercise createNewCreator's
	// atomicity contract in isolation.
	_, err := svc.createNewCreator(context.Background(), profile, "google", "")
	if err == nil {
		t.Fatal("expected createNewCreator to fail with UNIQUE(provider, provider_id) violation, got nil")
	}

	var after int
	if err := db.QueryRow(`SELECT COUNT(*) FROM creators`).Scan(&after); err != nil {
		t.Fatalf("count creators after: %v", err)
	}

	if after != before {
		t.Errorf("createNewCreator failure left orphan creator row(s): before=%d after=%d (BeginTx rollback expected)", before, after)
		// Best-effort cleanup so a regression run doesn't pollute the table.
		_, _ = db.Exec(`DELETE FROM creators WHERE nickname = 'orphan-test-new'`)
	}
}

// TestIntegration_CreateNewCreator_RaceNoOrphan pins the concurrent-callback
// scenario: two simultaneous FindOrCreateCreator calls for the same no-email
// identity both pass the L44 GetAuthAccountByProvider sql.ErrNoRows check,
// both fall through to createNewCreator, both succeed at CreateCreatorFromOAuth
// (creators has no UNIQUE on nickname), but only one wins the
// auth_accounts UNIQUE(provider, provider_id) race. The loser's transaction
// MUST roll back the loser's creators row; without the fix, that row would
// remain orphan.
func TestIntegration_CreateNewCreator_RaceNoOrphan(t *testing.T) {
	db := openAuthTestDB(t)
	svc := &Service{db: db}

	providerID := "test-race-" + uuid.NewString()
	nickname := "race-test-" + uuid.NewString()[:8]

	t.Cleanup(func() {
		// CASCADE on auth_accounts.creator_id handles child rows.
		_, _ = db.Exec(`DELETE FROM creators WHERE nickname LIKE $1`, nickname+"%")
	})

	var before int
	if err := db.QueryRow(`SELECT COUNT(*) FROM creators`).Scan(&before); err != nil {
		t.Fatalf("count creators before: %v", err)
	}

	const n = 2
	errs := make([]error, n)
	ids := make([]uuid.UUID, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			profile := &UserProfile{
				ProviderID: providerID,
				Nickname:   nickname,
				AvatarURL:  "",
			}
			<-start
			ids[i], errs[i] = svc.createNewCreator(context.Background(), profile, "google", "")
		}(i)
	}
	close(start)
	wg.Wait()

	successes := 0
	failures := 0
	for i := 0; i < n; i++ {
		if errs[i] == nil {
			successes++
		} else {
			failures++
		}
	}

	if successes < 1 {
		t.Fatalf("expected at least 1 successful createNewCreator, got 0 (errs=%v)", errs)
	}
	if failures > 1 {
		t.Fatalf("expected at most 1 failure (UNIQUE race), got %d (errs=%v)", failures, errs)
	}

	var after int
	if err := db.QueryRow(`SELECT COUNT(*) FROM creators`).Scan(&after); err != nil {
		t.Fatalf("count creators after: %v", err)
	}

	// Expected delta = number of winning transactions (successes). The loser's
	// tx rolled back; before the fix it would have left an orphan creators row
	// (delta == n instead of successes).
	gotDelta := after - before
	if gotDelta != successes {
		t.Errorf("creators row delta=%d, want=%d (successes=%d, failures=%d, errs=%v) — losing tx must roll back its creators row",
			gotDelta, successes, successes, failures, errs)
	}

	// Cross-check: no orphan creators (every creator has at least one auth_account).
	var orphans int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM creators c
		LEFT JOIN auth_accounts a ON a.creator_id = c.id
		WHERE c.nickname = $1 AND a.id IS NULL
	`, nickname).Scan(&orphans); err != nil {
		t.Fatalf("orphan count query: %v", err)
	}
	if orphans != 0 {
		t.Errorf("found %d orphan creator(s) with nickname=%q (BeginTx rollback expected to prevent this)", orphans, nickname)
	}
}

// TestIntegration_FindOrCreateWithEmail_OnConflictRecovery pins the
// concurrent same-email signup recovery: when a competing transaction wins the
// creators INSERT race, CreateCreatorFromOAuthOnConflict (ON CONFLICT (email)
// DO NOTHING RETURNING *) returns zero rows, which the sqlc :one query surfaces
// as sql.ErrNoRows. The losing request MUST treat that as the conflict signal,
// re-query the winning creator, and merge into it — NOT fail the login.
//
// Before the fix, service.go's `if err != nil { return }` guard caught
// sql.ErrNoRows and returned "create creator: sql: no rows in result set",
// making the re-query recovery branch (newCreator.ID == uuid.Nil) dead code and
// failing the losing OAuth callback with /login?error=account_failed.
//
// Deterministic: a competing uncommitted INSERT holds the unique email key, so
// the service call's SELECT sees no row (returns ErrNoRows at the creators
// lookup) but its INSERT ... ON CONFLICT blocks until we commit the winner —
// forcing the ON CONFLICT DO NOTHING / ErrNoRows path every run.
func TestIntegration_FindOrCreateWithEmail_OnConflictRecovery(t *testing.T) {
	db := openAuthTestDB(t)
	svc := &Service{db: db}

	email := "conflict-" + uuid.NewString() + "@example.com"
	nickname := "conflict-test-" + uuid.NewString()[:8]

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM creators WHERE email = $1`, email)
	})

	// Competing transaction inserts the creator with the target email but does
	// NOT commit yet: invisible to other snapshots (so the service's SELECT
	// returns no rows) while its uncommitted unique key forces the service's
	// INSERT ... ON CONFLICT to block, then fire DO NOTHING once we commit.
	winnerTx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin winner tx: %v", err)
	}
	defer func() { _ = winnerTx.Rollback() }()

	var winnerID uuid.UUID
	if err := winnerTx.QueryRow(
		`INSERT INTO creators (nickname, email) VALUES ($1, $2) RETURNING id`,
		nickname, email,
	).Scan(&winnerID); err != nil {
		t.Fatalf("winner insert: %v", err)
	}

	type result struct {
		id  uuid.UUID
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		profile := &UserProfile{
			ProviderID:    "conflict-loser-" + uuid.NewString(),
			Nickname:      nickname,
			Email:         email,
			EmailVerified: true,
		}
		id, err := svc.findOrCreateWithEmail(context.Background(), profile, "google", email)
		resCh <- result{id, err}
	}()

	// Let the goroutine reach (and block on) the INSERT ... ON CONFLICT, then
	// commit the winner so the loser's conflict fires and returns zero rows.
	time.Sleep(200 * time.Millisecond)
	if err := winnerTx.Commit(); err != nil {
		t.Fatalf("commit winner: %v", err)
	}

	res := <-resCh
	if res.err != nil {
		t.Fatalf("findOrCreateWithEmail must recover from ON CONFLICT zero-row (sql.ErrNoRows), got error: %v", res.err)
	}
	if res.id != winnerID {
		t.Errorf("loser merged into wrong creator: got %s, want winner %s", res.id, winnerID)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM creators WHERE email = $1`, email).Scan(&count); err != nil {
		t.Fatalf("count creators: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 creators row for email, got %d (ON CONFLICT must not create a duplicate)", count)
	}
}
