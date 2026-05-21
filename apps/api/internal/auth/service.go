package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sqlc-dev/pqtype"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

const (
	rtPrefix    = "rt:"
	rtIdxPrefix = "rt_index:"
	rtTTL       = 7 * 24 * time.Hour
	rtGrace     = 10 * time.Second
)

type Service struct {
	db     *sql.DB
	rdb    *redis.Client
	jwtSvc *JWTService
}

func NewService(database *sql.DB, rdb *redis.Client, jwtSvc *JWTService) *Service {
	return &Service{db: database, rdb: rdb, jwtSvc: jwtSvc}
}

// FindOrCreateCreator implements the account merge decision tree.
//
//  1. Look up by (provider, provider_id) → existing user
//  2. If email present and verified → look up by email → merge or create
//  3. If no email → create new creator
func (s *Service) FindOrCreateCreator(ctx context.Context, profile *UserProfile, providerName string) (uuid.UUID, error) {
	q := db.New(s.db)

	// Step 1: Check if this provider account already exists
	existing, err := q.GetAuthAccountByProvider(ctx, db.GetAuthAccountByProviderParams{
		Provider:   providerName,
		ProviderID: profile.ProviderID,
	})
	if err == nil {
		return existing.CreatorID, nil
	}
	if err != sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("lookup provider account: %w", err)
	}

	// Step 2: If email is present and verified, attempt merge
	email := profile.Email
	if !profile.EmailVerified {
		email = ""
	}

	if email != "" {
		return s.findOrCreateWithEmail(ctx, profile, providerName, email)
	}

	// Step 3: No email, create new creator
	return s.createNewCreator(ctx, profile, providerName, "")
}

func (s *Service) findOrCreateWithEmail(ctx context.Context, profile *UserProfile, providerName, email string) (uuid.UUID, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := db.New(tx)

	// Check auth_accounts by email (FOR UPDATE to prevent concurrent merge)
	authAccounts, err := q.GetAuthAccountByEmailForUpdate(ctx, sql.NullString{String: email, Valid: true})
	if err != nil {
		return uuid.Nil, fmt.Errorf("lookup auth by email: %w", err)
	}
	if len(authAccounts) > 0 {
		// Merge: add new auth_account to existing creator
		creatorID := authAccounts[0].CreatorID
		if err := s.addAuthAccount(ctx, q, creatorID, profile, providerName, email); err != nil {
			return uuid.Nil, err
		}
		if err := tx.Commit(); err != nil {
			return uuid.Nil, fmt.Errorf("commit merge: %w", err)
		}
		return creatorID, nil
	}

	// Check creators by email (FOR UPDATE)
	creator, err := q.GetCreatorByEmailForUpdate(ctx, sql.NullString{String: email, Valid: true})
	if err == nil {
		// Merge: add new auth_account to existing creator
		if err := s.addAuthAccount(ctx, q, creator.ID, profile, providerName, email); err != nil {
			return uuid.Nil, err
		}
		if err := tx.Commit(); err != nil {
			return uuid.Nil, fmt.Errorf("commit merge: %w", err)
		}
		return creator.ID, nil
	}
	if err != sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("lookup creator by email: %w", err)
	}

	// Create new creator with ON CONFLICT for race condition
	nickname := truncateNickname(profile.Nickname)
	newCreator, err := q.CreateCreatorFromOAuthOnConflict(ctx, db.CreateCreatorFromOAuthOnConflictParams{
		Nickname:  nickname,
		AvatarUrl: toNullString(truncateAvatarURL(profile.AvatarURL)),
		Email:     toNullString(email),
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create creator: %w", err)
	}

	var creatorID uuid.UUID
	if newCreator.ID == uuid.Nil {
		// ON CONFLICT DO NOTHING fired, re-query to get existing
		existing, err := q.GetCreatorByEmailForUpdate(ctx, sql.NullString{String: email, Valid: true})
		if err != nil {
			return uuid.Nil, fmt.Errorf("re-query creator after conflict: %w", err)
		}
		creatorID = existing.ID
	} else {
		creatorID = newCreator.ID
	}

	if err := s.addAuthAccount(ctx, q, creatorID, profile, providerName, email); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, fmt.Errorf("commit create: %w", err)
	}
	return creatorID, nil
}

