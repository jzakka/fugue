## ADDED Requirements

### Requirement: 보드 description 입력은 `boards.description` 컬럼 cap에 맞춰 사전 길이 검증된다

시스템은 보드 생성/수정 요청의 `description` 필드를 DB INSERT/UPDATE 호출 전에 rune 단위로 길이를 검증하여, `boards.description VARCHAR(500)` 컬럼 cap을 초과하는 입력을 HTTP 400 응답으로 거부해야 한다(SHALL).

#### Scenario: 생성 요청 description 길이 초과 거부

- **WHEN** 인증된 유저가 POST `/api/boards`로 `description`이 500 rune을 초과하는 본문을 전송하면
- **THEN** 서버는 DB 호출 없이 400 응답 `{"error": "보드 설명은 500자 이내여야 합니다"}`를 반환한다

#### Scenario: 수정 요청 description 길이 초과 거부

- **WHEN** 보드 소유자가 PUT `/api/boards/{id}`로 `description`이 500 rune을 초과하는 본문을 전송하면
- **THEN** 서버는 UpdateBoard DB 호출 없이 400 응답 `{"error": "보드 설명은 500자 이내여야 합니다"}`를 반환한다

#### Scenario: 멀티바이트 입력은 rune 단위로 측정한다

- **WHEN** 사용자가 한국어/이모지 등 멀티바이트 문자로 구성된 description을 전송하면
- **THEN** 서버는 byte 길이가 아닌 rune(`utf8.RuneCountInString`) 단위로 cap(500)을 비교하여 PostgreSQL의 `character varying(500)` 규칙과 일치하는 결정을 내린다

#### Scenario: cap 이내 또는 생략된 description은 검증을 통과한다

- **WHEN** 사용자가 description을 500 rune 이내로 전송하거나 본문에서 description을 생략하면
- **THEN** 서버는 cap 검증을 통과시키고 기존 핸들러 로직(생성/수정/기존 description 보존)을 정상 수행한다
