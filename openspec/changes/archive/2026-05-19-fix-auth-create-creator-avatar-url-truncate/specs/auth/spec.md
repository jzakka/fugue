## ADDED Requirements

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
