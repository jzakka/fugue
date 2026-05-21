# Tasks

## Implementation

- [ ] T1. `apps/api/internal/auth/service.go`에 `truncateAvatarURL(url string) string` helper 함수를 `truncateNickname`(L292-301) 바로 아래에 추가. `[]rune` slice 패턴으로 500 rune cap 적용.
- [ ] T2. `findOrCreateWithEmail`의 `AvatarUrl: toNullString(profile.AvatarURL)`(L114)을 `AvatarUrl: toNullString(truncateAvatarURL(profile.AvatarURL))`로 교체.
- [ ] T3. `createNewCreator`의 `AvatarUrl: toNullString(profile.AvatarURL)`(L149)을 `AvatarUrl: toNullString(truncateAvatarURL(profile.AvatarURL))`로 교체.

## Tests

- [ ] T4. 신규 파일 `apps/api/internal/auth/truncate_test.go` 생성. 다음 테스트 케이스:
  - `TestTruncateAvatarURL_BelowCap`: 100 char URL → 무손실 보존(input == output)
  - `TestTruncateAvatarURL_AtCap`: 500 char URL → 정확히 500 rune 반환(input == output)
  - `TestTruncateAvatarURL_AboveCap`: 600 char URL → 정확히 500 rune 반환(절단)
  - `TestTruncateAvatarURL_MultibyteAboveCap`: `가` × 600 rune → 500 rune 반환(rune cap, not byte cap)
  - `TestTruncateAvatarURL_Empty`: `""` → `""`
  - `TestTruncateNickname_AvatarURLPolicyParity`: `truncateNickname`이 같은 알고리즘으로 nickname cap(50)을 적용하는지 회귀 보장.

## Spec

- [ ] T5. `openspec/specs/auth/spec.md`의 "소셜 로그인으로 인증한다" Requirement에 ADDED Scenario 추가:
  > **WHEN** OAuth provider가 `creators` 컬럼 cap을 초과하는 길이의 프로필 필드(nickname, avatar_url)를 반환하면
  > **THEN** 시스템은 해당 필드를 컬럼 cap에 맞춰 rune-count 기준으로 절단한 후 계정을 생성한다(SHALL). 계정 자동 생성 자체는 실패하지 않는다.

## Verification

- [ ] T6. `cd apps/api && go vet ./...` 통과.
- [ ] T7. `cd apps/api && go build ./...` 통과.
- [ ] T8. `cd apps/api && go test ./...` 통과(신규 6 테스트 포함).
- [ ] T9. 실 환경 QA(proposal.md § QA Plan 참조):
  - 단위 테스트 `go test ./internal/auth/... -run TestTruncate` 6건 pass.
  - service-level: psql 직접 INSERT로 cap 초과 입력 거부 재현, helper 경유 INSERT는 성공 확인.
  - 회귀: `truncateNickname` 별도 호출로 50 rune cap 보존 확인.
  - 회귀: `GET /api/auth/providers` 200 정상.
