## ADDED Requirements

### Requirement: Discord OAuth provider 활성화
시스템은 `DISCORD_CLIENT_ID`와 `DISCORD_CLIENT_SECRET` 환경변수가 설정된 경우 Discord OAuth provider를 활성화해야 한다(SHALL).

#### Scenario: 환경변수 설정 시 provider 등록
- **WHEN** `DISCORD_CLIENT_ID`와 `DISCORD_CLIENT_SECRET`이 모두 비어있지 않은 값으로 설정됨
- **THEN** `/api/auth/providers` 응답에 `"discord"`가 포함됨

#### Scenario: 환경변수 미설정 시 provider 비활성
- **WHEN** `DISCORD_CLIENT_ID` 또는 `DISCORD_CLIENT_SECRET`이 비어있음
- **THEN** `/api/auth/providers` 응답에 `"discord"`가 포함되지 않음

### Requirement: Discord 로그인 플로우
사용자는 Discord 계정으로 Fugue에 로그인할 수 있어야 한다(SHALL). OAuth Authorization Code Flow를 사용한다.

#### Scenario: 정상 로그인
- **WHEN** 사용자가 Discord 로그인 버튼을 클릭하고 Discord에서 권한을 허용함
- **THEN** 시스템은 JWT 토큰 쌍(access + refresh)을 발급하고, httpOnly 쿠키로 설정한 뒤, 프론트엔드로 리다이렉트함

#### Scenario: 사용자가 Discord 권한 거부
- **WHEN** 사용자가 Discord 로그인 페이지에서 권한을 거부함
- **THEN** 시스템은 `/login?error=access_denied`로 리다이렉트함

### Requirement: Discord 프로필 정보 수집
시스템은 Discord API에서 사용자 프로필을 가져와 creator 레코드에 저장해야 한다(SHALL).

#### Scenario: Discord 프로필로 creator 생성
- **WHEN** 처음 Discord로 로그인하는 사용자가 인증을 완료함
- **THEN** Discord username이 nickname으로, avatar URL이 avatar_url로 저장된 새 creator가 생성됨

#### Scenario: 이메일 기반 계정 병합
- **WHEN** Discord 계정의 이메일이 기존 Google 로그인 creator의 이메일과 동일함
- **THEN** 새 creator를 생성하지 않고, 기존 creator에 Discord auth_account를 추가함

### Requirement: 프론트엔드 Discord 로그인 버튼
로그인 페이지는 Discord provider가 활성화된 경우 Discord 로그인 버튼을 표시해야 한다(SHALL).

#### Scenario: Discord 버튼 표시
- **WHEN** `/api/auth/providers` 응답에 `"discord"`가 포함됨
- **THEN** 로그인 페이지에 Discord 로그인 버튼이 렌더링됨

#### Scenario: Discord 버튼 미표시
- **WHEN** `/api/auth/providers` 응답에 `"discord"`가 포함되지 않음
- **THEN** 로그인 페이지에 Discord 로그인 버튼이 렌더링되지 않음
