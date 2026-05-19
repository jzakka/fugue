# Proposal: POST /api/pins 입력 길이 검증 추가

## Why

`apps/api/internal/pin/handler.go`의 `Create` 핸들러(L70-342)는 multipart form의 `title`, `description`, `url`, `og_image` 4개 텍스트 필드를 trim 후 그대로 `db.CreatePinParams`에 담아 `CreatePin` INSERT로 흘려보낸다(L312-325). 그러나 `pins` 컬럼은 `apps/api/db/migrations/000003_create_works.up.sql:4-9`에서 다음과 같이 정의되어 있다:

```sql
url         VARCHAR(1000) NOT NULL,
title       VARCHAR(200) NOT NULL,
description VARCHAR(500),
og_image    VARCHAR(1000),
```

이후 ALTER COLUMN으로 cap을 늘린 마이그레이션이 없다(`grep -irE "(title|description|url|og_image).*varchar" apps/api/db/migrations/` 1회 검증). 즉 production 스키마와 일치하는 cap이다.

결과: 클라이언트가 cap을 초과하는 텍스트 필드를 제출하면 PostgreSQL이 `value too long for type character varying(N)`로 INSERT를 거부 → L321의 `if err != nil` 분기가 모든 에러를 흡수해 `500 "핀 등록에 실패했습니다"`를 반환한다. 사용자는 (a) 자신이 어느 필드를 어느 길이로 줄여야 하는지 알 수 없고, (b) 서버 오류로 보이는 응답을 받아 자신의 입력 문제임을 인지하지 못하며, (c) 멀티미디어 파일을 이미 S3에 업로드한 뒤 INSERT 단계에서 실패하므로 orphan 미디어가 생성된다(rollback 없음).

이는 같은 패키지 내 기존 검증 패턴과의 비대칭 결함이기도 하다:
- `apps/api/internal/creator/handler.go:178-185`: `nickname` `utf8.RuneCountInString > 50 → 400 "닉네임은 50자를 초과할 수 없습니다"`
- `apps/api/internal/boards/handler.go:110`: `name` `utf8.RuneCountInString > 100 → 400 "보드 이름은 100자 이내여야 합니다"`

같은 codebase에서 다른 핸들러가 이미 enforce하는 패턴이 pin 핸들러에만 누락되어 있다.

spec 측면에서도 `openspec/specs/pin/spec.md:289-318` "외부 URL을 핀으로 저장한다" Requirement는 입력 길이 cap에 대한 Scenario를 갖고 있지 않은 갭이 있다. 본 change는 이 갭을 채워 spec과 코드를 동시 정렬한다.

## What Changes

1. **pin/handler.go의 Create 핸들러에 4개 텍스트 필드 길이 검증 추가** — `apps/api/internal/pin/handler.go`에서:
   - L96 직후(title 빈값 거부 다음): `if utf8.RuneCountInString(title) > 200 { 400 "제목은 200자 이내여야 합니다" }`
   - L288 내부(description trim 직후): `if utf8.RuneCountInString(d) > 500 { 400 "설명은 500자 이내여야 합니다" }`
   - L293 내부(url trim 직후): `if utf8.RuneCountInString(u) > 1000 { 400 "URL은 1000자 이내여야 합니다" }`
   - L298 내부(og_image trim 직후): `if utf8.RuneCountInString(o) > 1000 { 400 "og_image URL은 1000자 이내여야 합니다" }`

2. **`unicode/utf8` import 추가** — 동일 파일 import 블록.

3. **회귀 방지 단위 테스트 8건** — `apps/api/internal/pin/handler_test.go`에 신규 subtest:
   - title: ASCII 201자 → 400 / 한국어 201 rune → 400
   - description: ASCII 501자 → 400 / 한국어 501 rune → 400
   - url: ASCII 1001자 → 400 / 한국어 1001 rune → 400
   - og_image: ASCII 1001자 → 400 / 한국어 1001 rune → 400
   - 각 필드 cap 정확값(200/500/1000/1000)에서 무손실 통과(별도 1건씩 4개)

4. **pin spec에 ADDED Requirement 추가** — `openspec/specs/pin/spec.md`의 "외부 URL을 핀으로 저장한다" Requirement 부근에 "핀 생성 요청의 텍스트 필드는 pins 컬럼 cap에 맞춰 사전 길이 검증된다" Requirement를 ADDED.

## Why Now / Why Self-Contained

