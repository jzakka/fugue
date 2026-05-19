# Proposal: POST/PUT /api/boards `description` 입력 길이 검증 추가

## Why

`apps/api/internal/boards/handler.go`의 `Create`(L92-139)와 `Update`(L240-333) 핸들러는 `description` 필드에 대한 사전 길이 검증 없이 JSON body의 값을 그대로 `db.CreateBoardParams.Description` / `db.UpdateBoardParams.Description`로 흘려보낸다.

같은 핸들러 안의 `name` 필드(VARCHAR(100))는 다음 패턴으로 이미 enforce되어 있다:

```go
// boards/handler.go:110
if utf8.RuneCountInString(name) > 100 {
    writeError(w, http.StatusBadRequest, "보드 이름은 100자 이내여야 합니다")
    return
}
// boards/handler.go:287 (Update 대칭)
```

반면 `description` 필드(VARCHAR(500), `apps/api/db/migrations/000008_create_boards.up.sql:5`)는 같은 패턴이 누락되어 있다. 결과:

1. 클라이언트가 cap을 초과하는 description을 제출하면 PostgreSQL이 `value too long for type character varying(500)`로 INSERT/UPDATE를 거부.
2. 핸들러 L132-135(Create) / L313-316(Update)의 일반 500 분기가 이 에러를 흡수해 `500 "보드를 생성할 수 없습니다"` / `"보드를 수정할 수 없습니다"`를 반환.
3. 사용자는 (a) 어느 필드를 줄여야 하는지 모르고, (b) 서버 오류로 인지해 자신의 입력 문제임을 알 수 없으며, (c) 모니터링 측면에서 5xx false positive가 발생한다.

