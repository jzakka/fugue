## ADDED Requirements

### Requirement: `avatar_url` 입력은 `creators.avatar_url` 컬럼 cap에 맞춰 사전 길이 검증된다

시스템은 본인 프로필 수정 요청의 `avatar_url` 필드를 DB UPDATE 호출 전에 rune 단위로 길이를 검증하여, `creators.avatar_url VARCHAR(500)` 컬럼 cap을 초과하는 입력을 HTTP 400 응답으로 거부해야 한다(SHALL). 빈 문자열(`""`)은 cap 검증 대상이 아니며 기존 아바타 제거(clear) 동작을 유지한다.

#### Scenario: avatar_url 길이 초과 거부

- **WHEN** 인증된 유저가 PUT `/api/creators/me`로 `avatar_url`이 500 rune을 초과하는 본문을 전송하면
- **THEN** 서버는 UpdateCreator DB 호출 없이 400 응답 `{"error": "아바타 URL은 500자 이내여야 합니다"}`를 반환한다

#### Scenario: 멀티바이트 입력은 rune 단위로 측정한다

- **WHEN** 사용자가 한국어/이모지 등 멀티바이트 문자로 구성된 avatar_url을 전송하면
- **THEN** 서버는 byte 길이가 아닌 rune(`utf8.RuneCountInString`) 단위로 cap(500)을 비교하여 PostgreSQL의 `character varying(500)` 규칙과 일치하는 결정을 내린다

#### Scenario: 빈 문자열은 아바타 제거(clear)로 해석된다

- **WHEN** 사용자가 `{"avatar_url": ""}`로 PUT을 전송하면
- **THEN** 서버는 cap 검증을 건너뛰고 avatar_url을 NULL로 갱신한다(기존 clear semantics 보존)

#### Scenario: avatar_url이 누락된 부분 업데이트는 기존 값을 보존한다

- **WHEN** 사용자가 본문에서 `avatar_url`을 생략하고 다른 필드(예: nickname)만 갱신하면
- **THEN** 서버는 cap 검증 분기를 건너뛰고 기존 avatar_url 값을 그대로 유지한다
