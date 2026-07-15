## Purpose

유저가 본인을 식별할 수 있도록 인증을 수행하고, 인증 상태를 토큰으로 유지하며, 인증이 필요한 요청을 보호한다.
## Requirements
### Requirement: 소셜 로그인으로 인증한다
시스템은 Google 및 Discord OAuth를 통한 소셜 로그인을 제공해야 한다(SHALL).

#### Scenario: 첫 로그인 시 계정 자동 생성
- **WHEN** 처음 로그인하는 유저가 OAuth 인증을 완료하면
- **THEN** 유저 계정이 자동 생성되고, 닉네임은 OAuth 프로필에서 가져온다

#### Scenario: 이메일 기반 계정 병합
- **WHEN** 새 OAuth 로그인의 이메일이 기존 계정의 이메일과 일치하면
- **THEN** 기존 계정에 새 OAuth 연결을 추가하여 병합한다

#### Scenario: 이메일 없는 provider
- **WHEN** OAuth provider가 이메일을 제공하지 않으면
- **THEN** 이메일 기반 병합은 수행하지 않고 별도 계정으로 생성한다

#### Scenario: 동일 provider 중복 방지
- **WHEN** 이미 연결된 OAuth provider로 다시 로그인하면
- **THEN** 새 계정을 만들지 않고 기존 계정으로 로그인한다

---

### Requirement: 토큰 기반 인증 상태를 유지한다
시스템은 JWT 기반으로 인증 상태를 유지해야 한다(SHALL).

#### Scenario: 로그인 성공 시 토큰 발급
- **WHEN** OAuth 인증이 완료되면
- **THEN** access token과 refresh token이 발급된다

#### Scenario: 토큰 만료 시 갱신
- **WHEN** access token이 만료되었을 때 refresh 요청을 보내면
- **THEN** 새 access token이 발급된다

#### Scenario: 로그아웃
- **WHEN** 유저가 로그아웃하면
- **THEN** 토큰이 무효화된다

---

### Requirement: 인증이 필요한 요청을 보호한다
시스템은 인증이 필요한 API에 대해 유효한 토큰 없이 접근하면 거부해야 한다(SHALL).

#### Scenario: 토큰 없는 접근
- **WHEN** 인증 필요 API에 토큰 없이 요청하면
- **THEN** 401 응답이 반환된다

#### Scenario: 만료된 토큰
- **WHEN** 만료된 access token으로 요청하면
- **THEN** 401 응답과 함께 토큰 만료 여부가 전달된다

### Requirement: 토큰이 존재할 때 선택적으로 인증 컨텍스트를 노출한다

시스템은 인증·비인증을 모두 허용하는 엔드포인트가 사용할 수 있는 선택적 인증 미들웨어 surface를 제공해야 한다(SHALL). 이 surface는 요청에 인증 토큰이 존재하고 그 토큰이 유효하면 핸들러에 인증된 유저 식별자를 노출하고, 토큰이 부재하거나 유효하지 않으면 핸들러를 그대로 호출해야 한다(SHALL). 본 surface는 토큰 부재·만료·서명 불일치·식별자 파싱 실패 어느 경우에도 401을 반환해서는 안 된다(SHALL NOT).

본 Requirement는 기존 Requirement `인증이 필요한 요청을 보호한다`의 행위를 변경하지 않는다. 그 Requirement는 토큰 부재 시 401을 반환하는 강제 인증 surface를 계속 규정한다.

#### Scenario: 토큰 없는 요청

- **WHEN** 요청에 인증 토큰이 존재하지 않으면
- **THEN** 핸들러가 호출되며, 핸들러는 호출자가 인증되지 않았다고 관찰한다

#### Scenario: 유효한 토큰을 가진 요청

- **WHEN** 요청에 유효한 인증 토큰이 존재하면
- **THEN** 핸들러가 호출되며, 핸들러는 호출자의 유저 식별자를 관찰할 수 있다

#### Scenario: 유효하지 않은 토큰을 가진 요청

- **WHEN** 요청에 만료·서명 불일치·식별자 파싱 실패 중 어느 사유로든 유효하지 않은 토큰이 존재하면
- **THEN** 핸들러가 호출되며, 핸들러는 호출자가 인증되지 않았다고 관찰한다. 401은 반환되지 않는다

---

### Requirement: OAuth 프로필 필드 길이를 creators 컬럼 cap에 맞춰 절단한다

시스템은 OAuth provider가 반환한 프로필 필드(nickname, avatar_url)의 길이가 `creators` 테이블의 해당 컬럼 cap을 초과하면, INSERT 전에 rune-count 기준으로 cap에 맞춰 절단해야 한다(SHALL). 절단된 필드로 계정 INSERT는 정상 수행되어야 하며, 계정 자동 생성 자체는 실패하지 않아야 한다(SHALL NOT fail).

본 Requirement는 기존 Requirement "소셜 로그인으로 인증한다"의 Scenario "첫 로그인 시 계정 자동 생성"이 OAuth payload의 임의 길이에 대해 enforce되도록 보장하는 wiring 계약이다. 기존 Requirement의 의미는 변경하지 않는다.

