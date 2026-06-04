## ADDED Requirements

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