func (s *Service) createNewCreator(ctx context.Context, profile *UserProfile, providerName, email string) (uuid.UUID, error) {
	nickname := truncateNickname(profile.Nickname)

	q := db.New(s.db)
	creator, err := q.CreateCreatorFromOAuth(ctx, db.CreateCreatorFromOAuthParams{
		Nickname:  nickname,
		AvatarUrl: toNullString(truncateAvatarURL(profile.AvatarURL)),
		Email:     toNullString(email),
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create creator: %w", err)
	}

	if err := s.addAuthAccount(ctx, q, creator.ID, profile, providerName, email); err != nil {
		return uuid.Nil, err
	}
	return creator.ID, nil
}

func (s *Service) addAuthAccount(ctx context.Context, q *db.Queries, creatorID uuid.UUID, profile *UserProfile, providerName, email string) error {
	_, err := q.CreateAuthAccountWithProfile(ctx, db.CreateAuthAccountWithProfileParams{
		CreatorID:  creatorID,
		Provider:   providerName,
		ProviderID: profile.ProviderID,
		Email:      toNullString(email),
		Profile:    pqtype.NullRawMessage{RawMessage: profile.RawProfile, Valid: len(profile.RawProfile) > 0},
	})
	if err != nil {
		return fmt.Errorf("create auth account: %w", err)
	}
	return nil
}

// StoreRefreshToken stores a refresh token's JTI in Redis.
func (s *Service) StoreRefreshToken(ctx context.Context, jti string, creatorID uuid.UUID) error {
	key := rtPrefix + jti
	data, _ := json.Marshal(map[string]string{
		"creator_id": creatorID.String(),
		"status":     "active",
	})
	if err := s.rdb.Set(ctx, key, data, rtTTL).Err(); err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}
	idxKey := rtIdxPrefix + creatorID.String()
	if err := s.rdb.SAdd(ctx, idxKey, jti).Err(); err != nil {
		return fmt.Errorf("add to token index: %w", err)
	}
	s.rdb.Expire(ctx, idxKey, rtTTL)
	return nil
}

// RotateRefreshToken validates the old refresh token and issues a new pair.
func (s *Service) RotateRefreshToken(ctx context.Context, refreshTokenString string) (*TokenPair, error) {
	claims, err := s.jwtSvc.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	jti := claims.ID
	if jti == "" {
		return nil, fmt.Errorf("refresh token missing JTI")
	}

	creatorID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid creator ID in token: %w", err)
	}

	key := rtPrefix + jti
	val, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key doesn't exist. Could be TTL expiry (not compromise).
		// JWT exp check already done by ValidateToken, so if we're here
		// the JWT is not expired but the Redis key is gone.
		// This means the token was rotated and grace period passed, OR TTL expired.
		// Return simple 401, not compromise detection.
		return nil, fmt.Errorf("refresh token expired or revoked")
	}
	if err != nil {
		return nil, fmt.Errorf("check refresh token: %w", err)
	}

	var tokenData map[string]string
	if err := json.Unmarshal([]byte(val), &tokenData); err != nil {
		return nil, fmt.Errorf("parse refresh token data: %w", err)
	}

	status := tokenData["status"]
	if status == "rotated" {
		// Rotated token reused WITHIN grace period (key still exists with short TTL).
		// This is a legitimate concurrent request. Allow it.
		// But if the key had already expired (handled above as redis.Nil),
		// that would be post-grace, which we handle as simple expiry.
	} else if status != "active" {
		return nil, fmt.Errorf("refresh token in unexpected state: %s", status)
	}

	// Issue new pair
	pair, err := s.jwtSvc.IssueTokenPair(creatorID)
	if err != nil {
		return nil, err
	}

	// Store new refresh token
	if err := s.StoreRefreshToken(ctx, pair.RefreshJTI, creatorID); err != nil {
		return nil, err
	}

	// Mark old token as rotated with grace period.
	//
	// A silent failure here leaves the old rt:{JTI} at status=active with the
	// original ~7d TTL. The next RotateRefreshToken call with the same old
	// token would then bypass the grace branch (status != "rotated") and re-
	// issue another pair — same old token rotating indefinitely (token sprawl).
	// The new pair has already been returned to the client, so we cannot fail
	// the request, but the operator needs the timestamped trail to detect the
	// stuck-active window and the index leak that follows. Mirrors the cycle C
	// RevokeRefreshToken Del/SRem logging contract.
	rotatedData, _ := json.Marshal(map[string]string{
		"creator_id": creatorID.String(),
		"status":     "rotated",
	})
	if setErr := s.rdb.Set(ctx, key, rotatedData, rtGrace).Err(); setErr != nil {
		log.Printf("auth.RotateRefreshToken: grace mark Set error: %v (jti=%s sub=%s)", setErr, jti, creatorID)
	}

	// Remove old JTI from index. A silent failure leaves stale jti in
	// rt_index:{sub}; RevokeAllTokens compromise-detection sweeps over a
	// superset of live tokens (harmless but the index-correctness invariant
	// breaks).
	idxKey := rtIdxPrefix + creatorID.String()
	if sremErr := s.rdb.SRem(ctx, idxKey, jti).Err(); sremErr != nil {
		log.Printf("auth.RotateRefreshToken: index SRem error: %v (jti=%s sub=%s)", sremErr, jti, creatorID)
	}

	return pair, nil
}