#### Scenario: avatar_url이 컬럼 cap을 초과하는 OAuth payload

- **WHEN** OAuth provider가 500 rune을 초과하는 `avatar_url`을 반환하면
- **THEN** 시스템은 `avatar_url`을 정확히 500 rune으로 절단한 후 `creators` 테이블에 INSERT한다. INSERT는 성공하며 사용자는 정상적으로 로그인된다

#### Scenario: nickname이 컬럼 cap을 초과하는 OAuth payload

- **WHEN** OAuth provider가 50 rune을 초과하는 `nickname`을 반환하면
- **THEN** 시스템은 `nickname`을 정확히 50 rune으로 절단한 후 `creators` 테이블에 INSERT한다. INSERT는 성공하며 사용자는 정상적으로 로그인된다

#### Scenario: 정상 길이의 OAuth payload는 무손실 보존된다

- **WHEN** OAuth provider가 cap 이하 길이의 프로필 필드를 반환하면
- **THEN** 시스템은 절단 분기를 발동하지 않고 원본 값을 그대로 `creators` 테이블에 INSERT한다

#### Scenario: 멀티바이트 문자 프로필 필드는 rune count 기준으로 절단된다

- **WHEN** OAuth provider가 한국어·이모지 등 멀티바이트 문자로 구성된 cap 초과 프로필 필드를 반환하면
- **THEN** 시스템은 byte count가 아니라 rune count 기준으로 cap에 맞춰 절단한다. PostgreSQL `VARCHAR(N)`은 rune-count cap이므로 byte 길이가 N을 초과해도 정상 저장된다

### Requirement: 인증된 유저의 프로필을 노출한다

시스템은 인증된 유저가 본인 프로필(`GET /api/auth/me`)을 조회할 때, 미설정(NULL) `avatar_url`·`email` 필드를 JSON `null`로 직렬화해야 한다(SHALL). 빈 문자열(`""`)로 직렬화해서는 안 된다(SHALL NOT).

본 Requirement는 동일 인증 Creator를 반환하는 `GET /api/creators/me`(profile capability) 및 코드베이스 전반의 nullable 컬럼 직렬화 관용(미설정 nullable 값 → JSON `null`)에 정렬하는 응답 페이로드 계약이다. 인증·인가 동작과 그 외 응답 키(`id`·`nickname`)의 의미는 변경하지 않는다.

#### Scenario: avatar_url·email이 미설정인 유저의 프로필 조회

- **WHEN** `avatar_url`과 `email`이 NULL인 인증된 유저가 `GET /api/auth/me`를 호출하면
- **THEN** 응답의 `avatar_url`과 `email`은 JSON `null`로 직렬화된다. 두 필드는 응답에서 누락되지 않으며 빈 문자열로 직렬화되지 않는다

#### Scenario: avatar_url·email이 설정된 유저의 프로필 조회

- **WHEN** `avatar_url`과 `email`에 값이 있는 인증된 유저가 `GET /api/auth/me`를 호출하면
- **THEN** 응답의 `avatar_url`과 `email`은 저장된 문자열 값을 그대로 노출한다

#### Scenario: 동일 유저의 두 프로필 endpoint 응답이 동일한 null 표현을 사용한다

- **WHEN** `avatar_url`·`email`이 NULL인 동일 유저가 `GET /api/auth/me`와 `GET /api/creators/me`를 각각 호출하면
- **THEN** 두 응답 모두 `avatar_url`·`email`을 JSON `null`로 직렬화한다. 두 endpoint는 같은 필드의 미설정 상태를 서로 다르게 표현하지 않는다

### Requirement: 로그인 페이지 에러 메시지 표시

OAuth 콜백 과정에서 오류가 발생하여 로그인 페이지로 돌아온 경우, 시스템은 오류 원인에 대응하는 한국어 안내 문구를 로그인 페이지에 표시해야 한다(SHALL). 재시도를 권고하는 안내 문구의 보조용언 표기는 코드베이스에서 확립된 붙여쓰기 표기("다시 시도해주세요")를 따라야 한다(SHALL).

#### Scenario: 세션 만료 오류 안내

- **WHEN** 세션 만료(state 불일치) 오류로 로그인 페이지에 도착하면
- **THEN** "세션이 만료되었습니다. 다시 시도해주세요" 문구가 표시된다

#### Scenario: 인증 교환 실패 안내

- **WHEN** 인증 코드 교환 실패 오류로 로그인 페이지에 도착하면
- **THEN** "인증에 실패했습니다. 다시 시도해주세요" 문구가 표시된다

#### Scenario: 미등록 오류 코드 기본 안내

- **WHEN** 매핑되지 않은 오류 코드로 로그인 페이지에 도착하면
- **THEN** "로그인에 실패했습니다. 다시 시도해주세요" 문구가 표시된다

#### Scenario: 재시도 권고가 없는 오류 안내 유지

- **WHEN** 재시도 권고를 포함하지 않는 오류(알 수 없는 로그인 방법, 프로필 조회 실패, 계정 처리 오류, 토큰 처리 오류)로 로그인 페이지에 도착하면
- **THEN** 기존 안내 문구가 변경 없이 그대로 표시된다

