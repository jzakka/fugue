## ADDED Requirements

### Requirement: 핀 생성 요청의 텍스트 필드는 pins 컬럼 cap에 맞춰 사전 길이 검증된다

핀 생성 요청(POST /api/pins)을 처리할 때 시스템은 `title`, `description`, `url`, `og_image` 4개 텍스트 필드의 길이를 각각의 DB 컬럼 cap에 맞춰 사전 검증해야 한다(SHALL). 검증은 UTF-8 rune 단위로 수행되며(SHALL), 바이트 길이가 아닌 character 길이로 비교된다(이는 PostgreSQL `VARCHAR(N)` 의 N과 같은 단위다).

각 필드의 cap:
- `title`: 200 rune (`pins.title VARCHAR(200) NOT NULL`)
- `description`: 500 rune (`pins.description VARCHAR(500)`)
- `url`: 1000 rune (`pins.url VARCHAR(1000)`)
- `og_image`: 1000 rune (`pins.og_image VARCHAR(1000)`)

cap을 초과하는 입력에 대해 시스템은 `400 Bad Request`와 어느 필드가 어느 cap을 초과했는지 식별할 수 있는 한국어 에러 메시지를 반환해야 한다(SHALL). 이는 PostgreSQL이 INSERT 시점에 발생시키는 `value too long for type character varying(N)` 오류가 핸들러의 일반 INSERT 에러 분기에 흡수되어 사용자에게 `500 핀 등록에 실패했습니다`로 노출되는 것을 방지한다.

이는 같은 codebase의 `creator.UpdateMe`(nickname 50 rune cap), `boards.Create/Update`(name 100 rune cap)에서 이미 enforce하는 패턴과 동일한 정책이다.

#### Scenario: title이 200 rune을 초과하면 400으로 거부된다
- **WHEN** 인증된 유저가 201 rune 이상의 title로 핀 생성을 요청하면
- **THEN** 시스템은 400 응답과 "제목은 200자 이내여야 합니다" 메시지를 반환하며, S3 미디어 업로드를 시도하지 않는다

#### Scenario: 멀티바이트 문자열은 rune 단위로 비교된다
- **WHEN** title/description/url/og_image가 한국어/일본어/이모지 등 멀티바이트 문자열이고 rune 수가 cap을 1개 초과할 때
- **THEN** 시스템은 byte 길이가 아닌 rune 길이로 판정해 400 응답을 반환한다 (한국어 200 rune은 600 byte이지만 rune 기준으로는 cap 이내이므로 통과해야 한다)

#### Scenario: cap 정확값 입력은 무손실 통과된다
- **WHEN** title=200 rune / description=500 rune / url=1000 rune / og_image=1000 rune 입력일 때
- **THEN** 시스템은 입력을 그대로 받아들여 핀을 생성하며 길이/내용을 변경하지 않는다 (`>` 비교, `>=` 아님)

#### Scenario: description/url/og_image는 빈 값일 때 길이 검증을 건너뛴다
- **WHEN** description/url/og_image 필드가 trim 후 빈 문자열일 때
- **THEN** 시스템은 길이 검증을 수행하지 않고 해당 필드를 NULL로 저장한다 (사용자가 선택 필드를 비워둔 경우의 정상 흐름)