- **Why Now**: Discovery 모드에서 발견된 정합성 결함. 같은 패턴을 이미 enforce하는 핸들러가 codebase에 2개 있어(creator/nickname, boards/name) 확신도 5. fix가 한 파일 4 블록 추가로 self-contained.
- **Why Self-Contained**: 변경 범위가 (a) pin/handler.go 4 블록 + 1 import, (b) pin/handler_test.go 신규 subtest 12개, (c) spec ADDED Requirement 1개로 전부 한 changeset 안에 닫힌다. DB 마이그레이션, 다른 패키지 변경, infra 영향 없음.

## Scope

- 변경 파일: `apps/api/internal/pin/handler.go`(4 블록 + 1 import), `apps/api/internal/pin/handler_test.go`(신규 subtest), `openspec/specs/pin/spec.md`(1 ADDED Requirement).
- 변경 외 파일: bot/harvester 경로(이미 별도 truncate 정책 enforce), boards/handler, creator/handler(이미 동일 패턴 enforce 중).

## Out of Scope

- pins 컬럼 cap 확장(예: VARCHAR(200) → VARCHAR(500)) 마이그레이션 — 별개 change. 현 ERD와 spec이 합의된 cap이며, 본 change는 cap을 그대로 두고 enforce만 추가한다.
- boards/creator 핸들러의 description/avatar_url 길이 검증 누락 — `backlog-system.yaml`의 별개 item으로 분리되어 있다(`system-20260519-boards-handler-description-no-length-validation`, `system-20260519-creator-update-avatar-url-no-length-validation`). 본 change는 pin 핸들러에만 집중.
- 미디어 업로드 성공 후 INSERT 실패로 인한 orphan S3 객체 정리 정책 — 별개 이슈. 본 change로 인해 (cap 초과로 인한) orphan 발생 확률이 사라지므로 부분적 해소는 되지만, 다른 원인(예: 태그 연결 실패)에 의한 orphan은 여전히 별도 처리 필요.

## Rollback

`pin/handler.go`의 4 검증 블록과 unicode/utf8 import를 revert. 테스트 subtest 삭제. spec Requirement revert. DB나 데이터 변환이 없으므로 즉시 가역. 기존에 정상 입력(cap 이하)으로 동작하던 모든 호출 경로는 변경 없음.

## QA Plan (실 환경)

1. `docker-compose up -d`로 api+postgres+redis 기동.
2. `cd apps/api && go run cmd/server/main.go`.
3. 인증 세션 획득(`/api/auth/dev-login` 또는 동등 dev-only 경로).
4. 각 필드별로 cap+1 길이 입력 POST `/api/pins` curl 호출:
   - title: `A` × 201 → 400 + body `"제목은 200자 이내여야 합니다"`
   - description: `B` × 501 → 400 + body `"설명은 500자 이내여야 합니다"`
   - url: `https://example.com/` + `c` × 980 (total 1001) → 400 + body `"URL은 1000자 이내여야 합니다"`
   - og_image: 동일 패턴 1001자 → 400 + body `"og_image URL은 1000자 이내여야 합니다"`
5. 멀티바이트 검증: title=`가` × 201 → 400 (utf8.RuneCountInString이 rune 단위로 세는지 확인).
6. 경계값 검증: title=`A` × 200 → 201 (정상 생성). description=500/url=1000/og_image=1000 동일.
7. 회귀: 일반 입력(title=10자, description=20자, url=정상 URL, og_image 미포함, jpeg 미디어) → 201 정상.
8. 응답 확인: 모든 400 응답이 (a) 정확한 한국어 메시지, (b) `Content-Type: application/json`, (c) `{"error": "..."}` 스키마인지.
9. 회귀(인접 엔드포인트): `GET /api/pins/{id}`로 6에서 생성된 pin 조회 → 200 정상.
10. `psql`로 `SELECT length(title), length(description), length(url), length(og_image) FROM pins ORDER BY created_at DESC LIMIT 5;` → 모든 row가 cap 이하 확인.

## Threat Model / Failure Mode

- **이전 (fix 없음)**: 클라이언트 검증 우회 시 cap 초과 입력 → S3 미디어 업로드 성공 → DB INSERT 실패 → 500 "핀 등록에 실패했습니다" (사용자 친화 불가능 + orphan S3 객체 발생). 한 사이클당 미디어 1개 누수.
- **이후 (fix 적용)**: cap 초과 입력 → multipart 파싱 직후 또는 S3 업로드 직후 4xx 거부 → 사용자가 어느 필드를 줄여야 하는지 명확히 인지. title은 S3 업로드 전에 검증되어 orphan도 발생 안 함(description/url/og_image는 S3 업로드 직후 검증이지만 이는 멀티파트 파일 부분이 이미 disk spool된 이후이므로 회피 불가). spec과 코드, 같은 패키지 내 다른 핸들러와의 패턴 정합성 회복.
