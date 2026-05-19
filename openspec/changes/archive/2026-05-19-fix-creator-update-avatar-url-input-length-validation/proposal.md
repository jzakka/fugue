# Proposal: PUT /api/creators/me `avatar_url` 입력 길이 검증 추가

## Why

`apps/api/internal/creator/handler.go`의 `UpdateMe` 핸들러(L149-214)는 `avatar_url` 필드에 대한 사전 길이 검증 없이 JSON body의 값을 그대로 `db.UpdateCreatorParams.AvatarUrl`로 흘려보낸다(L196-200).

같은 핸들러 안의 `nickname` 필드(VARCHAR(50))는 이미 다음 패턴으로 enforce되어 있다(L178-185):

```go
if nickname == "" {
    writeError(w, http.StatusBadRequest, "닉네임은 비어있을 수 없습니다")
    return
}
if utf8.RuneCountInString(nickname) > 50 {
    writeError(w, http.StatusBadRequest, "닉네임은 50자를 초과할 수 없습니다")
    return
}
```

반면 `avatar_url` 필드(VARCHAR(500), `apps/api/db/migrations/000001_create_creators.up.sql:7`)는 같은 패턴이 누락되어 있다. 결과:

1. 클라이언트가 cap을 초과하는 `avatar_url`을 제출하면 PostgreSQL이 `value too long for type character varying(500)`로 UPDATE를 거부.
2. 핸들러 L201-205의 일반 500 분기가 이 에러를 흡수해 `500 "프로필 업데이트에 실패했습니다"`를 반환.
3. 사용자는 (a) 어느 필드 길이가 문제인지 모르고, (b) 서버 오류로 인지해 자신의 입력 문제임을 알 수 없으며, (c) 모니터링 측면에서 5xx false positive가 발생한다.

avatar_url은 통상 S3/CDN URL이며 query string(signed token, cache buster, analytics) 부착으로 500자를 자연 초과할 수 있어 악의 없는 사용자에게도 트리거된다.