이는 같은 핸들러 내부 비대칭 결함(name은 400, description은 500)이며, 같은 codebase가 이미 enforce하는 패턴(`creator/handler.go:178-185`의 nickname, 위에서 인용한 boards name)에서 description만 빠진 갭이다. `apps/api/internal/pin/handler.go`에도 cycle 8(`fix-pin-create-input-length-validation`, PR #53, c3f6899)에서 같은 패턴으로 4개 텍스트 필드 사전 검증이 추가되었다 — 본 change는 그 정합성 작업을 boards 핸들러로 확장한다.

spec 측면에서도 `openspec/specs/board/spec.md`의 "보드를 생성한다" / "보드를 수정한다" Requirement는 `description` 길이 cap에 대한 Scenario를 갖고 있지 않다. 본 change는 이 갭을 채워 spec과 코드를 동시 정렬한다.

## What Changes

1. **`boards/handler.go`의 Create 핸들러에 `description` 길이 검증 추가** — L121 `if req.Description != nil` 분기 안에서 L122 할당 직전:
   ```go
   if utf8.RuneCountInString(*req.Description) > 500 {
       writeError(w, http.StatusBadRequest, "보드 설명은 500자 이내여야 합니다")
       return
   }
   ```

2. **`boards/handler.go`의 Update 핸들러에 같은 검증 추가** — L293 `if req.Description != nil` 분기 안에서 L294 할당 직전 같은 블록.

3. **회귀 방지 단위 테스트 추가** — `apps/api/internal/boards/handler_test.go` (신규 파일) 또는 기존 `handler_optional_auth_test.go` 확장으로 Create 경로 검증 케이스 추가:
   - description ASCII 501자 → 400
   - description 한국어 501 rune → 400
   - description ASCII 500자 boundary → DB 도달(real-env에서는 성공, 단위 테스트는 DB 미설정이므로 500까지 통과만 확인)
   - description nil(생략) → 검증 건너뛰기 확인
   - Update 경로는 GetBoard 선행으로 DB 의존성이 있어 단위 테스트에서 제외하고 real-env QA로 검증

4. **board spec에 ADDED Requirement 추가** — `openspec/specs/board/spec.md`의 "보드를 생성한다" / "보드를 수정한다" Requirement 부근에 "보드 description 입력은 boards 컬럼 cap에 맞춰 사전 길이 검증된다" Requirement를 ADDED.

`unicode/utf8` import는 이미 `boards/handler.go:11`에 존재 — 추가 import 불필요.

## Why Now / Why Self-Contained

- **Why Now**: Discovery 모드에서 발견된 정합성 결함. 같은 패턴을 이미 enforce하는 핸들러가 codebase에 3개(creator/nickname, boards/name, pin/4 fields) 있어 확신도 5. fix가 한 파일 2 블록 추가로 self-contained.
- **Why Self-Contained**: 변경 범위가 (a) boards/handler.go 2 블록, (b) boards/handler_test.go(또는 기존 test 확장) Create 경로 단위 테스트, (c) spec ADDED Requirement 1개로 전부 한 changeset 안에 닫힌다. DB 마이그레이션, 다른 패키지 변경, infra 영향 없음.

## Scope

- 변경 파일:
  - `apps/api/internal/boards/handler.go` (2 블록)
  - `apps/api/internal/boards/handler_test.go` 또는 등가 파일 (단위 테스트 추가)
  - `openspec/specs/board/spec.md` (1 ADDED Requirement)
- 변경 외 파일: pin/handler(이미 cycle 8에서 enforce), creator/handler(이미 nickname enforce 중 — avatar_url은 별개 backlog), bot/harvester 경로(별개 truncate 정책).

## Out of Scope

- boards 컬럼 cap 확장(예: VARCHAR(500) → VARCHAR(1000)) 마이그레이션 — 별개 change. 현 ERD와 spec이 합의된 cap이며, 본 change는 cap을 그대로 두고 enforce만 추가한다.
- creator/handler.go의 avatar_url 길이 검증 누락 — `backlog-system.yaml`의 별개 item(`system-20260519-creator-update-avatar-url-no-length-validation`)으로 분리. 본 change는 boards 핸들러에만 집중.
- Update 경로 단위 테스트(DB 의존성으로 별도 scaffolding 필요) — real-env QA로 동등 보장.

## Rollback

`boards/handler.go`의 2 검증 블록을 revert. 테스트 파일 변경 revert. spec Requirement revert. DB나 데이터 변환이 없으므로 즉시 가역. 기존에 정상 입력(cap 이하)으로 동작하던 모든 호출 경로는 변경 없음.

## QA Plan (실 환경)

1. `docker-compose up -d`로 api+postgres+redis 기동 (기존 stack 재사용 가능).
2. `cd apps/api && go run cmd/server/main.go` 백그라운드 기동.
3. JWT 발급(throwaway `cmd/qajwt/main.go` 또는 동등 dev-only 경로).
4. **Create — fix 전 재현(reference)**: POST `/api/boards` description=`A*501` → 500 "보드를 생성할 수 없습니다" + 서버 로그 `pq: value too long for type character varying(500)`.
5. **Create — fix 적용**:
   - description=`A*501` → 400 `"보드 설명은 500자 이내여야 합니다"`
   - description=`가*501` (한국어 멀티바이트) → 400 (rune-count cap 확인)
   - description=`A*500` boundary → 201 정상 생성
   - description 생략(`{"name":"t"}`) → 201 정상 생성
6. **Update — fix 적용**: 위에서 생성한 board id로
   - PUT description=`B*501` → 400 같은 메시지
   - PUT description=`나*501` → 400
   - PUT description=`B*500` boundary → 200 정상
   - PUT description 생략 → 기존 description 보존 확인(L292 merge 분기)
7. **회귀(인접 엔드포인트)**: GET `/api/boards/{id}` → 200 정상.
8. **회귀(adjacent field)**: name 검증이 회귀하지 않았는지 name=`A*101` → 400 "보드 이름은 100자 이내여야 합니다" 동일 응답.
9. **DB 확인**: `psql`로 `SELECT length(description) FROM boards ORDER BY created_at DESC LIMIT 5;` → 모든 row가 500 이하.
10. **응답 스키마**: 모든 400이 `Content-Type: application/json` + `{"error": "..."}` 형식 준수.

## Threat Model / Failure Mode

- **이전 (fix 없음)**: 클라이언트 검증 우회 시 cap 초과 description → DB 거부 → 500 응답 + 로그 노이즈. boards에는 S3 업로드가 없어 orphan 리소스는 없음(pin 경로와 차이).
- **이후 (fix 적용)**: cap 초과 입력 → JSON 파싱 직후 400 거부 → 사용자가 어느 필드를 줄여야 하는지 명확히 인지. spec과 코드, 같은 핸들러의 다른 필드(name)와의 패턴 정합성 회복.
