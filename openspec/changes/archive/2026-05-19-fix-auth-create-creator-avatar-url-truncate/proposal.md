# Proposal: OAuth 첫 로그인 시 `profile.AvatarURL`을 `creators.avatar_url VARCHAR(500)` cap에 맞춰 사전 절단

## Why

`apps/api/internal/auth/service.go`의 creator 생성 경로 두 곳은 `profile.Nickname`은 `truncateNickname`(L292-301)으로 50 rune cap에 맞춰 사전 절단하지만, `profile.AvatarURL`은 `toNullString(profile.AvatarURL)`(L114, L149)로 그대로 흘려보낸다.

```go
// service.go L111-115 findOrCreateWithEmail
nickname := truncateNickname(profile.Nickname)         // ← 50 rune cap 적용
newCreator, err := q.CreateCreatorFromOAuthOnConflict(ctx, db.CreateCreatorFromOAuthOnConflictParams{
    Nickname:  nickname,
    AvatarUrl: toNullString(profile.AvatarURL),        // ← 절단 없음
    Email:     toNullString(email),
})
```

```go
// service.go L144-151 createNewCreator (동일 패턴)
nickname := truncateNickname(profile.Nickname)
creator, err := q.CreateCreatorFromOAuth(ctx, db.CreateCreatorFromOAuthParams{
    Nickname:  nickname,
    AvatarUrl: toNullString(profile.AvatarURL),        // ← 절단 없음
    Email:     toNullString(email),
})
```

`creators.avatar_url`은 `apps/api/db/migrations/000001_create_creators.up.sql:7`에서 `VARCHAR(500)`으로 정의된다. OAuth provider가 500 rune을 초과하는 avatar URL을 반환하면 PostgreSQL이 `value too long for type character varying(500)`으로 INSERT를 거부 → `fmt.Errorf("create creator: %w", err)`로 wrap → callback handler(`handler.go` Callback 경로)가 redirect 실패. **그 유저는 첫 로그인 자체가 결정적으로 불가능해진다.**

Google 프로필 사진 URL은 base `https://lh3.googleusercontent.com/a/...` + size/crop/signature 파라미터(`=s96-c-rg-br100-...`)가 누적되면 300~500+ chars 도달 가능하며 점진적 증가 추세에 있다. Discord URL은 `cdn.discordapp.com/avatars/{id}/{hash}.png` 형식으로 ~80 chars 고정이라 안전(`provider.go:163-166`).

스펙 측면: `openspec/specs/auth/spec.md:8-10` Requirement "소셜 로그인으로 인증한다" Scenario "첫 로그인 시 계정 자동 생성" — **WHEN** 처음 로그인하는 유저가 OAuth 인증을 완료하면 / **THEN** 유저 계정이 자동 생성된다(SHALL). 이 SHALL은 OAuth payload의 임의 길이에 대해 보장되어야 한다. 현재 코드는 `nickname` 50 rune 한정 보장만 충족 — `avatar_url`이 500 rune을 초과하는 OAuth payload에 대해서는 SHALL을 위반한다.

정책 정합성: `.fugue/decision-log.md` 2026-05-19 cycle 10 entry — "bot 경로(harvester `truncateRunes`)와 달리 user-facing 경로는 reject 400(사용자가 입력 수정 가능)." OAuth provider 응답은 user-input이 아니라 외부 시스템 데이터로, OAuth callback 중간에 400을 띄울 적절한 surface가 없다. 따라서 **silent truncate** 정책이 적합(harvester `truncateRunes` 패턴과 동일 카테고리, in-codebase precedent `truncateNickname`도 silent truncate).

## What Changes

1. **`auth/service.go`에 `truncateAvatarURL` 헬퍼 함수 추가** — `truncateNickname` 바로 아래(L301 직후)에 다음 함수 추가:

   ```go
   func truncateAvatarURL(url string) string {
       r := []rune(url)
       if len(r) > 500 {
           r = r[:500]
       }
       return string(r)
   }
   ```

   `truncateNickname`과 동일 시그니처·동일 절단 알고리즘(`[]rune` slice). 빈 문자열 입력은 빈 문자열 반환(특별 처리 없음 — `toNullString`이 `""`을 `sql.NullString{}`으로 변환하는 기존 동작 보존).

2. **`findOrCreateWithEmail`(L114)과 `createNewCreator`(L149) 두 곳에서 helper를 사용** — `toNullString(profile.AvatarURL)` → `toNullString(truncateAvatarURL(profile.AvatarURL))`. 변경 lines = 2.

3. **회귀 방지 단위 테스트 추가** — 신규 파일 `apps/api/internal/auth/truncate_test.go`에 다음 테스트:
   - `TestTruncateAvatarURL_BelowCap` — 100 char URL → 무손실 보존
   - `TestTruncateAvatarURL_AtCap` — 500 char URL → 정확히 500 rune 반환
   - `TestTruncateAvatarURL_AboveCap` — 600 char URL → 정확히 500 rune 반환(절단)
   - `TestTruncateAvatarURL_MultibyteAboveCap` — `가` × 600 rune → 500 rune 반환(rune count cap, byte count 아님)
   - `TestTruncateAvatarURL_Empty` — `""` → `""`
   - `TestTruncateNickname_AvatarURLPolicyParity` — 기존 `truncateNickname`이 동일 알고리즘인지 회귀 보장(같은 시그니처, `[]rune` slice 패턴 일치)