이는 같은 핸들러 내부 비대칭 결함(nickname은 400, avatar_url은 500)이며, cycle 8(`fix-pin-create-input-length-validation`, PR #53)과 cycle 9(`fix-boards-handler-description-input-length-validation`, PR #54)의 정합성 작업을 같은 area에서 마지막으로 닫는다.

spec 측면에서도 `openspec/specs/profile/spec.md`의 "본인 프로필을 수정한다" Requirement는 `avatar_url` 길이 cap에 대한 Scenario를 갖고 있지 않다. 본 change는 이 갭을 채워 spec과 코드를 동시 정렬한다.

## What Changes

1. **`creator/handler.go`의 UpdateMe 핸들러에 `avatar_url` 길이 검증 추가** — L189-193 `if req.AvatarURL != nil` 분기 안의 `else`(비어있지 않은 값) 가지 안에서 cap 검증:
   ```go
   if utf8.RuneCountInString(*req.AvatarURL) > 500 {
       writeError(w, http.StatusBadRequest, "아바타 URL은 500자 이내여야 합니다")
       return
   }
   avatarURL = sql.NullString{String: *req.AvatarURL, Valid: true}
   ```

   빈 문자열(`""`) 케이스는 의도된 clear 동작(L189-190 `avatarURL = sql.NullString{}`)이므로 cap 검증을 적용하지 않는다.

2. **회귀 방지 단위 테스트 추가** — `apps/api/internal/creator/handler_test.go`에 신규 subtest:
   - `TestUpdateMe_RejectsAvatarURLOverRuneCap` — ASCII 501 → 400
   - `TestUpdateMe_RejectsAvatarURLOverRuneCapMultibyte` — 한국어 501 rune → 400
   - `TestUpdateMe_AcceptsAvatarURLAtRuneCap` — ASCII 500 boundary → 200 + DB update 호출 확인
   - `TestUpdateMe_AcceptsAvatarURLEmptyAsClear` — `""` → 검증 통과, `AvatarUrl.Valid == false` 확인(기존 clear 정책 회귀 방지)
   - `TestUpdateMe_AcceptsAvatarURLOmitted` — `avatar_url` 누락 → 검증 분기 skip, 기존 값 보존

3. **profile spec에 ADDED Requirement 추가** — `openspec/specs/profile/spec.md`의 "본인 프로필을 수정한다" Requirement 부근에 "avatar_url 입력은 creators 컬럼 cap에 맞춰 사전 길이 검증된다" Requirement를 ADDED. clear semantics(빈 문자열 허용)도 명시.

`unicode/utf8` import는 이미 `creator/handler.go:10`에 존재 — 추가 import 불필요.

## Why Now / Why Self-Contained

- **Why Now**: Discovery 모드에서 발견된 정합성 결함. 같은 패턴을 이미 enforce하는 핸들러가 codebase에 3개(pin/4 fields, boards/name+description, creator/nickname) 있어 확신도 5. cycle 8/9에서 같은 area를 닫아왔으며 본 cycle이 마지막 후보.
- **Why Self-Contained**: 변경 범위가 (a) creator/handler.go 1 블록, (b) creator/handler_test.go 5 신규 subtest, (c) spec ADDED Requirement 1개로 전부 한 changeset 안에 닫힌다. DB 마이그레이션, 다른 패키지 변경, infra 영향 없음.

## Scope

- 변경 파일:
  - `apps/api/internal/creator/handler.go` (1 블록)
  - `apps/api/internal/creator/handler_test.go` (5 신규 subtest)
  - `openspec/specs/profile/spec.md` (1 ADDED Requirement)
- 변경 외 파일: pin/handler·boards/handler·creator nickname(이미 enforce 중), bot harvester(별개 truncate 정책).

## Out of Scope

- creators 컬럼 cap 확장(예: VARCHAR(500) → VARCHAR(1000)) 마이그레이션 — 별개 change. 현 ERD가 합의된 cap이며, 본 change는 cap을 그대로 두고 enforce만 추가한다.
- avatar_url 형식(URL scheme/host) 검증 — 별개 시맨틱 검증. 본 change는 길이 cap에만 집중.
- `http.MaxBytesReader` 적용(JSON body size cap) — 별개 보안 후보.

## Rollback

`creator/handler.go`의 1 검증 블록을 revert. 테스트 신규 subtest 삭제. spec Requirement revert. DB나 데이터 변환이 없으므로 즉시 가역. 기존에 정상 입력(cap 이하)으로 동작하던 모든 호출 경로는 변경 없음.

## QA Plan (실 환경)

1. `docker-compose up -d`로 api+postgres 기동(기존 stack 재사용).
2. `cd apps/api && go run cmd/server/main.go` 백그라운드 기동.
3. JWT 발급(throwaway `cmd/qajwt` 또는 동등 dev-only).
4. **fix 적용**:
   - PUT `/api/creators/me` `avatar_url=A*501` → 400 `"아바타 URL은 500자 이내여야 합니다"`
   - PUT `avatar_url=가*501` (한국어 멀티바이트) → 400 (rune-count cap 확인)
   - PUT `avatar_url=A*500` boundary → 200 정상 + 응답 body에 length 500
   - PUT `avatar_url=""` (clear semantics) → 200 + 응답 avatar_url=null (기존 정책 회귀 방지)
   - PUT `avatar_url` 누락(부분 업데이트, nickname만) → 200 + 기존 avatar_url 보존
5. **회귀(adjacent field)**: nickname 검증이 회귀하지 않았는지 nickname=`A*51` → 400 "닉네임은 50자를 초과할 수 없습니다" 동일 응답.
6. **회귀(인접 엔드포인트)**: GET `/api/creators/me` → 200 정상.
7. **DB 확인**: `psql`로 `SELECT length(avatar_url) FROM creators WHERE avatar_url IS NOT NULL ORDER BY updated_at DESC LIMIT 5;` → 모든 row가 500 이하.
8. **응답 스키마**: 모든 400이 `Content-Type: application/json` + `{"error": "..."}` 형식 준수.

## Threat Model / Failure Mode

- **이전 (fix 없음)**: 클라이언트 검증 우회 시 cap 초과 avatar_url → DB 거부 → 500 응답 + 로그 노이즈. S3 signed URL의 자연 cap 초과 시나리오(token + tracking params)에서 악의 없이도 발생.
- **이후 (fix 적용)**: cap 초과 입력 → JSON 파싱 직후 400 거부 → 사용자가 어느 필드를 줄여야 하는지 명확히 인지. spec과 코드, 같은 핸들러의 다른 필드(nickname)와의 패턴 정합성 회복. cycle 8/9의 user-input→VARCHAR cap area 정합성 작업이 본 cycle로 완결.
