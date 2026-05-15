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