// RevokeRefreshToken removes a single refresh token.
//
// The Redis DEL/SREM results are logged on failure. The function signature
// remains void (best-effort revocation policy preserved), but a Redis outage
// would otherwise leave the rt:{JTI} key alive at status=active for the full
// 7-day TTL while the user sees only 200 OK + cookie expiry — meaning a
// stolen refresh token could keep rotating via /api/auth/refresh after the
// user believes they are logged out. The log line gives an operator the
// timestamped trail to detect that orphan-key window.
func (s *Service) RevokeRefreshToken(ctx context.Context, refreshTokenString string) {
	claims, err := s.jwtSvc.ValidateToken(refreshTokenString)
	if err != nil {
		return
	}
	if claims.ID != "" {
		if delErr := s.rdb.Del(ctx, rtPrefix+claims.ID).Err(); delErr != nil {
			log.Printf("auth.RevokeRefreshToken: Del error: %v (jti=%s)", delErr, claims.ID)
		}
		if sub := claims.Subject; sub != "" {
			if sremErr := s.rdb.SRem(ctx, rtIdxPrefix+sub, claims.ID).Err(); sremErr != nil {
				log.Printf("auth.RevokeRefreshToken: SRem error: %v (jti=%s sub=%s)", sremErr, claims.ID, sub)
			}
		}
	}
}

// RevokeAllTokens revokes all refresh tokens for a creator (compromise detection).
func (s *Service) RevokeAllTokens(ctx context.Context, creatorID uuid.UUID) {
	idxKey := rtIdxPrefix + creatorID.String()
	jtis, err := s.rdb.SMembers(ctx, idxKey).Result()
	if err != nil {
		return
	}
	for _, jti := range jtis {
		s.rdb.Del(ctx, rtPrefix+jti)
	}
	s.rdb.Del(ctx, idxKey)
}

func truncateNickname(name string) string {
	if name == "" {
		return "creator-" + uuid.New().String()[:8]
	}
	r := []rune(name)
	if len(r) > 50 {
		r = r[:50]
	}
	return string(r)
}

// truncateAvatarURL caps profile avatar URLs at 500 runes to match
// creators.avatar_url VARCHAR(500). OAuth providers (notably Google) can return
// signed profile picture URLs that grow past 500 chars with accumulated size/
// crop/signature params; without truncation the creators INSERT fails with
// `value too long for type character varying(500)` and first-login is blocked
// for that user. Silent truncation parallels truncateNickname above and the
// bot harvester's truncateRunes — external-system ingress applies silent
// truncate per .fugue/decision-log.md 2026-05-19 cycle 10.
func truncateAvatarURL(url string) string {
	r := []rune(url)
	if len(r) > 500 {
		r = r[:500]
	}
	return string(r)
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