4. **auth spec에 ADDED Requirement 추가** — `openspec/specs/auth/spec.md`의 "소셜 로그인으로 인증한다" Requirement에 ADDED Scenario:
   > **WHEN** OAuth provider가 `creators` 컬럼 cap을 초과하는 길이의 프로필 필드(nickname, avatar_url)를 반환하면
   > **THEN** 시스템은 해당 필드를 컬럼 cap에 맞춰 rune-count 기준으로 절단한 후 계정을 생성한다(SHALL). 계정 자동 생성 자체는 실패하지 않는다.

   기존 Scenario "첫 로그인 시 계정 자동 생성"의 enforcement를 명시화하는 ADDED 항목.

## Why Now / Why Self-Contained

- **Why Now**: cycles 7-10(`pin`·`boards`·`creator` user-facing handler input validation)이 user 입력 → DB VARCHAR cap area를 닫았고, 본 cycle 11이 그 area의 외부 ingress 짝(OAuth provider 응답 → DB VARCHAR cap)을 닫는다. in-codebase precedent(`truncateNickname`)가 동일 파일에 이미 존재해 패턴이 명확.
- **Why Self-Contained**: 변경 범위가 (a) `auth/service.go` 헬퍼 1개 + 호출부 2 lines, (b) 신규 `truncate_test.go` 6 테스트, (c) `auth/spec.md` ADDED Scenario 1개. DB 마이그레이션 없음. 다른 패키지 변경 없음. infra 영향 없음.

## Scope

- 변경 파일:
  - `apps/api/internal/auth/service.go` (1 헬퍼 추가 + 2 call sites 수정)
  - `apps/api/internal/auth/truncate_test.go` (신규 6 테스트)
  - `openspec/specs/auth/spec.md` (1 ADDED Scenario)
- 변경 외 파일: `auth/provider.go`(profile field 그대로 전달), `auth/handler.go`(callback flow), `creator/handler.go`(user-facing 경로, 별개 enforce 정책).

## Out of Scope

- **`creators.email` 절단** — 별개 후보. RFC 5321은 email max length 254로 제한하며 VARCHAR(255)가 RFC 한계를 cover한다(Google·Discord 모두 RFC 준수). 현재 트리거 시나리오 없음.
- **`auth_accounts.email` 절단** — 동일 사유로 별개 후보.
- **`auth_accounts.provider_id` 절단** — Google ID 21 chars, Discord snowflake 18-20 chars. VARCHAR(255)에 비해 충분. 변경 불필요.
- **`creators.nickname` cap 변경** — 본 change는 cap을 그대로 두고 enforce만 추가.
- **avatar_url 형식 검증(URL scheme/host)** — 별개 시맨틱 검증.

## Rollback

`auth/service.go`의 헬퍼 함수 + 2 call site 변경을 revert. 신규 test 파일 삭제. spec ADDED Scenario revert. DB나 데이터 변환이 없으므로 즉시 가역. 기존에 정상 입력(500 rune 이하 avatar URL)으로 동작하던 모든 OAuth flow는 변경 없음(절단 분기 미발동).

## QA Plan (실 환경)

1. `docker-compose up -d`로 api+postgres+redis 기동(기존 stack 재사용).
2. `cd apps/api && go run cmd/server/main.go` 백그라운드 기동.
3. **단위 테스트**: `cd apps/api && go test ./internal/auth/... -run TestTruncate` — 6 테스트 모두 pass.
4. **service-level 통합 검증** (OAuth provider mock 또는 직접 DB 호출):
   - **T1 (truncate 발동)**: profile.AvatarURL = `"https://example.com/" + strings.Repeat("A", 600)` (총 620 chars) 넘긴 service flow → `creators` row INSERT 성공, `SELECT length(avatar_url) FROM creators WHERE nickname='<test>'` = 500.
   - **T2 (boundary 통과)**: profile.AvatarURL = 500 char URL → INSERT 성공, length = 500.
   - **T3 (정상 길이 무손실)**: profile.AvatarURL = 80 char Discord URL → INSERT 성공, length = 80 (절단 미발동).
   - **T4 (한국어 멀티바이트)**: profile.AvatarURL = `strings.Repeat("가", 600)` → length(rune) = 500, length(byte) = 1500. PostgreSQL VARCHAR(N)은 rune-count cap이라 정상 저장.
   - **T5 (빈 문자열)**: profile.AvatarURL = `""` → `toNullString`이 `sql.NullString{}` 반환, DB는 NULL 저장.
5. **fix 전 재현(증명용)**: helper 추가 없이 직접 SQL `INSERT INTO creators (nickname, avatar_url, email) VALUES ('t', repeat('A', 501), 't@x.com')` 실행 → `pq: value too long for type character varying(500)` 에러 확인. fix 적용 후 service flow에서는 이 에러가 발생하지 않음을 대비 확인.
6. **회귀(adjacent ingress)**: nickname truncation이 동일하게 동작하는지 `truncateNickname("X" 60문자)` → 50 rune 반환 확인(기존 동작 보존).
7. **회귀(인접 엔드포인트)**: `GET /api/auth/providers` → 200 정상.

## Threat Model / Failure Mode

- **이전 (fix 없음)**: OAuth provider가 500+ rune avatar URL을 반환하면 첫 로그인 시 `creators` INSERT가 PostgreSQL `value too long` 에러로 결정적 실패 → 그 유저는 영구히 로그인 불가. 자연 발생 시나리오: Google이 향후 signed URL 길이를 늘리면 모든 신규 유저 영향. 운영팀 입장에서는 callback 500 redirect로만 관측되어 원인 진단이 어려움.
- **이후 (fix 적용)**: 500 rune 초과 avatar URL은 silent truncate되어 INSERT 성공. `nickname` 절단 정책과 일관. 정상 길이(~100 chars) 입력은 절단 분기 미발동으로 무손실. OAuth callback flow 회귀 0.
